package backup_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"github.com/virtualmin/virtualmin-backup-browser/internal/archive"
	"github.com/virtualmin/virtualmin-backup-browser/internal/backup"
)

// writeTar builds an uncompressed tar from name->content pairs, using the
// "./name" convention GNU tar produces for `tar cf - .`.
func writeTar(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for name, content := range files {
		hdr := &tar.Header{Name: "./" + name, Mode: 0644, Size: int64(len(content))}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// makeBackup writes a gzipped single-file backup with a nested home tar,
// mirroring the real Virtualmin layout, and returns its path.
func makeBackup(t *testing.T) string {
	t.Helper()
	innerTar := writeTar(t, map[string]string{
		"public_html/index.html": "<h1>hello</h1>",
		".virtualmin-src":        "id=123",
	})
	outer := writeTar(t, map[string]string{
		"example.com_dir.tar":      string(innerTar),
		"example.com_mysql":        "hosts=localhost",
		"example.com_mysql_maindb": "-- SQL dump\n",
		"sub.example.com_dir.tar":  string(writeTar(t, map[string]string{"index.html": "sub"})),
		"virtualmin_config":        "key=value",
	})

	path := filepath.Join(t.TempDir(), "backup.tar.gz")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	if _, err := gz.Write(outer); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestOpenAndDomains(t *testing.T) {
	b, err := backup.Open(makeBackup(t))
	if err != nil {
		t.Fatal(err)
	}
	if b.Format != archive.FormatGzip {
		t.Errorf("format = %s, want tar.gz", b.Format)
	}
	domains := b.Domains()
	names := map[string]*backup.Domain{}
	for _, d := range domains {
		names[d.Name] = d
	}
	if _, ok := names["example.com"]; !ok {
		t.Fatalf("example.com not found; got %v", names)
	}
	if _, ok := names["sub.example.com"]; !ok {
		t.Error("sub.example.com not found")
	}
	if _, ok := names["virtualmin"]; !ok {
		t.Error("global virtualmin config not found")
	}
	// example.com should expose dir and mysql features.
	feats := names["example.com"].Features
	if len(feats) != 2 || feats[0] != "dir" || feats[1] != "mysql" {
		t.Errorf("example.com features = %v, want [dir mysql]", feats)
	}
}

func TestOpenDirectoryFormat(t *testing.T) {
	b, err := backup.Open("testdata/dirbackup")
	if err != nil {
		t.Fatal(err)
	}
	if !b.IsDir {
		t.Error("IsDir = false, want true for directory-format backup")
	}
	domains := b.Domains()
	if len(domains) != 1 || domains[0].Name != "example.com" {
		t.Fatalf("domains = %v, want [example.com]", domains)
	}
	wantFeat := []string{"mail", "mysql", "virtualmin"}
	if got := domains[0].Features; !equalStrings(got, wantFeat) {
		t.Errorf("features = %v, want %v", got, wantFeat)
	}
}

func TestDomainMetaPrefersSidecar(t *testing.T) {
	b, err := backup.Open("testdata/dirbackup")
	if err != nil {
		t.Fatal(err)
	}
	conf, err := b.DomainMeta("example.com")
	if err != nil {
		t.Fatal(err)
	}
	// The .dom sidecar carries /home/exampleco; the in-archive _virtualmin
	// member carries /home/FROM_MEMBER. The sidecar must win.
	if conf["home"] != "/home/exampleco" {
		t.Errorf("home = %q, want /home/exampleco (sidecar should win)", conf["home"])
	}
	if conf["uid"] != "5001" {
		t.Errorf("uid = %q, want 5001 (sidecar-only field)", conf["uid"])
	}
}

func TestReadMemberRoutesToSource(t *testing.T) {
	b, err := backup.Open("testdata/dirbackup")
	if err != nil {
		t.Fatal(err)
	}
	data, err := b.ReadMember("example.com_mysql")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte("hosts=127.0.0.1")) {
		t.Errorf("example.com_mysql = %q, missing expected content", data)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestNestedWalk(t *testing.T) {
	path := makeBackup(t)
	var inner []string
	err := archive.WalkFile(path, func(e archive.Entry) error {
		if e.Name != "example.com_dir.tar" {
			return nil
		}
		return archive.WalkNested(e.Reader, func(ne archive.Entry) error {
			inner = append(inner, ne.Name)
			return nil
		})
	})
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, n := range inner {
		if n == "public_html/index.html" {
			found = true
		}
	}
	if !found {
		t.Errorf("nested index.html not found; got %v", inner)
	}
}

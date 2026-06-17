package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"github.com/virtualmin/virtualmin-backup-browser/internal/backup"
)

func tarBytes(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for name, content := range files {
		if err := tw.WriteHeader(&tar.Header{Name: "./" + name, Mode: 0644, Size: int64(len(content))}); err != nil {
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

func makeGzBackup(t *testing.T) string {
	t.Helper()
	inner := tarBytes(t, map[string]string{"public_html/index.html": "<h1>hi</h1>"})
	outer := tarBytes(t, map[string]string{
		"example.com_dir.tar": string(inner),
		"example.com_mysql":   "hosts=localhost",
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

func TestExtractAllExpandsNested(t *testing.T) {
	b, err := backup.Open(makeGzBackup(t))
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	if err := extractAll(b, out, false); err != nil {
		t.Fatal(err)
	}
	// Nested archive expanded into a directory named after the member.
	if got, err := os.ReadFile(filepath.Join(out, "example.com_dir", "public_html", "index.html")); err != nil {
		t.Errorf("expanded nested file missing: %v", err)
	} else if string(got) != "<h1>hi</h1>" {
		t.Errorf("nested content = %q", got)
	}
	// Plain member written as a file.
	if _, err := os.Stat(filepath.Join(out, "example.com_mysql")); err != nil {
		t.Errorf("plain member missing: %v", err)
	}
}

func TestExtractAllRawKeepsArchives(t *testing.T) {
	b, err := backup.Open(makeGzBackup(t))
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	if err := extractAll(b, out, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(out, "example.com_dir.tar")); err != nil {
		t.Errorf("--raw should keep the nested .tar: %v", err)
	}
	if _, err := os.Stat(filepath.Join(out, "example.com_dir")); !os.IsNotExist(err) {
		t.Errorf("--raw should not expand nested archive")
	}
}

func TestSafeJoinRejectsTraversal(t *testing.T) {
	base := t.TempDir()
	for _, bad := range []string{"../escape", "a/../../escape", "/etc/passwd"} {
		dest, err := safeJoin(base, bad)
		if err == nil && !filepath.HasPrefix(dest, base) {
			t.Errorf("safeJoin(%q) = %q, escaped base", bad, dest)
		}
	}
	good, err := safeJoin(base, "sub/dir/file")
	if err != nil || !filepath.HasPrefix(good, base) {
		t.Errorf("safeJoin rejected a safe path: %q, %v", good, err)
	}
}

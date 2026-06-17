package backup

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/virtualmin/virtualmin-backup-browser/internal/archive"
)

// Backup is the in-memory view of a backup. A backup is either a single archive
// file (single-file or home format) or a directory of per-domain archives plus
// .info/.dom sidecars (directory format). Both present the same flat list of
// members; each member records the source archive it came from.
type Backup struct {
	Path    string
	IsDir   bool
	Format  archive.Format // single-file: the file's format; directory: first source's
	Members []Member

	// domSidecars maps a domain name to the raw bytes of its .dom sidecar
	// (Webmin serialise_variable), present only in directory-format backups.
	domSidecars map[string][]byte
}

// Domain groups the members belonging to one domain.
type Domain struct {
	Name     string
	Members  []Member
	Features []string // sorted, unique feature ids present
}

// archiveSuffixes are the per-domain archive extensions a directory-format
// backup uses, longest first so ".tar.gz" wins over ".tar".
var archiveSuffixes = []string{".tar.gz", ".tar.bz2", ".tar.zst", ".tar", ".zip"}

// Open scans the backup at path and parses every member. It reads each
// archive's table of contents only; member contents are not buffered. If path
// is a directory it is treated as a directory-format backup.
func Open(path string) (*Backup, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if fi.IsDir() {
		return openDir(path)
	}
	return openFile(path)
}

func openFile(path string) (*Backup, error) {
	format, err := archive.DetectFile(path)
	if err != nil {
		return nil, err
	}
	b := &Backup{Path: path, Format: format}
	err = archive.WalkFile(path, func(e archive.Entry) error {
		m := ParseMember(e.Name, e.Size, e.IsDir)
		m.SourcePath = path
		b.Members = append(b.Members, m)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return b, nil
}

// openDir treats path as a directory-format backup: one archive per domain
// (<domain>.<suffix>) accompanied by optional <domain>.<suffix>.info and .dom
// serialise_variable sidecars.
func openDir(path string) (*Backup, error) {
	ents, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	b := &Backup{Path: path, IsDir: true, domSidecars: map[string][]byte{}}

	for _, ent := range ents {
		if ent.IsDir() {
			continue
		}
		name := ent.Name()
		full := filepath.Join(path, name)

		// Sidecars are read into memory and indexed by domain.
		if dom, ok := strings.CutSuffix(name, ".dom"); ok {
			if data, derr := os.ReadFile(full); derr == nil {
				b.domSidecars[domainFromArchive(dom)] = data
			}
			continue
		}
		if strings.HasSuffix(name, ".info") {
			continue // .info duplicates feature data we already derive from members
		}

		// Anything else should be a per-domain archive. Skip files we can't
		// recognise as a container rather than failing the whole open.
		format, derr := archive.DetectFile(full)
		if derr != nil {
			continue
		}
		if len(b.Members) == 0 {
			b.Format = format
		}
		werr := archive.WalkFile(full, func(e archive.Entry) error {
			m := ParseMember(e.Name, e.Size, e.IsDir)
			m.SourcePath = full
			b.Members = append(b.Members, m)
			return nil
		})
		if werr != nil {
			return nil, werr
		}
	}
	return b, nil
}

// domainFromArchive strips a per-domain archive suffix to recover the domain
// name, e.g. "example.com.tar.gz" -> "example.com".
func domainFromArchive(name string) string {
	for _, sfx := range archiveSuffixes {
		if s, ok := strings.CutSuffix(name, sfx); ok {
			return s
		}
	}
	return name
}

// Domains returns the per-domain grouping of feature files, sorted by name. The
// global "virtualmin" pseudo-domain, if present, sorts last.
func (b *Backup) Domains() []*Domain {
	byName := map[string]*Domain{}
	for _, m := range b.Members {
		if !m.IsFeatureFile() {
			continue
		}
		d := byName[m.Domain]
		if d == nil {
			d = &Domain{Name: m.Domain}
			byName[m.Domain] = d
		}
		d.Members = append(d.Members, m)
	}

	out := make([]*Domain, 0, len(byName))
	for _, d := range byName {
		seen := map[string]bool{}
		for _, m := range d.Members {
			if !seen[m.Feature] {
				seen[m.Feature] = true
				d.Features = append(d.Features, m.Feature)
			}
		}
		sort.Strings(d.Features)
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool {
		gi, gj := out[i].Name == "virtualmin", out[j].Name == "virtualmin"
		if gi != gj {
			return gj // non-global first
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// Strays returns members that did not parse as feature files (unexpected or
// non-Virtualmin entries), useful for diagnostics.
func (b *Backup) Strays() []Member {
	var out []Member
	for _, m := range b.Members {
		if !m.IsFeatureFile() && !m.IsDir {
			out = append(out, m)
		}
	}
	return out
}

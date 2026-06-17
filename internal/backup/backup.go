package backup

import (
	"sort"

	"github.com/virtualmin/virtualmin-backup-browser/internal/archive"
)

// Backup is the in-memory view of a single backup archive file.
type Backup struct {
	Path    string
	Format  archive.Format
	Members []Member
}

// Domain groups the members belonging to one domain.
type Domain struct {
	Name     string
	Members  []Member
	Features []string // sorted, unique feature ids present
}

// Open scans the backup file at path and parses every member. It reads the
// archive's table of contents only; member contents are not buffered.
func Open(path string) (*Backup, error) {
	format, err := archive.DetectFile(path)
	if err != nil {
		return nil, err
	}
	b := &Backup{Path: path, Format: format}
	err = archive.WalkFile(path, func(e archive.Entry) error {
		b.Members = append(b.Members, ParseMember(e.Name, e.Size, e.IsDir))
		return nil
	})
	if err != nil {
		return nil, err
	}
	return b, nil
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

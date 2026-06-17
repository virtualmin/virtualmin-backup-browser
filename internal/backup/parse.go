package backup

import "strings"

// nestedSuffixes are the extensions Virtualmin appends to a feature file that
// is itself an archive. The "dir" feature in a single-file or directory backup
// is written as "<domain>_dir.<suffix>" (compression_to_suffix_inner yields a
// plain "tar", or "zip" in zip mode); the whole thing is then placed inside the
// outer container, producing the "tar inside a tar" users find confusing.
var nestedSuffixes = []string{".tar.gz", ".tar.bz2", ".tar.zst", ".tar", ".zip"}

// Member is a parsed archive entry. A Virtualmin member name has the shape
//
//	[.backup/]<domain>_<feature>[_<sub>][<nested-archive-suffix>]
//
// where <domain> never contains an underscore (hostnames cannot), so the first
// underscore separates the domain from the feature. Global configuration is
// stored under the pseudo-domain "virtualmin".
type Member struct {
	Raw      string // cleaned entry name as it appears in the archive
	Domain   string // owning domain, or "" if unparseable
	Feature  string // feature id (token after the first underscore)
	Sub      string // remainder after the feature id, e.g. "users" or a db name
	Size     int64
	IsDir    bool
	IsNested bool // the member is itself an archive (e.g. <domain>_dir.tar)
	IsGlobal bool // a virtualmin_* global-config member

	// SourcePath is the filesystem path of the archive file this member was
	// read from. For a single-file backup every member shares the backup path;
	// for a directory-format backup it is the per-domain archive.
	SourcePath string
}

// ParseMember splits a cleaned archive entry name into its components.
func ParseMember(name string, size int64, isDir bool) Member {
	m := Member{Raw: name, Size: size, IsDir: isDir}

	// Home-format backups nest the feature files under .backup/.
	core := strings.TrimPrefix(name, ".backup/")

	// Strip a nested-archive suffix before splitting, so "example.com_dir.tar"
	// parses to feature "dir" rather than leaving ".tar" stuck to the feature.
	for _, sfx := range nestedSuffixes {
		if strings.HasSuffix(core, sfx) {
			m.IsNested = true
			core = strings.TrimSuffix(core, sfx)
			break
		}
	}

	us := strings.IndexByte(core, '_')
	if us < 0 {
		// No underscore: not a recognisable feature file (could be a stray
		// file such as .virtualmin-src inside a home dir).
		return m
	}
	m.Domain = core[:us]
	rest := core[us+1:]

	// The feature id is the leading run of [a-z0-9-]; anything after is the sub.
	fend := len(rest)
	for i := 0; i < len(rest); i++ {
		c := rest[i]
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
			fend = i
			break
		}
	}
	m.Feature = rest[:fend]
	if fend < len(rest) {
		m.Sub = strings.TrimPrefix(rest[fend:], "_")
	}

	if m.Domain == "virtualmin" {
		m.IsGlobal = true
	}
	return m
}

// IsFeatureFile reports whether the member is a parseable feature file (as
// opposed to a stray or unrecognised entry).
func (m Member) IsFeatureFile() bool {
	return m.Domain != "" && m.Feature != ""
}

package backup

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/virtualmin/virtualmin-backup-browser/internal/archive"
	"github.com/virtualmin/virtualmin-backup-browser/internal/serialise"
)

// ParseConfig decodes a Webmin write_file()-format file: one "key=value" pair
// per line. This is the encoding used by the per-domain <domain>_virtualmin
// metadata file and the per-feature info files (e.g. <domain>_mysql) inside a
// backup. (The .info/.dom sidecars of directory-format backups use the
// serialise_variable encoding instead; see internal/serialise.)
func ParseConfig(data []byte) map[string]string {
	out := map[string]string{}
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			continue
		}
		out[line[:eq]] = line[eq+1:]
	}
	return out
}

// sourceFor returns the archive path that contains the named top-level member.
func (b *Backup) sourceFor(name string) (string, bool) {
	for _, m := range b.Members {
		if m.Raw == name {
			if m.SourcePath != "" {
				return m.SourcePath, true
			}
			return b.Path, true
		}
	}
	return "", false
}

// WalkMember walks the source archive that holds the named top-level member and
// invokes fn on its entry. It is the directory-aware replacement for routing a
// read to the right per-domain archive.
func (b *Backup) WalkMember(name string, fn func(archive.Entry) error) error {
	src, ok := b.sourceFor(name)
	if !ok {
		return fmt.Errorf("member %q not found", name)
	}
	return archive.WalkFile(src, func(e archive.Entry) error {
		if e.Name != name {
			return nil
		}
		if err := fn(e); err != nil {
			return err
		}
		return archive.SkipEntry
	})
}

// ReadMember returns the full contents of a single top-level member. It is
// intended for the small metadata files, not for streaming large entries.
func (b *Backup) ReadMember(name string) ([]byte, error) {
	var data []byte
	found := false
	err := b.WalkMember(name, func(e archive.Entry) error {
		found = true
		d, err := io.ReadAll(e.Reader)
		if err != nil {
			return err
		}
		data = d
		return nil
	})
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("member %q not found", name)
	}
	return data, nil
}

// DomainMeta returns a domain's key=value metadata. It prefers the .dom
// serialise_variable sidecar of a directory-format backup, falling back to the
// <domain>_virtualmin member written inside every backup.
func (b *Backup) DomainMeta(domain string) (map[string]string, error) {
	if raw, ok := b.domSidecars[domain]; ok {
		if conf, err := domFromSidecar(raw, domain); err == nil {
			return conf, nil
		}
		// Fall through to the in-archive member on a decode failure.
	}
	data, err := b.ReadMember(domain + "_virtualmin")
	if err != nil {
		return nil, err
	}
	return ParseConfig(data), nil
}

// domFromSidecar decodes a .dom sidecar, which serialises { domain => {hash} },
// and flattens the inner scalar fields to strings.
func domFromSidecar(raw []byte, domain string) (map[string]string, error) {
	v, err := serialise.Unserialise(string(raw))
	if err != nil {
		return nil, err
	}
	top, ok := v.(map[string]any)
	if !ok {
		return nil, fmt.Errorf(".dom sidecar is %T, want hash", v)
	}
	inner, ok := top[domain].(map[string]any)
	if !ok {
		// Some sidecars key by the domain's own "dom" value; take the sole entry.
		for _, val := range top {
			if m, ok := val.(map[string]any); ok {
				inner = m
				break
			}
		}
	}
	if inner == nil {
		return nil, fmt.Errorf(".dom sidecar has no domain hash for %q", domain)
	}
	conf := make(map[string]string, len(inner))
	for k, val := range inner {
		if s, ok := val.(string); ok {
			conf[k] = s
		}
	}
	return conf, nil
}

package backup

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/virtualmin/virtualmin-backup-browser/internal/archive"
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

// ReadMember returns the full contents of a single top-level member. It is
// intended for the small metadata files, not for streaming large entries.
func ReadMember(backupPath, name string) ([]byte, error) {
	var data []byte
	found := false
	err := archive.WalkFile(backupPath, func(e archive.Entry) error {
		if e.Name != name {
			return nil
		}
		found = true
		b, err := io.ReadAll(e.Reader)
		if err != nil {
			return err
		}
		data = b
		return archive.SkipEntry
	})
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, fmt.Errorf("member %q not found", name)
	}
	return data, nil
}

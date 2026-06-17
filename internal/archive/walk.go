package archive

import (
	"archive/tar"
	"archive/zip"
	"errors"
	"io"
	"os"
	"strings"
)

// Entry is one member of an archive presented to a Walk callback. Reader is
// valid only for the duration of the callback; for streaming (tar) formats it
// reads directly from the underlying stream and must not be retained.
type Entry struct {
	Name   string // cleaned member name, leading "./" stripped
	Size   int64
	IsDir  bool
	Reader io.Reader
}

// SkipEntry can be returned from a Walk callback to stop iteration early
// without reporting an error (e.g. once a sought entry has been found).
var SkipEntry = errors.New("skip remaining entries")

// WalkFile opens the backup file at path, detects its container format, and
// invokes fn for each member. Directory members are reported with IsDir set and
// a nil-equivalent (empty) Reader.
func WalkFile(path string, fn func(Entry) error) error {
	format, err := DetectFile(path)
	if err != nil {
		return err
	}
	if format.IsZip() {
		return walkZip(path, fn)
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	dr, closer, err := decompress(f, format)
	if err != nil {
		return err
	}
	if closer != nil {
		defer closer.Close()
	}
	return WalkTar(dr, fn)
}

// WalkTar iterates a (decompressed) tar stream. Exposed so callers can recurse
// into a nested tar member discovered during an outer walk.
func WalkTar(r io.Reader, fn func(Entry) error) error {
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		name := cleanName(hdr.Name)
		if name == "" {
			continue // the "." root entry
		}
		isDir := hdr.Typeflag == tar.TypeDir || strings.HasSuffix(hdr.Name, "/")
		// Only emit regular files and directories; skip the GNU incremental
		// and other special typeflags, which carry no browseable content.
		if !isDir && hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeRegA {
			continue
		}
		e := Entry{Name: name, Size: hdr.Size, IsDir: isDir, Reader: tr}
		if err := fn(e); err != nil {
			if err == SkipEntry {
				return nil
			}
			return err
		}
	}
}

func walkZip(path string, fn func(Entry) error) error {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return err
	}
	defer zr.Close()
	for _, zf := range zr.File {
		name := cleanName(zf.Name)
		if name == "" {
			continue
		}
		isDir := zf.FileInfo().IsDir()
		var rc io.ReadCloser
		if !isDir {
			rc, err = zf.Open()
			if err != nil {
				return err
			}
		}
		e := Entry{Name: name, Size: int64(zf.UncompressedSize64), IsDir: isDir}
		if rc != nil {
			e.Reader = rc
		}
		cberr := fn(e)
		if rc != nil {
			rc.Close()
		}
		if cberr != nil {
			if cberr == SkipEntry {
				return nil
			}
			return cberr
		}
	}
	return nil
}

// WalkNested iterates a nested archive whose bytes come from r. The compression
// is sniffed from the stream. Only tar-based nested archives are supported;
// nested zip members require random access the stream cannot provide.
func WalkNested(r io.Reader, fn func(Entry) error) error {
	format, sr, err := sniffReader(r)
	if err != nil {
		return err
	}
	if format.IsZip() {
		return errors.New("nested zip archives are not yet supported")
	}
	dr, closer, err := decompress(sr, format)
	if err != nil {
		return err
	}
	if closer != nil {
		defer closer.Close()
	}
	return WalkTar(dr, fn)
}

// cleanName strips a leading "./" and trailing "/" so member names are
// consistent across tar (which records "./name") and zip (which records "name").
func cleanName(name string) string {
	name = strings.TrimPrefix(name, "./")
	name = strings.TrimSuffix(name, "/")
	if name == "." {
		return ""
	}
	return name
}

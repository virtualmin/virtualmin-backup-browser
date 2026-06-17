package archive

import (
	"bufio"
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"os"

	"github.com/klauspost/compress/zstd"
)

// Format identifies the outer compression/container of a backup file.
type Format int

const (
	FormatTar   Format = iota // uncompressed tar (or unknown -> assume tar)
	FormatGzip                // gzip-compressed tar
	FormatBzip2               // bzip2-compressed tar
	FormatZstd                // zstd-compressed tar
	FormatZip                 // zip archive (self-contained)
)

func (f Format) String() string {
	switch f {
	case FormatTar:
		return "tar"
	case FormatGzip:
		return "tar.gz"
	case FormatBzip2:
		return "tar.bz2"
	case FormatZstd:
		return "tar.zst"
	case FormatZip:
		return "zip"
	default:
		return "unknown"
	}
}

// IsZip reports whether the format is a zip container, which is random-access
// and handled differently from the streaming tar formats.
func (f Format) IsZip() bool { return f == FormatZip }

// detectMagic identifies the format from a file's leading bytes. These are the
// same signatures Virtualmin checks in virtual-server-lib-funcs.pl. A buffer
// that matches nothing is assumed to be an uncompressed tar.
func detectMagic(magic []byte) Format {
	switch {
	case len(magic) >= 2 && magic[0] == 0x1f && magic[1] == 0x8b:
		return FormatGzip
	case len(magic) >= 3 && magic[0] == 'B' && magic[1] == 'Z' && magic[2] == 'h':
		return FormatBzip2
	case len(magic) >= 4 && magic[0] == 0x28 && magic[1] == 0xb5 && magic[2] == 0x2f && magic[3] == 0xfd:
		return FormatZstd
	case len(magic) >= 2 && magic[0] == 'P' && magic[1] == 'K':
		return FormatZip
	default:
		return FormatTar
	}
}

// DetectFile returns the format of the file at path by sniffing its magic bytes.
func DetectFile(path string) (Format, error) {
	f, err := os.Open(path)
	if err != nil {
		return FormatTar, err
	}
	defer f.Close()
	var magic [4]byte
	n, err := io.ReadFull(f, magic[:])
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return FormatTar, err
	}
	return detectMagic(magic[:n]), nil
}

// decompress wraps a reader so the underlying tar stream can be read. It is not
// used for zip, which is opened via archive/zip directly. The returned closer
// releases any resources the decompressor holds (nil for those that hold none).
func decompress(r io.Reader, f Format) (io.Reader, io.Closer, error) {
	switch f {
	case FormatTar:
		return r, nil, nil
	case FormatGzip:
		gz, err := gzip.NewReader(r)
		if err != nil {
			return nil, nil, fmt.Errorf("gzip: %w", err)
		}
		return gz, gz, nil
	case FormatBzip2:
		return bzip2.NewReader(r), nil, nil
	case FormatZstd:
		zr, err := zstd.NewReader(r)
		if err != nil {
			return nil, nil, fmt.Errorf("zstd: %w", err)
		}
		return zr.IOReadCloser(), zr.IOReadCloser(), nil
	default:
		return nil, nil, fmt.Errorf("format %s is not a streaming tar format", f)
	}
}

// sniffReader detects the format of a stream and returns a reader positioned at
// the start. Used when walking nested archives whose compression we don't know
// from a filename.
func sniffReader(r io.Reader) (Format, io.Reader, error) {
	br := bufio.NewReader(r)
	magic, err := br.Peek(4)
	if err != nil && err != io.EOF && err != bufio.ErrBufferFull {
		// A short read still lets us classify on what we have.
		if len(magic) == 0 {
			return FormatTar, br, nil
		}
	}
	return detectMagic(magic), br, nil
}

// Package serialise decodes Webmin's serialise_variable() format, used by
// Virtualmin for the .info and .dom metadata files that accompany a backup.
//
// The format is a recursive, comma-separated, URL-encoded encoding produced by
// web-lib-funcs.pl:serialise_variable. Each value is rendered as
// "<TYPE>,<payload>" where TYPE is one of VAL, SCALAR, ARRAY, HASH, REF or
// UNDEF. Container payloads are the child values, each individually serialised
// and then url-encoded so their commas cannot collide with the separators of
// the enclosing level.
package serialise

import (
	"fmt"
	"strconv"
	"strings"
)

// Unserialise decodes a serialise_variable() string into a Go value:
//
//	VAL / SCALAR  -> string
//	ARRAY         -> []any
//	HASH / OBJECT -> map[string]any
//	UNDEF         -> nil
//
// It is the inverse of Webmin's unserialise_variable for the cases Virtualmin
// emits. The Data::Dumper variant ($VAR1 = ...) is not supported; Virtualmin
// writes backup metadata with the plain encoder.
func Unserialise(s string) (any, error) {
	v, err := decode(s)
	if err != nil {
		return nil, err
	}
	return v, nil
}

func decode(s string) (any, error) {
	if strings.HasPrefix(s, "$VAR1") {
		return nil, fmt.Errorf("Data::Dumper format is not supported")
	}
	parts := strings.Split(s, ",")
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty value")
	}
	switch parts[0] {
	case "UNDEF":
		return nil, nil
	case "VAL", "SCALAR":
		// Re-split without limit so a missing payload decodes to "".
		if len(parts) < 2 {
			return "", nil
		}
		return unURLize(parts[1]), nil
	case "ARRAY":
		out := make([]any, 0, len(parts)-1)
		for _, p := range parts[1:] {
			child, err := decode(unURLize(p))
			if err != nil {
				return nil, err
			}
			out = append(out, child)
		}
		return out, nil
	case "REF":
		if len(parts) < 2 {
			return nil, nil
		}
		return decode(unURLize(parts[1]))
	default:
		// HASH, or "OBJECT <class>" which is treated as a hash. The tag may
		// contain a url-encoded space, so anything that isn't a known scalar
		// or array tag is decoded as a key/value map.
		out := make(map[string]any)
		for i := 1; i+1 < len(parts); i += 2 {
			key, err := decode(unURLize(parts[i]))
			if err != nil {
				return nil, err
			}
			val, err := decode(unURLize(parts[i+1]))
			if err != nil {
				return nil, err
			}
			out[toString(key)] = val
		}
		return out, nil
	}
}

func toString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", t)
	}
}

// unURLize reverses Webmin's urlize: "+" becomes space, then %XX byte escapes
// are decoded. Bytes are reassembled before interpreting as UTF-8, matching
// Perl's byte-oriented pack("c", hex).
func unURLize(s string) string {
	if !strings.ContainsAny(s, "+%") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '+':
			b.WriteByte(' ')
		case c == '%' && i+2 < len(s):
			if n, err := strconv.ParseUint(s[i+1:i+3], 16, 8); err == nil {
				b.WriteByte(byte(n))
				i += 2
				continue
			}
			b.WriteByte(c)
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

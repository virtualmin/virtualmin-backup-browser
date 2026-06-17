package serialise

import (
	"reflect"
	"testing"
)

// Fixtures generated with Webmin's actual serialise_variable (web-lib-funcs.pl)
// so the decoder is tested against ground truth, not a re-implementation.

func TestUnserialiseInfo(t *testing.T) {
	const in = `HASH,VAL%2Csub%255Fdom%252Eexample%252Ecom,ARRAY%2CVAL%252Cdir,VAL%2Cexample%252Ecom,ARRAY%2CVAL%252Cmail%2CVAL%252Cdir%2CVAL%252Cmysql`
	got, err := Unserialise(in)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"example.com":         []any{"mail", "dir", "mysql"},
		"sub_dom.example.com": []any{"dir"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %#v\nwant %#v", got, want)
	}
}

func TestUnserialiseDom(t *testing.T) {
	const in = `HASH,VAL%2Cexample%252Ecom,HASH%2CVAL%252Cuser%2CVAL%252Cexampleco%2CVAL%252Cmail%2CVAL%252C1%2CVAL%252Chome%2CVAL%252C%25252Fhome%25252Fexampleco%2CVAL%252Cnote%2CVAL%252Ca%25252C%252520b%252520%252526%252520c%2CVAL%252Cdom%2CVAL%252Cexample%25252Ecom%2CVAL%252Cuid%2CVAL%252C5001`
	got, err := Unserialise(in)
	if err != nil {
		t.Fatal(err)
	}
	top, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("top level is %T, want map", got)
	}
	dom, ok := top["example.com"].(map[string]any)
	if !ok {
		t.Fatalf("example.com is %T, want map", top["example.com"])
	}
	want := map[string]any{
		"user": "exampleco",
		"mail": "1",
		"home": "/home/exampleco",
		"note": "a, b & c", // exercises encoded comma, space and ampersand
		"dom":  "example.com",
		"uid":  "5001",
	}
	if !reflect.DeepEqual(dom, want) {
		t.Errorf("got  %#v\nwant %#v", dom, want)
	}
}

func TestUnserialiseScalarAndUndef(t *testing.T) {
	got, err := Unserialise(`VAL,plain%20value%20with%20spaces%2C%20commas%20%26%20%25`)
	if err != nil {
		t.Fatal(err)
	}
	if got != "plain value with spaces, commas & %" {
		t.Errorf("got %q", got)
	}

	got, err = Unserialise("UNDEF")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Errorf("UNDEF decoded to %#v, want nil", got)
	}
}

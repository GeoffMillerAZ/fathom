package extractor

import (
	"reflect"
	"testing"
)

// TestDocValueCut (LB-F126): a value alternative may end with ` cut:<marker>`,
// which keeps the text before the marker's first occurrence. A Conan reference
// `zlib/1.3.1#rev%ts` in a lockfile names the recipe `zlib`: the name of an
// entity, a reference and a cross-file target is read through it, and a text
// without the marker or starting with it is left as written.
func TestDocValueCut(t *testing.T) {
	x := mustDoc(t, `{"decode":"json","detect":{},"entities":[
  {"select":"$","kind":"package","source_kind":"Lock","name":"$file"}],
 "references":[
  {"from":"package","select":"$.requires[*]","value":"$ cut:/","to_kind":"package","edge":"requires"}],
 "cross_file":[
  {"select":"$.requires[*]","value":"$ cut:/","resolve":"module-source","edge":"requires"}]}`)
	src := `{"requires": ["zlib/1.3.1#abc%1700000000.1", "bare", "/leading", "openssl/[>=3 <4]@u/c"]}`
	r, err := x.ExtractDoc("s", "conan.lock", []byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Nodes) != 1 {
		t.Fatalf("nodes: %v", brief(r.Nodes))
	}
	if got, want := r.Nodes[0].Calls, []string{"/leading", "bare", "openssl", "zlib"}; !reflect.DeepEqual(got, want) {
		t.Errorf("calls %q, want %q", got, want)
	}
	var raws []string
	for _, c := range r.CrossFile {
		raws = append(raws, c.Raw)
	}
	if want := []string{"/leading", "bare", "openssl", "zlib"}; !reflect.DeepEqual(raws, want) {
		t.Errorf("cross-file raws %q, want %q", raws, want)
	}
}

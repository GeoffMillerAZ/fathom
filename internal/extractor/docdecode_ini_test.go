package extractor

import (
	"reflect"
	"testing"
)

// TestDecodeINIDoc (LB-F125): the "ini" decode. The root is a map from section
// name to a sequence; each non-blank, non-comment line of a section is a map
// with text (the line, trimmed), key (the text before the first "=", trimmed, or
// the whole text) and value (the text after it, only when there is an "="). A
// "#" or ";" starts a comment only at the head of a line (a Conan reference
// carries "#" as its revision marker). Lines before the first header are not
// in a section. A header written twice extends one section. Lines are 1-based.
func TestDecodeINIDoc(t *testing.T) {
	src := "# comment\r\n" +
		"stray line\n" +
		"[requires]\n" +
		"zlib/1.3.1#abc\n" +
		"  openssl/[>=3 <4]  \n" +
		"\n" +
		"; semicolon comment\n" +
		"[options]\n" +
		"zlib/*:shared=True\n" +
		"*:fPIC = False\n" +
		"[requires]\n" +
		"fmt/10.2.1\n" +
		"[layout]\n" +
		"[generators]\n" +
		"CMakeDeps\n"
	docs, err := decodeDoc("ini", []byte(src), false, "")
	if err != nil || len(docs) != 1 {
		t.Fatalf("decode: %v %d", err, len(docs))
	}
	root := docs[0]
	if want := []string{"requires", "options", "layout", "generators"}; !reflect.DeepEqual(root.keys, want) {
		t.Fatalf("sections %q, want %q", root.keys, want)
	}
	type item struct {
		text, key, value string
		line             int
		hasValue         bool
	}
	read := func(sec string) []item {
		var out []item
		for _, it := range root.get(sec).items {
			i := item{line: it.line}
			i.text = it.get("text").scalar
			i.key = it.get("key").scalar
			if v := it.get("value"); v != nil {
				i.value, i.hasValue = v.scalar, true
			}
			out = append(out, i)
		}
		return out
	}
	if got, want := read("requires"), []item{
		{"zlib/1.3.1#abc", "zlib/1.3.1#abc", "", 4, false},
		{"openssl/[>=3 <4]", "openssl/[>=3 <4]", "", 5, false},
		{"fmt/10.2.1", "fmt/10.2.1", "", 12, false},
	}; !reflect.DeepEqual(got, want) {
		t.Errorf("requires %+v, want %+v", got, want)
	}
	if got, want := read("options"), []item{
		{"zlib/*:shared=True", "zlib/*:shared", "True", 9, true},
		{"*:fPIC = False", "*:fPIC", "False", 10, true},
	}; !reflect.DeepEqual(got, want) {
		t.Errorf("options %+v, want %+v", got, want)
	}
	if n := root.get("layout"); n == nil || len(n.items) != 0 || n.line != 13 {
		t.Errorf("layout %+v", n)
	}
	if g := root.get("generators"); g == nil || g.line != 14 || g.endLine != 15 {
		t.Errorf("generators lines %+v", g)
	}
	if root.get("requires").line != 3 {
		t.Errorf("requires header line %d, want 3 (first header)", root.get("requires").line)
	}
	if root.line != 1 || root.endLine != 15 {
		t.Errorf("root lines %d-%d", root.line, root.endLine)
	}
}

// TestDecodeINIDocEmpty: no section is a document with no entries, not an error.
func TestDecodeINIDocEmpty(t *testing.T) {
	docs, err := decodeDoc("ini", []byte("just text\n# c\n"), false, "")
	if err != nil || len(docs) != 1 || len(docs[0].keys) != 0 {
		t.Fatalf("got %v %v", docs, err)
	}
}

// TestDocDecodeFiles (LB-F125): a spec's decode_files names the decode of a
// basename that differs from the spec's own, so one dialect reads a YAML file
// and an INI file (Conan's conandata.yml and conanfile.txt). An unknown decode is refused.
func TestDocDecodeFiles(t *testing.T) {
	x := mustDoc(t, `{"decode":"yaml","decode_files":{"conanfile.txt":"ini"},"detect":{},"entities":[
  {"select":"$","files":["conanfile.txt"],"kind":"package","source_kind":"Txt","name":"$file"},
  {"select":"$.sources.*","files":["conandata.yml"],"kind":"key","source_kind":"Src","name":"$key"}],
 "references":[
  {"from":"package","select":"$.requires[*].key","value":"$ cut:/","to_kind":"package","edge":"requires"}]}`)
	txt, err := x.ExtractDoc("s", "p/conanfile.txt", []byte("[requires]\nzlib/1.3.1\nfmt/10\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got := brief(txt.Nodes); !reflect.DeepEqual(got, []string{"Txt conanfile.txt@1-3 calls=fmt,zlib"}) {
		t.Errorf("conanfile.txt: %q", got)
	}
	yml, err := x.ExtractDoc("s", "p/conandata.yml", []byte("sources:\n  \"1.3.1\":\n    url: u\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got := brief(yml.Nodes); !reflect.DeepEqual(got, []string{"Src 1.3.1@2-3"}) {
		t.Errorf("conandata.yml: %q", got)
	}
	for _, bad := range []string{
		`{"decode":"yaml","decode_files":{"a":"nope"},"detect":{},"entities":[]}`,
	} {
		s, err := ParseDocSpec([]byte(bad))
		if err != nil {
			continue
		}
		if _, err := NewDocExtractor(s); err == nil {
			t.Errorf("spec %s: accepted", bad)
		}
	}
}

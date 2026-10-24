package extractor

import (
	"reflect"
	"strings"
	"testing"
)

// TestDecodeLibconfigDoc (LB-Q-proxysql): the "libconfig" decode, the grammar
// ProxySQL's proxysql.cnf is written in. A group is a map, a list and an array
// are sequences, a scalar is its text (a string without quotes, adjacent strings
// joined, a number as written). A setting's line is where its name is written, a
// list item's where its "{" is, and a collection ends at its closing bracket.
// Comments are "#", "//" and "/* */"; ":" or "=" separates a name; ";" or ","
// may end a setting; "@include" is dropped.
func TestDecodeLibconfigDoc(t *testing.T) {
	src := "# top\n" +
		"datadir=\"/var/lib/\"\n" + // 2
		"flag : true;\n" + // 3
		"/* block\n" + // 4
		"   comment */\n" +
		"admin_variables =\n" + // 6
		"{\n" +
		"\tcreds=\"a:b\" \"c\" // trailing\n" + // 8
		"\tn = -0x1F, ratio=1.5e3\n" + // 9
		"}\n" + // 10
		"@include \"other.cnf\"\n" + // 11
		"mysql_servers =\n" + // 12
		"(\n" +
		"\t{ address=\"h1\" , port=3306, hostgroup=0 },\n" + // 14
		"\t{\n" + // 15
		"\t\taddress=\"h2\"\n" +
		"\t\tport=3307\n" +
		"\t\ttags = [ \"a\", \"b\" ]\n" + // 18
		"\t}\n" + // 19
		")\n" + // 20
		"empty = ()\n" // 21
	docs, err := decodeDoc("libconfig", []byte(src), false, "")
	if err != nil || len(docs) != 1 {
		t.Fatalf("decode: %v %d", err, len(docs))
	}
	root := docs[0]
	if want := []string{"datadir", "flag", "admin_variables", "mysql_servers", "empty"}; !reflect.DeepEqual(root.keys, want) {
		t.Fatalf("keys %q, want %q", root.keys, want)
	}
	scalar := func(n *docNode) string {
		if n == nil || n.kind != docScalar {
			return "<not a scalar>"
		}
		return n.scalar
	}
	if got := scalar(root.get("datadir")); got != "/var/lib/" {
		t.Errorf("datadir = %q", got)
	}
	if got := scalar(root.get("flag")); got != "true" {
		t.Errorf("flag = %q", got)
	}
	av := root.get("admin_variables")
	if av.kind != docMap || av.line != 6 || av.endLine != 10 {
		t.Errorf("admin_variables: kind %v lines %d-%d, want a map on 6-10", av.kind, av.line, av.endLine)
	}
	if got := scalar(av.get("creds")); got != "a:bc" {
		t.Errorf("creds = %q, want adjacent strings joined", got)
	}
	if got := scalar(av.get("n")); got != "-0x1F" {
		t.Errorf("n = %q", got)
	}
	if got := scalar(av.get("ratio")); got != "1.5e3" || av.get("ratio").line != 9 {
		t.Errorf("ratio = %q on line %d", got, av.get("ratio").line)
	}
	ms := root.get("mysql_servers")
	if ms.kind != docSeq || ms.line != 12 || ms.endLine != 20 || len(ms.items) != 2 {
		t.Fatalf("mysql_servers: kind %v lines %d-%d, %d items", ms.kind, ms.line, ms.endLine, len(ms.items))
	}
	if a, b := ms.items[0], ms.items[1]; a.line != 14 || a.endLine != 14 || b.line != 15 || b.endLine != 19 {
		t.Errorf("item lines %d-%d and %d-%d, want 14-14 and 15-19", a.line, a.endLine, b.line, b.endLine)
	}
	if got := scalar(ms.items[0].get("port")); got != "3306" {
		t.Errorf("port = %q", got)
	}
	if tags := ms.items[1].get("tags"); tags == nil || tags.kind != docSeq || len(tags.items) != 2 || tags.items[1].scalar != "b" {
		t.Errorf("tags = %+v", tags)
	}
	if e := root.get("empty"); e.kind != docSeq || len(e.items) != 0 || e.line != 21 {
		t.Errorf("empty = %+v", e)
	}
	for _, bad := range []string{"a = {", "a = (1,", "a = \"open", "= 1", "a 1", "a = GET", "a = 12abc"} {
		if _, err := decodeDoc("libconfig", []byte(bad), false, ""); err == nil {
			t.Errorf("%q decoded, want an error", strings.TrimSpace(bad))
		}
	}
}

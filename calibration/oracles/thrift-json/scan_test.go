package main

import (
	"strconv"
	"strings"
	"testing"
)

const sample = `# hash comment
include "shared.thrift" // trailing
cpp_include "x.h"
namespace java com.acme /* inline */
namespace * plain
typedef map<string, list<shared.Money>> Ledger (cpp.type = "std::map")
const double BIG = 1.5e+308
enum E { A = 1, B (x = "y"); C }
struct S {
  1: required shared.Money m = {"a": 1},
  2: RecList & next,
  uuid id
}
service X extends shared.Base {
  S get(1: shared.Money m) throws (1: Err e),
  oneway void ping();
  void done()
}
`

func TestScan(t *testing.T) {
	doc, err := Scan(sample)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, d := range doc.Decls {
		refs := make([]string, 0, len(d.Refs))
		for _, r := range d.Refs {
			refs = append(refs, r.Written)
		}
		got = append(got, strings.Join([]string{d.Kind, d.Container, d.Name, strconv.Itoa(d.Line), strconv.Itoa(d.End), strings.Join(refs, "+")}, "|"))
	}
	want := []string{
		"namespace||java|4|4|", "namespace||*|5|5|",
		"typedef||Ledger|6|6|shared.Money",
		"const||BIG|7|7|",
		"enum||E|8|8|", "enum_value|E|A|8|8|", "enum_value|E|B|8|8|", "enum_value|E|C|8|8|",
		"struct||S|9|13|",
		"field|S|m|10|10|shared.Money", "field|S|next|11|11|RecList", "field|S|id|12|12|",
		"service||X|14|18|shared.Base",
		"function|X|get|15|15|S+shared.Money+Err", "function|X|ping|16|16|", "function|X|done|17|17|",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	if len(doc.Includes) != 1 || doc.Includes[0].Path != "shared.thrift" || doc.Includes[0].Line != 2 {
		t.Errorf("includes = %+v", doc.Includes)
	}
}

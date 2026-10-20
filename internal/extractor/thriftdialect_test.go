package extractor

// thriftdialect_test.go -- LB-Q-thrift (design 13.2 steps 10 and 12). The per-fixture goldens run in the generic
// TestDialectGoldens (dialects/thrift/testdata). This file pins the contracts the goldens cannot: the SourceKind
// strings, a member qualifying under its container, the references a field, a function and a service make (a
// qualified type keeps its qualifier, uuid is none), an include being an import and a cpp_include not, the .thrift extension for
// any directory, determinism and an empty file.

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

const thriftContractFixture = `namespace go acme
include "types.thrift"
cpp_include "legacy.h"

enum Level { LOW = 1, HIGH }

struct Job {
  1: required types.Id id,
  2: Level level,
  3: list<Job> children,
  4: uuid token
}

service Runner extends types.Base {
  Job run(1: types.Id id) throws (1: types.Failure f),
  oneway void stop()
}
`

func thriftDialect(t *testing.T) *TSExtractor {
	t.Helper()
	d, err := NewTSExtractor("thrift")
	if err != nil {
		t.Fatalf("NewTSExtractor(thrift): %v", err)
	}
	t.Cleanup(d.Close)
	return d
}

// TestThriftDialectKindsAndReferences: every kind, a member qualified under its container, and the references each
// definition makes -- `types.Id` stays qualified, a built-in type is none, `extends` names its service.
func TestThriftDialectKindsAndReferences(t *testing.T) {
	for _, rel := range []string{"job.thrift", "idl/api/job.thrift"} {
		nodes, err := thriftDialect(t).Extract("svc", rel, []byte(thriftContractFixture))
		if err != nil {
			t.Fatalf("Extract %s: %v", rel, err)
		}
		by := map[string]core.SymbolNode{}
		for _, n := range nodes {
			by[string(n.Kind)+":"+n.Name] = n
		}
		for _, key := range []string{
			"ThriftNamespace:go", "ThriftEnum:Level", "ThriftEnumValue:Level.HIGH", "ThriftStruct:Job", "ThriftField:Job.id",
			"ThriftService:Runner", "ThriftFunction:Runner.run", "ThriftFunction:Runner.stop",
		} {
			n, ok := by[key]
			if !ok {
				t.Errorf("%s: no %s", rel, key)
				continue
			}
			if n.Tier != core.TierVerified || n.EndLine < n.Line {
				t.Errorf("%s: %s tier %q lines %d-%d", rel, key, n.Tier, n.Line, n.EndLine)
			}
		}
		for key, calls := range map[string]string{
			"ThriftField:Job.id":         "types.Id",
			"ThriftField:Job.level":      "Level",
			"ThriftField:Job.children":   "Job",
			"ThriftService:Runner":       "Base",
			"ThriftFunction:Runner.run":  "Job,types.Failure,types.Id",
			"ThriftFunction:Runner.stop": "",
		} {
			if got := strings.Join(by[key].Calls, ","); got != calls {
				t.Errorf("%s: %s references %q, want %q", rel, key, got, calls)
			}
		}
	}
}

// TestThriftDialectIncludes: an include is a whole-file import of its path and a cpp_include (a C++ header) is none.
func TestThriftDialectIncludes(t *testing.T) {
	specs, err := thriftDialect(t).ExtractImports("job.thrift", []byte(thriftContractFixture))
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 1 || specs[0].Module != "types.thrift" || specs[0].Line != 2 {
		t.Errorf("imports = %+v, want one include of types.thrift at line 2", specs)
	}
}

// TestThriftDialectDeterministicAndEmpty: two extractions agree and an empty file extracts nothing.
func TestThriftDialectDeterministicAndEmpty(t *testing.T) {
	d := thriftDialect(t)
	n1, err1 := d.Extract("svc", "job.thrift", []byte(thriftContractFixture))
	n2, err2 := d.Extract("svc", "job.thrift", []byte(thriftContractFixture))
	if err1 != nil || err2 != nil || len(n1) != len(n2) {
		t.Fatalf("non-deterministic: %v %v %d vs %d", err1, err2, len(n1), len(n2))
	}
	for i := range n1 {
		if n1[i].Name != n2[i].Name || n1[i].Kind != n2[i].Kind || n1[i].Line != n2[i].Line {
			t.Errorf("non-deterministic at [%d]: %+v vs %+v", i, n1[i], n2[i])
		}
	}
	if nodes, err := d.Extract("svc", "empty.thrift", []byte("")); err != nil || len(nodes) != 0 {
		t.Errorf("empty input: %d nodes, err %v; want 0, nil", len(nodes), err)
	}
}

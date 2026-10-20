package fusion_test

// LB-Q-thrift (L3): a Thrift `include "x.thrift"` names one file, relative to the including file's directory first
// and then to the repository root, and a type written through it (`types.Money`) binds to the type that file
// defines. An unqualified name is the file's own type in Thrift, so `Money` in a file that defines its own Money
// draws no edge to the included file's Money, and a cpp_include names a header, not a Thrift file.

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/fusion"
)

func TestResolveFileEdges_ThriftIncludes(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("testdata", "thriftfixture"))
	if err != nil {
		t.Fatal(err)
	}
	files, symbols := loadFileResolveFixture(t, dir, "thrift", fixtureFiles(t, dir, ".thrift"))
	edges, report := fusion.ResolveFileEdges(context.Background(), "thrift", dir, files, symbols)
	if report.Skipped {
		t.Fatalf("skipped: %s", report.SkipReason)
	}
	want := []string{
		"api/audit.thrift|Audited.last -> api/audit.thrift|Stamp", // an unqualified name is the file's own type
		"api/audit.thrift|Audited.last -> common/types.thrift|NotFound",
		"api/audit.thrift|Stamp.fee -> common/types.thrift|Money",
		"api/orders.thrift|Order.local -> api/orders.thrift|Money", // not the included file's Money
		"api/orders.thrift|Order.stamp -> api/audit.thrift|Stamp",
		"api/orders.thrift|Order.total -> common/types.thrift|Money",
		"api/orders.thrift|Orders -> api/audit.thrift|Audited",
		"api/orders.thrift|Orders.get -> api/orders.thrift|Order",
		"api/orders.thrift|Orders.get -> common/types.thrift|NotFound",
	}
	got := namedEdges(edges, symbols)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("edges:\n  %s\nwant:\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}

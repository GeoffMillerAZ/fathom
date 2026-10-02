package fusion_test

// LB-F77 (O274): a protobuf import names a path under a module root, not the
// repository root — `import "buf/validate/validate.proto";` lives at
// vendor/protovalidate/buf/validate/validate.proto when buf.yaml (v2) lists
// `modules: [{path: vendor/protovalidate}]`, or under a buf.work.yaml
// directory (v1). The protobuf manifest's imports.module_roots "buf" looks an
// import up under each such root, so the message types it brings into scope
// resolve: User's fields reach Common and FieldRules. Under type_paths
// "package-dir" a type reference is read with its package
// (`acme.v2.Common.Kind`: the nested enum of the Common in directory
// acme/v2), and an unqualified one (`Common`) in the referring file's own
// package first — the acme/v1 Common, though acme/v2 declares one too.

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/fusion"
)

func TestResolveFileEdges_BufModuleRoots(t *testing.T) {
	for _, c := range []struct {
		name, dir string
		want      []string
	}{
		{"buf.yaml v2 modules", "buffixture", []string{
			"proto/acme/v1/user.proto|User -> proto/acme/v1/common.proto|Common",
			"proto/acme/v1/user.proto|User -> proto/acme/v2/common.proto|Common.Kind",
			"proto/acme/v1/user.proto|User -> vendor/protovalidate/buf/validate/validate.proto|FieldRules",
		}},
		{"buf.work.yaml directories", "bufworkfixture", []string{
			"proto/acme/v1/user.proto|User -> proto/acme/v1/common.proto|Common",
			"proto/acme/v1/user.proto|User -> third_party/buf/validate/validate.proto|FieldRules",
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir, err := filepath.Abs(filepath.Join("testdata", c.dir))
			if err != nil {
				t.Fatal(err)
			}
			files, symbols := loadFileResolveFixture(t, dir, "protobuf", fixtureFiles(t, dir, ".proto"))
			edges, report := fusion.ResolveFileEdges(context.Background(), "protobuf", dir, files, symbols)
			if report.Skipped {
				t.Fatalf("skipped: %s", report.SkipReason)
			}
			got := namedEdges(edges, symbols)
			if strings.Join(got, "\n") != strings.Join(c.want, "\n") {
				t.Errorf("edges:\n  %s\nwant:\n  %s", strings.Join(got, "\n  "), strings.Join(c.want, "\n  "))
			}
		})
	}
}

package lang_test

import (
	"context"
	"testing"

	"github.com/geoffmilleraz/fathom/pkg/lang"
)

// This proves actual public dispatch, not independent template evaluation.
func TestAzurePipeline211PublicDispatch(t *testing.T) {
	r := newReader(t)
	got, err := r.Extract(context.Background(), lang.File{Path: "azure-pipelines.yml", Src: []byte("stages:\n- stage: Build\n  jobs:\n  - job: Compile\n    steps:\n    - script: echo ok\n")})
	if err != nil {
		t.Fatal(err)
	}
	if got.Dialect != "azure-pipelines" || got.Skipped != "" || got.Incomplete {
		t.Fatalf("dispatch=%s skipped=%s incomplete=%v", got.Dialect, got.Skipped, got.Incomplete)
	}
	found := false
	for _, s := range got.Symbols {
		if s.Name == "Build" && s.Line == 2 {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing Build stage: %+v", got.Symbols)
	}
}

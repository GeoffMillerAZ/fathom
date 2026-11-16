package lang_test

import (
	"context"
	"testing"

	"github.com/geoffmilleraz/fathom/pkg/lang"
)

// This proves public dispatch, never independent calibration truth.
func TestJenkinsfile211PublicDispatch(t *testing.T) {
	r := newReader(t)
	for _, path := range []string{"Jenkinsfile", "probe.jenkinsfile"} {
		got, err := r.Extract(context.Background(), lang.File{Path: path, Src: []byte("pipeline {\n agent any\n stages {\n stage('Build') {\n steps { echo 'ok' }\n }\n }\n}\n")})
		if err != nil {
			t.Fatal(err)
		}
		if got.Dialect != "jenkinsfile" || got.Skipped != "" || got.Incomplete {
			t.Fatalf("%s: dispatch=%s skipped=%s incomplete=%v", path, got.Dialect, got.Skipped, got.Incomplete)
		}
		found := false
		for _, s := range got.Symbols {
			if s.Name == "Build" && s.Line == 4 && s.EndLine == 6 {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s: missing positioned Build stage: %+v", path, got.Symbols)
		}
	}
}

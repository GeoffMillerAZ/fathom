package lang_test

import (
	"bytes"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

var updateAPI = flag.Bool("update-api", false, "rewrite the go doc -all snapshots under testdata/api")

// LL-22 (design section 8, row 3): the exported API is pinned by a go doc -all
// snapshot, so a break is a deliberate edit of a reviewed file and never a side
// effect. After an intended change, run the test with -update-api, read the
// diff of testdata/api as the API review, and commit it with the change.
func TestLangAPISnapshot(t *testing.T) {
	for _, pkg := range []struct{ dir, golden string }{
		{".", "lang.txt"},
		{"./cue", "cue.txt"},
		{"./iac", "iac.txt"},
		{"./langtest", "langtest.txt"},
	} {
		cmd := exec.Command("go", "doc", "-all", pkg.dir)
		cmd.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=")
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		got, err := cmd.Output()
		if err != nil {
			t.Fatalf("go doc -all %s: %v\n%s", pkg.dir, err, stderr.String())
		}
		if len(got) < 200 {
			t.Fatalf("go doc -all %s printed %d bytes", pkg.dir, len(got))
		}
		path := filepath.Join("testdata", "api", pkg.golden)
		if *updateAPI {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, got, 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%v (run with -update-api to write the snapshot)", err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("the exported API of %s changed. Review the change, then run: go test ./pkg/lang -run TestLangAPISnapshot -args -update-api\n%s", pkg.dir, firstDiff(want, got))
		}
	}
}

// firstDiff names the first line where two texts differ.
func firstDiff(want, got []byte) string {
	w, g := bytes.Split(want, []byte("\n")), bytes.Split(got, []byte("\n"))
	for i := 0; i < len(w) || i < len(g); i++ {
		var a, b []byte
		if i < len(w) {
			a = w[i]
		}
		if i < len(g) {
			b = g[i]
		}
		if !bytes.Equal(a, b) {
			return "line " + strconv.Itoa(i+1) + "\n  snapshot: " + string(a) + "\n  current:  " + string(b)
		}
	}
	return ""
}

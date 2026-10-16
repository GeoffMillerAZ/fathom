package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/cli/command"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designevidence"
)

func captureEvidenceCLI(t *testing.T, run func([]string) error, args ...string) ([]byte, error) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	done := make(chan []byte, 1)
	go func() {
		body, _ := io.ReadAll(r)
		done <- body
	}()
	runErr := run(args)
	os.Stdout = old
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	body := <-done
	_ = r.Close()
	return body, runErr
}

func TestAssetsEvidenceCLIOptionalStoreAndArguments(t *testing.T) {
	root := t.TempDir()
	body, err := captureEvidenceCLI(t, dataAssetsEvidence, "-root", root)
	if err != nil {
		t.Fatal(err)
	}
	var page designevidence.Page
	if json.Unmarshal(body, &page) != nil || page.Revision == "" || page.Total != 0 || page.Entries == nil || page.NextCursor != "" {
		t.Fatalf("missing evidence child did not produce an empty metadata page: %s", body)
	}
	if _, err := os.Stat(filepath.Join(root, "evidence")); !os.IsNotExist(err) {
		t.Fatal("read-only search created the optional evidence store")
	}

	for _, tc := range []struct {
		run  func([]string) error
		args []string
	}{
		{dataAssetsEvidence, []string{"-root", root, "-limit", "51"}},
		{dataAssetsEvidence, []string{"-root", root, "extra"}},
		{dataAssetsEvidenceGet, []string{"-root", root}},
		{dataAssetsEvidenceGet, []string{"-root", root, "-id", "bad"}},
		{dataAssetsEvidenceSave, []string{"-root", root}},
		{dataAssetsEvidenceSave, []string{"-root", root, "-bundle", "missing", "-contract-sha", strings.Repeat("0", 64), "-source-sha", strings.Repeat("0", 64)}},
	} {
		out, gotErr := captureEvidenceCLI(t, tc.run, tc.args...)
		if command.Code(gotErr) != 2 || len(out) != 0 {
			t.Fatalf("bad arguments emitted success/data: %v output=%q err=%v", tc.args, out, gotErr)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "evidence")); !os.IsNotExist(err) {
		t.Fatal("invalid commands created the evidence store")
	}
}

func TestAssetsEvidenceSaveRejectsNonRegularOrOversizedBundle(t *testing.T) {
	root, scratch := t.TempDir(), t.TempDir()
	regular := filepath.Join(scratch, "bundle.json")
	if err := os.WriteFile(regular, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	symlink := filepath.Join(scratch, "bundle-link.json")
	if err := os.Symlink(regular, symlink); err != nil {
		t.Fatal(err)
	}
	oversized := filepath.Join(scratch, "oversized.json")
	f, err := os.Create(oversized)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(int64(designevidence.MaxBundleBytes) + 1); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	pins := []string{"-contract-sha", strings.Repeat("0", 64), "-source-sha", strings.Repeat("0", 64), "-artifact-sha", strings.Repeat("0", 64)}
	for _, path := range []string{scratch, symlink, oversized} {
		args := append([]string{"-root", root, "-bundle", path}, pins...)
		body, gotErr := captureEvidenceCLI(t, dataAssetsEvidenceSave, args...)
		if command.Code(gotErr) != 2 || len(bytes.TrimSpace(body)) != 0 {
			t.Fatalf("unsafe bundle %q was not a silent could-not-check failure: %q %v", path, body, gotErr)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "evidence")); !os.IsNotExist(err) {
		t.Fatal("rejected bundle created the evidence store")
	}
}

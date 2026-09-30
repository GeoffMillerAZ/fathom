package fusion_test

// emacslisp_ingest_test.go — LB-Q-emacs-lisp: a .el file reaches the emacs-lisp dialect through discovery,
// classification and dispatch (it was disclosed as an unsupported language and fingerprinted by the
// no-extractor path). What only the dialect does — a defcustom, a defvar with no value, a definition below
// the top level, a signature that stops at the parameter list, a mode — proves the dispatch is the dialect's.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func TestIngest_EmacsLispReachesTheEmacsLispDialect(t *testing.T) {
	dir := t.TempDir()
	src := `;;; notes.el --- take notes  -*- lexical-binding: t; -*-

(defgroup notes nil
  "Take notes."
  :group 'applications)

(defcustom notes-directory "~/notes/"
  "Directory holding the notes."
  :type 'directory)

(defvar notes-declared)

(defun notes-new (title
                  &optional open)
  "Create a note called TITLE."
  (interactive "sTitle: ")
  (when open (find-file title)))

(with-eval-after-load 'org
  (defun notes-org-link (title)
    "Link to TITLE."
    title))

(define-derived-mode notes-mode text-mode "Notes"
  "Major mode for notes.")

(provide 'notes)
`
	p := filepath.Join(dir, "lisp", "notes.el")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	st := store.NewMemoryStore()
	svc := core.ServiceRef{ID: "el-svc", Path: dir}
	pl := fusion.New(connector.NewLocalConnector(), st, fusion.Config{Concurrency: 1})
	t.Cleanup(pl.Close)
	ctx := context.Background()
	if _, err := pl.Ingest(ctx, svc); err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	obs, err := st.QueryObservations(ctx, svc.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]core.Observation{}
	for _, o := range obs {
		got[o.Symbol] = o
	}
	for symbol, want := range map[string]struct {
		kind core.SourceKind
		sig  string
	}{
		"notes":           {"EmacsLispGroup", ""},
		"notes-directory": {"EmacsLispCustom", ""},
		"notes-declared":  {"EmacsLispVariable", ""},
		"notes-new":       {"EmacsLispFunction", "(defun notes-new (title &optional open)"},
		"notes-org-link":  {"EmacsLispFunction", "(defun notes-org-link (title)"},
		"notes-mode":      {"EmacsLispDerivedMode", `(define-derived-mode notes-mode text-mode "Notes"`},
	} {
		o, ok := got[symbol]
		if !ok {
			t.Errorf("no observation for %q (got %v)", symbol, keysOf(got))
			continue
		}
		if o.SourceKind != want.kind || o.Claim != want.sig || o.EndLine < o.Line {
			t.Errorf("%s: kind %q claim %q lines %d-%d, want %s %q", symbol, o.SourceKind, o.Claim, o.Line, o.EndLine, want.kind, want.sig)
		}
	}
	if o := got["notes-new"]; o.Line != 13 || o.EndLine != 17 {
		t.Errorf("notes-new lines %d-%d, want 13-17", o.Line, o.EndLine)
	}
}

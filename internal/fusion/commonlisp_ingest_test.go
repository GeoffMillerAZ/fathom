package fusion_test

// commonlisp_ingest_test.go — LB-Q-commonlisp: .lisp, .lsp and .asd files reach the commonlisp dialect through
// discovery, classification and dispatch (they were disclosed as an unsupported language and fingerprinted by the
// no-extractor path). What only the dialect does — a defgeneric and its methods told apart from a defun by the
// grammar's keyword, a defstruct, a definition below the top level, a signature that stops at the lambda list — proves
// the dispatch is the dialect's. An ASDF system file is claimed and defines nothing.

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

func TestIngest_CommonLispReachesTheCommonLispDialect(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"src/shapes.lisp": `(in-package :shapes)

(defvar *unit* 1
  "The unit length.")

(defstruct point
  (x 0)
  (y 0))

(defclass circle ()
  ((radius :initarg :radius)))

(defgeneric area (shape)
  (:documentation "Return the area of SHAPE."))

(defmethod area ((shape circle))
  (* pi (expt (slot-value shape 'radius) 2)))

(defun scale (shape
              &key (by 2))
  "Scale SHAPE by BY."
  (list shape by))

(eval-when (:compile-toplevel :load-toplevel :execute)
  (defmacro with-unit ((var) &body body)
    "Bind VAR to the unit."
    ` + "`" + `(let ((,var *unit*)) ,@body)))
`,
		"src/legacy.lsp": `(defun legacy-total (xs)
  (reduce #'+ xs))
`,
		"shapes.asd": `(asdf:defsystem "shapes"
  :components ((:file "src/shapes")))
`,
	}
	for rel, src := range files {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	st := store.NewMemoryStore()
	svc := core.ServiceRef{ID: "lisp-svc", Path: dir}
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
	// area is both a generic and a method, so observations are keyed by kind too.
	got := map[string]core.Observation{}
	for _, o := range obs {
		got[o.Symbol+"/"+string(o.SourceKind)] = o
	}
	for key, want := range map[string]string{
		"*unit*/CommonLispVariable":       "",
		"point/CommonLispStruct":          "",
		"circle/CommonLispClass":          "",
		"area/CommonLispGeneric":          "(defgeneric area (shape)",
		"area/CommonLispMethod":           "(defmethod area ((shape circle))",
		"scale/CommonLispFunction":        "(defun scale (shape &key (by 2))",
		"with-unit/CommonLispMacro":       "(defmacro with-unit ((var) &body body)",
		"legacy-total/CommonLispFunction": "(defun legacy-total (xs)",
	} {
		o, ok := got[key]
		if !ok {
			t.Errorf("no observation %q (got %v)", key, keysOf(got))
			continue
		}
		if o.Claim != want || o.EndLine < o.Line {
			t.Errorf("%s: claim %q lines %d-%d, want %q", key, o.Claim, o.Line, o.EndLine, want)
		}
	}
	for key, o := range got {
		if o.SourceRef == "shapes.asd" {
			t.Errorf("the ASDF system file yielded %q: it defines nothing the dialect extracts", key)
		}
	}
	if o := got["scale/CommonLispFunction"]; o.Line != 19 || o.EndLine != 22 {
		t.Errorf("scale lines %d-%d, want 19-22", o.Line, o.EndLine)
	}
}

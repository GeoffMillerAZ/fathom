package langbench

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestFilesWithErrorMerged (LB-F59, O203): files_with_error is optional. A
// subject measured without it (a doc or native engine, every reading
// committed before it) carries none and the reading omits the field; the
// reading's count is the sum over the subjects that report one, and a count
// above the files measured is refused.
func TestFilesWithErrorMerged(t *testing.T) {
	n := func(v int) *int { return &v }
	sub := func(name string, fwe *int) Subject {
		return Subject{Name: name, Files: 10, Bytes: 1 << 20,
			Tenancy: &SubjectTenancy{FaultingFiles: []string{}, ExtractMS: 10, ExtractCPUMS: 10, FilesWithError: fwe}}
	}
	r := NewReading(Manifest{ID: "x", Kind: "language"}, "L1")
	MergeSubject(&r, sub("a", nil))
	b, err := MarshalReading(r)
	if err != nil {
		t.Fatal(err)
	}
	if r.Tenancy.FilesWithError != nil || bytes.Contains(b, []byte("files_with_error")) {
		t.Fatalf("a subject without the count wrote one:\n%s", b)
	}
	MergeSubject(&r, sub("b", n(2)))
	MergeSubject(&r, sub("c", n(3)))
	if r.Tenancy.FilesWithError == nil || *r.Tenancy.FilesWithError != 5 {
		t.Fatalf("files_with_error = %v, want 5 (2 + 3; a is not measured)", r.Tenancy.FilesWithError)
	}
	if err := r.Validate(); err != nil {
		t.Fatal(err)
	}
	if b, _ = MarshalReading(r); !bytes.Contains(b, []byte(`"files_with_error": 5`)) {
		t.Errorf("reading tenancy lacks files_with_error:\n%s", b)
	}
	r.Tenancy.FilesWithError = n(31)
	if err := r.Validate(); err == nil || !strings.Contains(err.Error(), "files_with_error") {
		t.Errorf("31 files with an error of 30 measured: Validate = %v, want a refusal", err)
	}
}

// TestCommittedReadingsRoundTrip (LB-F59): every committed reading decodes
// strictly, validates, and re-marshals to the same JSON value (a hand-edited
// note may spell < or > where the encoder escapes it), so a field the
// harness adds (files_with_error) leaves a reading measured before it
// unchanged.
func TestCommittedReadingsRoundTrip(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "docs", "bench", "readings", "lang", "*.json"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no committed readings (%v)", err)
	}
	for _, p := range paths {
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		var r Reading
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&r); err != nil {
			t.Errorf("%s: %v", p, err)
			continue
		}
		if err := r.Validate(); err != nil {
			t.Errorf("%s: %v", p, err)
		}
		b, err := MarshalReading(r)
		if err != nil {
			t.Fatal(err)
		}
		var was, now any
		if err := json.Unmarshal(raw, &was); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(b, &now); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(was, now) {
			t.Errorf("%s: re-marshalled reading differs from the committed one", p)
		}
	}
}

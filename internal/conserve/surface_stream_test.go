package conserve

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSurfaceReadFailureIsUnproven(t *testing.T) {
	readErr := func(string) ([]byte, bool, error) { return nil, false, errors.New("permission denied") }
	readOK := fileReader(map[string]string{"a.go": "package a\nfunc A() {}\n"})
	for _, tc := range []struct {
		name       string
		base, head func(string) ([]byte, bool, error)
	}{
		{"base", readErr, readOK},
		{"head", readOK, readErr},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := Input{ChangedFiles: []string{"a.go"}, BaseFile: tc.base, HeadFile: tc.head}
			rep := Run(context.Background(), in, []Plane{surfacePlane{}}).Planes[0]
			if rep.Verdict != Unproven || len(rep.Missing) == 0 || !strings.Contains(strings.Join(rep.Missing, " "), "permission denied") {
				t.Fatalf("read failure became absence: %+v", rep)
			}
		})
	}
}

func TestSurfaceCancellationStopsNextRead(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	baseReads, headReads := 0, 0
	in := Input{ChangedFiles: []string{"a.go", "b.go"},
		BaseFile: func(string) ([]byte, bool, error) { baseReads++; cancel(); return []byte("package a\n"), true, nil },
		HeadFile: func(string) ([]byte, bool, error) { headReads++; return []byte("package a\n"), true, nil },
	}
	rep := Run(ctx, in, []Plane{surfacePlane{}}).Planes[0]
	if baseReads != 1 || headReads != 0 || rep.Verdict != Unproven || len(rep.Missing) == 0 {
		t.Fatalf("cancel: base=%d head=%d report=%+v", baseReads, headReads, rep)
	}
}

func TestSurfacePreCanceledReadsNothing(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	reads := 0
	read := func(string) ([]byte, bool, error) { reads++; return nil, false, nil }
	in := Input{ChangedFiles: []string{"a.go"}, BaseFile: read, HeadFile: read}
	rep := Run(ctx, in, []Plane{surfacePlane{}}).Planes[0]
	if reads != 0 || rep.Verdict != Unproven || len(rep.Missing) == 0 {
		t.Fatalf("pre-cancel: reads=%d report=%+v", reads, rep)
	}
}

func TestSurfaceExtractsBeforeReaderReusesBuffer(t *testing.T) {
	buf := make([]byte, len("package a\nfunc A() {}\n"))
	in := Input{ChangedFiles: []string{"a/a.go", "a/b.go"},
		BaseFile: func(path string) ([]byte, bool, error) {
			if path == "a/a.go" {
				copy(buf, "package a\nfunc A() {}\n")
			} else {
				copy(buf, "package a\nfunc B() {}\n")
			}
			return buf, true, nil
		},
		HeadFile: func(string) ([]byte, bool, error) { return nil, false, nil },
	}
	rep := Run(context.Background(), in, []Plane{surfacePlane{}}).Planes[0]
	keys := map[string]bool{}
	for _, f := range rep.Findings {
		keys[f.Key] = true
	}
	if !keys["api:a.A"] || !keys["api:a.B"] || rep.Verdict != Losses {
		t.Fatalf("reused buffer lost a file's surface: %+v", rep)
	}
}

func TestSurfaceDirectoryAtHeadIsAbsent(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "nested.go"), 0755); err != nil {
		t.Fatal(err)
	}
	in := Input{ChangedFiles: []string{"nested.go"}, BaseFile: fileReader(nil), HeadFile: func(p string) ([]byte, bool, error) { return readWorktreeFile(context.Background(), root, p) }}
	findings, missing := (surfacePlane{}).Check(context.Background(), in)
	if len(findings) != 0 || len(missing) != 0 {
		t.Fatalf("directory became unreadable source: findings=%v missing=%v", findings, missing)
	}
}

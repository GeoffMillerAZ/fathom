package specdialect

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/verify"
)

func foreignTestsRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	err := fs.WalkDir(os.DirFS("testdata/foreigntests"), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		dest := filepath.Join(root, filepath.FromSlash(path))
		if d.IsDir() {
			return os.MkdirAll(dest, 0755)
		}
		body, err := os.ReadFile(filepath.Join("testdata/foreigntests", filepath.FromSlash(path)))
		if err != nil {
			return err
		}
		return os.WriteFile(dest, body, 0644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func TestForeignTestRefsBecomeCertifiedBy(t *testing.T) {
	for _, tc := range []struct {
		d    SpecDialect
		path string
		want [][]string
	}{
		{Kiro(), ".kiro/specs/example/tasks.md", [][]string{{"TestExisting", "TestZulu"}, nil}},
		{SpecKit(), "specs/001-example/tasks.md", [][]string{{"TestExisting"}, nil}},
		{TaskMaster(), ".taskmaster/tasks/tasks.json", [][]string{{"TestExisting"}, nil, nil}},
	} {
		t.Run(tc.d.ID(), func(t *testing.T) {
			root := foreignTestsRepo(t)
			before := mustRead(t, filepath.Join(root, tc.path))
			p, _, _, err := ParseFile(tc.d, root, tc.path)
			if err != nil {
				t.Fatal(err)
			}
			back := verify.ParseMarkdownPlan("", Synthesize(p))
			if len(back.Tasks) != len(tc.want) {
				t.Fatalf("tasks = %d, want %d", len(back.Tasks), len(tc.want))
			}
			for i, want := range tc.want {
				if got := back.Tasks[i].Expect.CertifiedBy; !reflect.DeepEqual(got, want) {
					t.Errorf("task %s certified_by = %v, want %v", back.Tasks[i].ID, got, want)
				}
				if len(back.Tasks[i].DerivedTests) != 0 {
					t.Errorf("task %s gained derived tests: %v", back.Tasks[i].ID, back.Tasks[i].DerivedTests)
				}
			}
			if after := mustRead(t, filepath.Join(root, tc.path)); string(after) != string(before) {
				t.Fatal("foreign source changed")
			}
		})
	}
}

func TestForeignTestRefsSynthesizeRoundTrip(t *testing.T) {
	root := foreignTestsRepo(t)
	file := filepath.Join(root, "specs/001-example/tasks.md")
	body := strings.ReplaceAll(string(mustRead(t, file)), ", certified_by: TestExisting", "")
	p, _, err := ParseContent(SpecKit(), file, []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	wire := Synthesize(p)
	if !strings.Contains(string(wire), `derived_tests: "TestAlpha,TestRoot,TestZulu"`) || strings.Contains(string(wire), "certified_by:") {
		t.Fatalf("synthesis lost separate derived test names: %s", wire)
	}
	back := verify.ParseMarkdownPlan("", wire)
	if got := string(Synthesize(back)); got != string(wire) {
		t.Fatalf("round trip changed synthesized row: %s", got)
	}
}

func TestForeignTestRefsParseContent(t *testing.T) {
	for _, tc := range []struct {
		d    SpecDialect
		path string
		want [][]string
	}{
		{Kiro(), ".kiro/specs/example/tasks.md", [][]string{{"TestAlpha", "TestZulu"}, nil}},
		{SpecKit(), "specs/001-example/tasks.md", [][]string{{"TestAlpha", "TestRoot", "TestZulu"}, nil}},
		{TaskMaster(), ".taskmaster/tasks/tasks.json", [][]string{{"TestAlpha", "TestZulu"}, nil, nil}},
	} {
		t.Run(tc.d.ID(), func(t *testing.T) {
			root := foreignTestsRepo(t)
			file := filepath.Join(root, tc.path)
			body := strings.ReplaceAll(string(mustRead(t, file)), `, certified_by: "TestExisting,TestZulu"`, "")
			body = strings.ReplaceAll(body, ", certified_by: TestExisting", "")
			p, spans, err := ParseContent(tc.d, file, []byte(body))
			if err != nil {
				t.Fatal(err)
			}
			if len(p.Tasks) != len(tc.want) || len(spans) != len(tc.want) {
				t.Fatalf("tasks = %d, spans = %d, want %d each", len(p.Tasks), len(spans), len(tc.want))
			}
			for i, want := range tc.want {
				if got := p.Tasks[i].DerivedTests; !reflect.DeepEqual(got, want) {
					t.Errorf("task %s derived tests = %v, want %v", p.Tasks[i].ID, got, want)
				}
				if len(p.Tasks[i].Expect.CertifiedBy) != 0 {
					t.Errorf("task %s gained explicit certificate names: %v", p.Tasks[i].ID, p.Tasks[i].Expect.CertifiedBy)
				}
			}
		})
	}
}

func TestForeignTestRefsDoNotTreatPathsAsIdentifiers(t *testing.T) {
	got := mapForeignTestRefs("", "/generated/TestDirectory/check_test.go and TestFilename_test.go", false)
	if len(got) != 0 {
		t.Fatalf("path names produced test identifiers: %v", got)
	}
}

func TestForeignTestRefsSpecKitPathsStayInsideRepo(t *testing.T) {
	root := foreignTestsRepo(t)
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "outside_test.go"), []byte("package example\nimport \"testing\"\nfunc TestOutside(t *testing.T) {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "broken_test.go"), []byte("package example\nfunc TestBroken("), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "notes_test.go.bak"), []byte("package example\nimport \"testing\"\nfunc TestBackup(t *testing.T) {}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"escape/outside_test.go", "../outside_test.go", filepath.Join(outside, "outside_test.go"), "https://example.invalid/outside_test.go", "missing_test.go", "broken_test.go", "notes_test.go.bak"} {
		t.Run(path, func(t *testing.T) {
			body := []byte("- [ ] T001 Check `" + path + "` {symbol: Check}\n")
			p, _, err := ParseContent(SpecKit(), filepath.Join(root, "specs/001-example/tasks.md"), body)
			if err != nil {
				t.Fatal(err)
			}
			if got := verify.ParseMarkdownPlan("", Synthesize(p)).Tasks[0].DerivedTests; len(got) != 0 {
				t.Fatalf("unsafe or unresolved path produced certificates: %v", got)
			}
		})
	}
}

func TestForeignTestRefsRequireExplicitNames(t *testing.T) {
	for _, name := range []string{"TestFlight", "TestCafe", "TestNG", "TestRail"} {
		t.Run(name, func(t *testing.T) {
			if got := mapForeignTestRefs("", "Integrate with "+name+" for checks.", false); len(got) != 0 {
				t.Fatalf("prose produced certificates: %v", got)
			}
		})
	}
	for _, tc := range []struct {
		text string
		want []string
	}{
		{"Run `TestAlpha` and `TestZulu`.", []string{"TestAlpha", "TestZulu"}},
		{"go test -run TestAlpha", []string{"TestAlpha"}},
		{"go test -run=TestZulu", []string{"TestZulu"}},
		{"go test -run 'TestAlpha/case'", []string{"TestAlpha"}},
		{"Run `TestZulu/case`", []string{"TestZulu"}},
		{"Run `Testhelper`, `Test` and `prefixTestWrong`", nil},
		{"Run `Test7`, `Test_case`", []string{"Test7", "Test_case"}},
		{"Run `internal/TestDirectory/check_test.go` and `TestFilename_test.go`", nil},
	} {
		t.Run(tc.text, func(t *testing.T) {
			if got := mapForeignTestRefs("", tc.text, false); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("certificates = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestForeignTestRefsUnanchoredStayVisible(t *testing.T) {
	root := foreignTestsRepo(t)
	p, _, _, err := ParseFile(TaskMaster(), root, ".taskmaster/tasks/tasks.json")
	if err != nil {
		t.Fatal(err)
	}
	rep := Converge(p, verify.Report{}, nil)
	for _, item := range rep.Unanchored {
		if item.TaskID == "1.1" {
			return
		}
	}
	t.Fatalf("unanchored child missing from converge: %+v", rep)
}

func TestForeignTestRefsKiroTaskOnly(t *testing.T) {
	root := foreignTestsRepo(t)
	file := filepath.Join(root, ".kiro/specs/example/tasks.md")
	req := "## Requirement 1\n1. WHEN checking THEN run `TestRequirement`.\n"
	if err := os.WriteFile(filepath.Join(filepath.Dir(file), "requirements.md"), []byte(req), 0644); err != nil {
		t.Fatal(err)
	}
	body := "- [x] 1. Run `TestTask` {symbol: Check}\n  _Requirements: 1.1_\n- [x] 2. Check again {symbol: Check}\n  _Requirements: 1.1_\n"
	p, _, err := ParseContent(Kiro(), file, []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if got := p.Tasks[0].DerivedTests; !reflect.DeepEqual(got, []string{"TestTask"}) {
		t.Fatalf("task certificates = %v", got)
	}
	if got := p.Tasks[1].DerivedTests; len(got) != 0 {
		t.Fatalf("inherited requirement produced certificates: %v", got)
	}
	if !strings.Contains(p.Tasks[1].Accept, "TestRequirement") {
		t.Fatal("requirement acceptance text lost")
	}
}

func TestForeignTestRefsNestedSpecKit(t *testing.T) {
	root := foreignTestsRepo(t)
	if err := os.Mkdir(filepath.Join(root, ".git"), 0755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "svc/specs/001-example/tasks.md")
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		t.Fatal(err)
	}
	body := []byte("- [x] T001 Check `internal/example/check_test.go` {symbol: Check}\n")
	if err := os.WriteFile(file, body, 0644); err != nil {
		t.Fatal(err)
	}
	for _, parse := range []func() (verify.Plan, error){
		func() (verify.Plan, error) {
			p, _, _, err := ParseFile(SpecKit(), root, "svc/specs/001-example/tasks.md")
			return p, err
		},
		func() (verify.Plan, error) { p, _, err := ParseContent(SpecKit(), file, body); return p, err },
	} {
		p, err := parse()
		if err != nil {
			t.Fatal(err)
		}
		if got := p.Tasks[0].DerivedTests; !reflect.DeepEqual(got, []string{"TestAlpha", "TestZulu"}) {
			t.Fatalf("nested spec certificates = %v", got)
		}
	}
}

func TestForeignTestRefsSpecKitReadBound(t *testing.T) {
	root := t.TempDir()
	body := "package example\nimport \"testing\"\n//" + strings.Repeat("x", 1<<20) + "\nfunc TestBeyondLimit(t *testing.T) {}\n"
	if err := os.WriteFile(filepath.Join(root, "large_test.go"), []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	if got := mapForeignTestRefs(root, "large_test.go", true); len(got) != 0 {
		t.Fatalf("read beyond bounded input: %v", got)
	}
}

func TestForeignTestRefsSpecKitFIFO(t *testing.T) {
	mkfifo, err := exec.LookPath("mkfifo")
	if err != nil {
		t.Skip("mkfifo unavailable")
	}
	shell, err := exec.LookPath("sh")
	if err != nil {
		t.Skip("FIFO writer shell unavailable")
	}
	root := t.TempDir()
	fifo := filepath.Join(root, "pipe_test.go")
	if out, err := exec.Command(mkfifo, fifo).CombinedOutput(); err != nil {
		t.Skipf("mkfifo unsupported: %v: %s", err, out)
	}
	// A finite writer lets the old reader return a false certificate instead
	// of hanging the regression run. The fixed reader never opens the FIFO.
	writer := exec.Command(shell, "-c", `printf 'package example\nimport "testing"\nfunc TestFIFO(t *testing.T) {}\n' > "$1"`, "writer", fifo)
	if err := writer.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = writer.Process.Kill(); _ = writer.Wait() }()
	if got := mapForeignTestRefs(root, "pipe_test.go", true); len(got) != 0 {
		t.Fatalf("FIFO produced certificates: %v", got)
	}
}

func TestForeignTestRefsNestedSpecKitExport(t *testing.T) {
	root := foreignTestsRepo(t)
	file := "svc/specs/001-example/tasks.md"
	if err := os.MkdirAll(filepath.Dir(filepath.Join(root, file)), 0755); err != nil {
		t.Fatal(err)
	}
	body := []byte("- [x] T001 Check `internal/example/check_test.go` {symbol: Check}\n")
	if err := os.WriteFile(filepath.Join(root, file), body, 0644); err != nil {
		t.Fatal(err)
	}
	p, _, _, err := ParseFile(SpecKit(), root, file)
	if err != nil {
		t.Fatal(err)
	}
	if got := p.Tasks[0].DerivedTests; !reflect.DeepEqual(got, []string{"TestAlpha", "TestZulu"}) {
		t.Fatalf("exported nested spec certificates = %v", got)
	}
}

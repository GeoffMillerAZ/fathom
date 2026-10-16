package adoption

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/catalog"
)

func adoptionCheckWrite(t *testing.T, root, path string, body []byte) {
	t.Helper()
	name := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(name), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, body, 0600); err != nil {
		t.Fatal(err)
	}
}

func adoptionCheckHash(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func adoptionCheckSave(t *testing.T, root string, selection Selection) {
	t.Helper()
	body, err := json.Marshal(selection)
	if err != nil {
		t.Fatal(err)
	}
	adoptionCheckWrite(t, root, DefaultManifest, body)
}

func adoptionCheckFixture(t *testing.T) (string, Selection) {
	t.Helper()
	root := t.TempDir()
	c, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := c.Get("foundation.tokens"); !exists {
		t.Fatal("fixture requires the real first-party token capability")
	}
	artifact := []byte("/* Synthetic consumer-owned artifact, not a generated-origin claim. */\n")
	s := Selection{SchemaVersion: 1, Consumer: "synthetic-consumer", Profile: "portable", Layers: []string{"L0", "L1"}, Capabilities: []string{"foundation.tokens"},
		CatalogRevision: c.Revision, Contract: "docs/app-kit.md", Artifacts: []Artifact{{Path: "public/theme.css", SHA256: adoptionCheckHash(artifact)}}, Agents: true}
	adoptionCheckWrite(t, root, s.Contract, []byte("# Synthetic adoption\nTask: inspect static declarations; never run project commands.\n"))
	adoptionCheckWrite(t, root, s.Artifacts[0].Path, artifact)
	router, err := Router(s.Contract)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"CLAUDE.md", "AGENTS.md"} {
		adoptionCheckWrite(t, root, name, []byte("# Unrelated consumer instructions\n\n"+router+"\nKeep this consumer-specific tail.\n"))
	}
	adoptionCheckSave(t, root, s)
	return root, s
}

func adoptionCheckJSON(t *testing.T, report Report) []byte {
	t.Helper()
	body, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func adoptionCheckOutcome(t *testing.T, report Report, code int, outcome string) {
	t.Helper()
	if report.ExitCode != code || report.Outcome != outcome {
		t.Fatalf("got outcome=%s exit=%d checks=%+v; want %s/%d", report.Outcome, report.ExitCode, report.Checks, outcome, code)
	}
}

func adoptionCheckFinding(t *testing.T, report Report, id, status string) {
	t.Helper()
	for _, finding := range report.Checks {
		if finding.ID == id && finding.Status == status {
			return
		}
	}
	t.Fatalf("missing finding %s/%s in %+v", id, status, report.Checks)
}

func adoptionCheckSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			result[relative] = "directory"
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		result[relative] = adoptionCheckHash(body)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestCheckDeterministicAndReadOnly(t *testing.T) {
	root, selection := adoptionCheckFixture(t)
	before := adoptionCheckSnapshot(t, root)
	first := Check(context.Background(), root, DefaultManifest)
	adoptionCheckOutcome(t, first, 0, "passed")
	if first.Mode != "check" || first.Selection == nil || first.CatalogRevision != selection.CatalogRevision || first.SelectionSHA256 == "" || first.Files == nil || first.Checks == nil || len(first.Limitations) == 0 {
		t.Fatal("check lacks explicit selection, revision or evidence boundaries")
	}
	for _, id := range []string{"selection", "catalog", "profile", "contract", "artifact:1", "capability:1", "router:CLAUDE.md", "router:AGENTS.md"} {
		adoptionCheckFinding(t, first, id, "pass")
	}
	// A consumer that keeps both files gets the report ADR-0024 produced: the
	// router findings in the same order, with the same detail.
	var routers []string
	for _, finding := range first.Checks {
		if strings.HasPrefix(finding.ID, "router:") {
			routers = append(routers, finding.ID+"|"+finding.Detail)
		}
	}
	const paired = "Canonical router text exists; this does not prove the host loaded it."
	if !reflect.DeepEqual(routers, []string{"router:CLAUDE.md|" + paired, "router:AGENTS.md|" + paired}) {
		t.Fatalf("paired-router report changed: %q", routers)
	}
	second := Check(context.Background(), root, DefaultManifest)
	if !bytes.Equal(adoptionCheckJSON(t, first), adoptionCheckJSON(t, second)) {
		t.Fatal("repeated checks over identical bytes produced different receipts")
	}
	if !reflect.DeepEqual(before, adoptionCheckSnapshot(t, root)) {
		t.Fatal("read-only check changed files or created directories")
	}
}

func TestCheckRequiredArtifactFailures(t *testing.T) {
	t.Run("missing selected artifact and restoration", func(t *testing.T) {
		root, selection := adoptionCheckFixture(t)
		selected := selection.Artifacts[0].Path
		selectedBody, err := os.ReadFile(filepath.Join(root, selected))
		if err != nil {
			t.Fatal(err)
		}
		const unselected = "public/unselected.css"
		adoptionCheckWrite(t, root, unselected, []byte("/* Outside the authored selection. */\n"))
		assertUnselectedIgnored := func(report Report) {
			t.Helper()
			for _, file := range report.Files {
				if file.Path == unselected {
					t.Fatal("check inspected an unselected neighboring artifact")
				}
			}
		}

		control := Check(context.Background(), root, DefaultManifest)
		adoptionCheckOutcome(t, control, 0, "passed")
		adoptionCheckFinding(t, control, "artifact:1", "pass")
		assertUnselectedIgnored(control)

		if err := os.Remove(filepath.Join(root, selected)); err != nil {
			t.Fatal(err)
		}
		before := adoptionCheckSnapshot(t, root)
		missing := Check(context.Background(), root, DefaultManifest)
		adoptionCheckOutcome(t, missing, 1, "failed")
		adoptionCheckFinding(t, missing, "artifact:1", "fail")
		assertUnselectedIgnored(missing)
		for _, finding := range missing.Checks {
			if finding.ID == "artifact:1" {
				if finding.Path != selected || finding.Detail != "Required file is missing." || finding.Remedy != "Restore the selected file or review the adoption contract." {
					t.Fatalf("missing selected artifact diagnostic = %+v", finding)
				}
			}
		}
		if !reflect.DeepEqual(before, adoptionCheckSnapshot(t, root)) {
			t.Fatal("failed check repaired or rewrote the consumer")
		}

		adoptionCheckWrite(t, root, selected, selectedBody)
		restored := Check(context.Background(), root, DefaultManifest)
		adoptionCheckOutcome(t, restored, 0, "passed")
		adoptionCheckFinding(t, restored, "artifact:1", "pass")
		assertUnselectedIgnored(restored)
		if err := os.Remove(filepath.Join(root, unselected)); err != nil {
			t.Fatal(err)
		}
		adoptionCheckOutcome(t, Check(context.Background(), root, DefaultManifest), 0, "passed")
	})

	t.Run("changed", func(t *testing.T) {
		root, selection := adoptionCheckFixture(t)
		adoptionCheckWrite(t, root, selection.Artifacts[0].Path, []byte("Synthetic byte drift\n"))
		before := adoptionCheckSnapshot(t, root)
		report := Check(context.Background(), root, DefaultManifest)
		adoptionCheckOutcome(t, report, 1, "failed")
		adoptionCheckFinding(t, report, "artifact:1", "fail")
		if !reflect.DeepEqual(before, adoptionCheckSnapshot(t, root)) {
			t.Fatal("failed check repaired or rewrote the consumer")
		}
	})
}

func TestCheckMissingSelectionVersusInspect(t *testing.T) {
	root, _ := adoptionCheckFixture(t)
	if err := os.Remove(filepath.Join(root, DefaultManifest)); err != nil {
		t.Fatal(err)
	}
	before := adoptionCheckSnapshot(t, root)
	checked := Check(context.Background(), root, DefaultManifest)
	adoptionCheckOutcome(t, checked, 1, "failed")
	adoptionCheckFinding(t, checked, "selection", "fail")
	observed := Inspect(context.Background(), root, DefaultManifest)
	adoptionCheckOutcome(t, observed, 0, "observed")
	adoptionCheckFinding(t, observed, "selection", "not-applicable")
	if observed.Selection != nil || observed.SelectionSHA256 != "" || !reflect.DeepEqual(before, adoptionCheckSnapshot(t, root)) {
		t.Fatal("inspect inferred selection or created missing files")
	}
}

func TestCheckCatalogAndFitFailures(t *testing.T) {
	for _, tc := range []struct {
		name, finding string
		mutate        func(*Selection)
	}{
		{"stale revision", "catalog", func(s *Selection) { s.CatalogRevision = strings.Repeat("0", 64) }},
		{"unknown profile", "profile", func(s *Selection) { s.Profile = "unshipped-native-profile" }},
		{"unknown layer", "layer:1", func(s *Selection) { s.Layers = []string{"L9"} }},
		{"unknown capability", "capability:1", func(s *Selection) { s.Capabilities = []string{"synthetic.missing"} }},
		{"incompatible profile", "capability:1", func(s *Selection) { s.Profile = "cli" }},
		{"incompatible layers", "capability:1", func(s *Selection) { s.Layers = []string{"L3a"} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, selection := adoptionCheckFixture(t)
			tc.mutate(&selection)
			adoptionCheckSave(t, root, selection)
			report := Check(context.Background(), root, DefaultManifest)
			adoptionCheckOutcome(t, report, 1, "failed")
			adoptionCheckFinding(t, report, tc.finding, "fail")
		})
	}
}

func TestCheckModuleRequirements(t *testing.T) {
	for _, tc := range []struct {
		name, declaration string
		allowLocal        bool
		code              int
	}{
		{"exact", "require github.com/geoffmilleraz/app-kit v0.2.3\n", false, 0},
		{"wrong version", "require github.com/geoffmilleraz/app-kit v0.2.4\n", false, 1},
		{"absent kit", "require example.test/other v1.0.0\n", false, 1},
		{"unallowed local", "require github.com/geoffmilleraz/app-kit v0.2.3\nreplace github.com/geoffmilleraz/app-kit => ../unfollowed-kit\n", false, 1},
		{"allowed local", "require github.com/geoffmilleraz/app-kit v0.2.3\nreplace github.com/geoffmilleraz/app-kit => ../unfollowed-kit\n", true, 0},
		{"remote replacement", "require github.com/geoffmilleraz/app-kit v0.2.3\nreplace github.com/geoffmilleraz/app-kit => example.invalid/fork v0.7.0\n", true, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, selection := adoptionCheckFixture(t)
			selection.Module = &ModuleRequirement{File: "tools/go.mod", Version: "v0.2.3", AllowLocalReplace: tc.allowLocal}
			adoptionCheckWrite(t, root, selection.Module.File, []byte("module example.test/consumer\n"+tc.declaration))
			adoptionCheckSave(t, root, selection)
			report := Check(context.Background(), root, DefaultManifest)
			outcome, status := "passed", "pass"
			if tc.code != 0 {
				outcome, status = "failed", "fail"
			}
			adoptionCheckOutcome(t, report, tc.code, outcome)
			adoptionCheckFinding(t, report, "module", status)
			if report.Module == nil || report.Module.Resolution != "declarations-only" {
				t.Fatal("module check inferred effective resolution")
			}
			if tc.name == "allowed local" && (report.Module.Replacement == nil || report.Module.Replacement.Path != "../unfollowed-kit") {
				t.Fatal("local replacement was followed or lost")
			}
		})
	}
}

func TestCheckPairedRoutersAndOptOut(t *testing.T) {
	for _, mode := range []string{"drift", "duplicate", "missing pair", "CRLF", "unselected"} {
		t.Run(mode, func(t *testing.T) {
			root, selection := adoptionCheckFixture(t)
			block, err := Router(selection.Contract)
			if err != nil {
				t.Fatal(err)
			}
			code, outcome := 1, "failed"
			switch mode {
			case "drift":
				adoptionCheckWrite(t, root, "AGENTS.md", []byte(strings.Replace(block, selection.Contract, "docs/wrong.md", 1)))
			case "duplicate":
				adoptionCheckWrite(t, root, "AGENTS.md", []byte(block+block))
			case "missing pair":
				if err := os.Remove(filepath.Join(root, "AGENTS.md")); err != nil {
					t.Fatal(err)
				}
			case "CRLF":
				adoptionCheckWrite(t, root, "AGENTS.md", []byte(strings.ReplaceAll(block, "\n", "\r\n")))
				code, outcome = 0, "passed"
			case "unselected":
				selection.Agents = false
				adoptionCheckSave(t, root, selection)
				adoptionCheckWrite(t, root, "AGENTS.md", []byte("Unselected malformed router\n"))
				code, outcome = 0, "passed"
			}
			report := Check(context.Background(), root, DefaultManifest)
			adoptionCheckOutcome(t, report, code, outcome)
			if mode == "unselected" {
				adoptionCheckFinding(t, report, "agent-routing", "not-applicable")
				for _, file := range report.Files {
					if file.Path == "AGENTS.md" || file.Path == "CLAUDE.md" {
						t.Fatal("unselected agent routing files were inspected")
					}
				}
			} else if code == 1 {
				adoptionCheckFinding(t, report, "router:AGENTS.md", "fail")
			}
		})
	}
}

// ADR-0088: AGENTS.md is the one instruction file, so it alone carries the
// router. CLAUDE.md is optional; when present it must reach that router.
func TestCheckAgentsMDAloneSatisfiesTheRouter(t *testing.T) {
	root, _ := adoptionCheckFixture(t)
	if err := os.Remove(filepath.Join(root, "CLAUDE.md")); err != nil {
		t.Fatal(err)
	}
	before := adoptionCheckSnapshot(t, root)
	first := Check(context.Background(), root, DefaultManifest)
	adoptionCheckOutcome(t, first, 0, "passed")
	adoptionCheckFinding(t, first, "router:AGENTS.md", "pass")
	adoptionCheckFinding(t, first, "router:CLAUDE.md", "not-applicable")
	for _, file := range first.Files {
		if file.Path == "CLAUDE.md" {
			t.Fatal("an absent CLAUDE.md was recorded as inspected evidence")
		}
	}
	for _, finding := range first.Checks {
		if finding.ID == "router:CLAUDE.md" && !strings.Contains(finding.Detail, "AGENTS.md") {
			t.Fatalf("absent CLAUDE.md is not explained: %q", finding.Detail)
		}
	}
	if !bytes.Equal(adoptionCheckJSON(t, first), adoptionCheckJSON(t, Check(context.Background(), root, DefaultManifest))) {
		t.Fatal("repeated checks over identical bytes produced different receipts")
	}
	if !reflect.DeepEqual(before, adoptionCheckSnapshot(t, root)) {
		t.Fatal("read-only check changed files or created directories")
	}
}

func TestCheckClaudeMDMustReachTheAgentsMDRouter(t *testing.T) {
	const wrongBlock = "<!-- appkit:begin -->\n## App-kit\n\nApp-kit adoption lives at docs/wrong.md.\n<!-- appkit:end -->\n"
	for _, tc := range []struct {
		name   string
		claude string
		pass   bool
	}{
		{"import line", "# Notes\n\n@AGENTS.md\n", true},
		{"dot-slash import in a marked region", "<!-- region:begin -->\n@./AGENTS.md\n<!-- region:end -->\n", true},
		{"import with CRLF and trailing space", "Notes\r\n@AGENTS.md  \r\n", true},
		{"inline import", "Project rules live in @AGENTS.md and nowhere else.\n", true},
		{"import after a closed code fence", "```sh\nls\n```\n@AGENTS.md\n", true},
		{"legacy canonical copy stays valid", "# Notes\n\n{{router}}\nTail.\n", true},
		{"import inside a code fence", "```markdown\n@AGENTS.md\n```\n", false},
		{"import inside a tilde fence", "~~~\n@AGENTS.md\n~~~\n", false},
		{"import inside a code span", "Write `@AGENTS.md` to import it.\n", false},
		{"import of another file", "@docs/AGENTS.md\n@OTHER.md\n", false},
		{"import with trailing text", "@AGENTS.md.bak\n", false},
		{"unrelated instructions only", "# Consumer notes\n\nKeep tests fast.\n", false},
		{"empty", "", false},
		{"drifted legacy copy without an import", wrongBlock, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, selection := adoptionCheckFixture(t)
			router, err := Router(selection.Contract)
			if err != nil {
				t.Fatal(err)
			}
			adoptionCheckWrite(t, root, "CLAUDE.md", []byte(strings.ReplaceAll(tc.claude, "{{router}}", router)))
			report := Check(context.Background(), root, DefaultManifest)
			adoptionCheckFinding(t, report, "router:AGENTS.md", "pass")
			if tc.pass {
				adoptionCheckOutcome(t, report, 0, "passed")
				adoptionCheckFinding(t, report, "router:CLAUDE.md", "pass")
				return
			}
			adoptionCheckOutcome(t, report, 1, "failed")
			adoptionCheckFinding(t, report, "router:CLAUDE.md", "fail")
			for _, finding := range report.Checks {
				if finding.ID == "router:CLAUDE.md" && !strings.Contains(finding.Remedy, "@AGENTS.md") {
					t.Fatalf("the remedy does not name the import: %q", finding.Remedy)
				}
			}
		})
	}
}

func TestCheckRouterLivesInAgentsMDNotOnlyInClaudeMD(t *testing.T) {
	for _, mode := range []string{"missing", "drift", "duplicate"} {
		t.Run(mode, func(t *testing.T) {
			root, selection := adoptionCheckFixture(t)
			// CLAUDE.md imports AGENTS.md, so the router must actually be there.
			adoptionCheckWrite(t, root, "CLAUDE.md", []byte("@AGENTS.md\n"))
			block, err := Router(selection.Contract)
			if err != nil {
				t.Fatal(err)
			}
			switch mode {
			case "missing":
				if err := os.Remove(filepath.Join(root, "AGENTS.md")); err != nil {
					t.Fatal(err)
				}
			case "drift":
				adoptionCheckWrite(t, root, "AGENTS.md", []byte(strings.Replace(block, selection.Contract, "docs/wrong.md", 1)))
			case "duplicate":
				adoptionCheckWrite(t, root, "AGENTS.md", []byte(block+block))
			}
			report := Check(context.Background(), root, DefaultManifest)
			adoptionCheckOutcome(t, report, 1, "failed")
			adoptionCheckFinding(t, report, "router:AGENTS.md", "fail")
			adoptionCheckFinding(t, report, "router:CLAUDE.md", "pass")
		})
	}
}

func TestCheckClaudeMDLinkedToAgentsMD(t *testing.T) {
	for _, tc := range []struct {
		name, target string
		pass         bool
	}{
		{"link to AGENTS.md", "AGENTS.md", true},
		{"dot-slash link to AGENTS.md", "./AGENTS.md", true},
		{"link to another file", "OTHER.md", false},
		{"absolute link", "/etc/hosts", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, _ := adoptionCheckFixture(t)
			adoptionCheckWrite(t, root, "OTHER.md", []byte("@AGENTS.md\n"))
			if err := os.Remove(filepath.Join(root, "CLAUDE.md")); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(tc.target, filepath.Join(root, "CLAUDE.md")); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
			report := Check(context.Background(), root, DefaultManifest)
			if tc.pass {
				adoptionCheckOutcome(t, report, 0, "passed")
				adoptionCheckFinding(t, report, "router:CLAUDE.md", "pass")
				return
			}
			adoptionCheckOutcome(t, report, 1, "failed")
			adoptionCheckFinding(t, report, "router:CLAUDE.md", "fail")
			for _, file := range report.Files {
				if file.Path == "CLAUDE.md" {
					t.Fatal("check followed a link that does not name AGENTS.md")
				}
			}
		})
	}
}

func TestCheckVocabularyOnlyDoesNotRequireGo(t *testing.T) {
	root, selection := adoptionCheckFixture(t)
	selection.Layers, selection.Artifacts, selection.Agents = []string{"L0"}, []Artifact{}, false
	adoptionCheckSave(t, root, selection)
	report := Check(context.Background(), root, DefaultManifest)
	adoptionCheckOutcome(t, report, 0, "passed")
	adoptionCheckFinding(t, report, "module", "not-applicable")
	if report.Module != nil {
		t.Fatal("portable vocabulary adoption fabricated a Go dependency")
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); !os.IsNotExist(err) {
		t.Fatal("check created or required an unselected Go module")
	}
}

func TestCheckUnavailableBoundaries(t *testing.T) {
	root, selection := adoptionCheckFixture(t)
	for _, inspect := range []bool{false, true} {
		run := Check
		if inspect {
			run = Inspect
		}
		adoptionCheckOutcome(t, run(context.Background(), filepath.Join(root, "absent"), DefaultManifest), 2, "unavailable")
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		adoptionCheckOutcome(t, run(ctx, root, DefaultManifest), 2, "unavailable")
	}
	adoptionCheckWrite(t, root, selection.Artifacts[0].Path, bytes.Repeat([]byte("x"), MaxFileBytes+1))
	adoptionCheckOutcome(t, Check(context.Background(), root, DefaultManifest), 2, "unavailable")
	adoptionCheckWrite(t, root, DefaultManifest, bytes.Repeat([]byte("x"), MaxManifestBytes+1))
	adoptionCheckOutcome(t, Check(context.Background(), root, DefaultManifest), 2, "unavailable")
	adoptionCheckOutcome(t, Inspect(context.Background(), root, DefaultManifest), 2, "unavailable")
}

func TestCheckUnsafePaths(t *testing.T) {
	outside := t.TempDir()
	secret := []byte("OUTSIDE-SYNTHETIC-BODY-MUST-NOT-LEAK\n")
	adoptionCheckWrite(t, outside, "secret.txt", secret)
	for _, mode := range []string{"manifest traversal", "artifact traversal", "absolute artifact", "leaf symlink", "parent symlink", "nonregular node"} {
		t.Run(mode, func(t *testing.T) {
			root, selection := adoptionCheckFixture(t)
			manifest := DefaultManifest
			unsafePath := "unsafe.txt"
			switch mode {
			case "manifest traversal":
				manifest = "../outside.json"
			case "artifact traversal":
				unsafePath = "../secret.txt"
			case "absolute artifact":
				unsafePath = filepath.Join(outside, "secret.txt")
			case "leaf symlink":
				if err := os.Symlink(filepath.Join(outside, "secret.txt"), filepath.Join(root, unsafePath)); err != nil {
					t.Fatal(err)
				}
			case "parent symlink":
				unsafePath = "linked/secret.txt"
				if err := os.Symlink(outside, filepath.Join(root, "linked")); err != nil {
					t.Fatal(err)
				}
			case "nonregular node":
				if err := os.Mkdir(filepath.Join(root, unsafePath), 0700); err != nil {
					t.Fatal(err)
				}
			}
			if mode != "manifest traversal" {
				selection.Artifacts = []Artifact{{Path: unsafePath, SHA256: adoptionCheckHash(secret)}}
				adoptionCheckSave(t, root, selection)
			}
			report := Check(context.Background(), root, manifest)
			adoptionCheckOutcome(t, report, 1, "failed")
			if bytes.Contains(adoptionCheckJSON(t, report), bytes.TrimSpace(secret)) {
				t.Fatal("unsafe path exposed outside file content")
			}
			for _, file := range report.Files {
				if file.Path == unsafePath {
					t.Fatal("unsafe or special path was opened and recorded as regular-file evidence")
				}
			}
		})
	}
	if got, err := os.ReadFile(filepath.Join(outside, "secret.txt")); err != nil || !bytes.Equal(got, secret) {
		t.Fatal("inspection changed the outside sentinel")
	}
}

func TestInspectModuleFactsDoNotSelectOrResolve(t *testing.T) {
	root := t.TempDir()
	adoptionCheckWrite(t, root, "go.mod", []byte("module example.test/consumer\nrequire github.com/geoffmilleraz/app-kit v0.2.3\nreplace github.com/geoffmilleraz/app-kit => ../absent-local-kit\n"))
	// Invalid workspace syntax is intentionally just inventory bytes, never an
	// instruction to execute Go, resolve the workspace, or inspect outside roots.
	adoptionCheckWrite(t, root, "go.work", []byte("synthetic workspace contents: do not resolve ../outside\n"))
	before := adoptionCheckSnapshot(t, root)
	report := Inspect(context.Background(), root, DefaultManifest)
	adoptionCheckOutcome(t, report, 0, "observed")
	if report.Mode != "inspect" || report.Selection != nil || report.SelectionSHA256 != "" || report.Module == nil || report.Module.DeclaredVersion != "v0.2.3" || report.Module.Resolution != "declarations-only" || report.Module.Replacement == nil || report.Module.Replacement.Path != "../absent-local-kit" {
		t.Fatal("inspection inferred selection or effective module resolution")
	}
	adoptionCheckFinding(t, report, "inventory:go.work", "pass")
	if !reflect.DeepEqual(before, adoptionCheckSnapshot(t, root)) || !bytes.Equal(adoptionCheckJSON(t, report), adoptionCheckJSON(t, Inspect(context.Background(), root, DefaultManifest))) {
		t.Fatal("inventory wrote files or produced unstable evidence")
	}
	selectedRoot, selection := adoptionCheckFixture(t)
	adoptionCheckWrite(t, selectedRoot, selection.Artifacts[0].Path, []byte("Changed artifact is outside inspect's validation scope\n"))
	observed := Inspect(context.Background(), selectedRoot, DefaultManifest)
	adoptionCheckOutcome(t, observed, 0, "observed")
	for _, file := range observed.Files {
		if file.Path == selection.Artifacts[0].Path || file.Path == selection.Contract {
			t.Fatal("inspect silently expanded into checking selected artifact/contract contents")
		}
	}
}

func TestCheckSocket(t *testing.T) {
	// A Unix socket is a real special node, not a directory or a symlink. It
	// lives only in this synthetic root and accepts no connections.
	root, selection := adoptionCheckFixture(t)
	listener, err := net.Listen("unix", filepath.Join(root, "socket"))
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	selection.Artifacts = []Artifact{{Path: "socket", SHA256: adoptionCheckHash(nil)}}
	adoptionCheckSave(t, root, selection)
	report := Check(context.Background(), root, DefaultManifest)
	adoptionCheckOutcome(t, report, 1, "failed")
	adoptionCheckFinding(t, report, "artifact:1", "fail")
	for _, file := range report.Files {
		if file.Path == "socket" {
			t.Fatal("special node was recorded as regular-file evidence")
		}
	}
}

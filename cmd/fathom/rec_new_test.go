package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/recordids"
	"github.com/geoffmilleraz/fathom/internal/records"
)

func recGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = root
	c.Env = append(os.Environ(), "GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid")
	b, e := c.CombinedOutput()
	if e != nil {
		t.Fatalf("git %v: %s %v", args, b, e)
	}
	return strings.TrimSpace(string(b))
}
func recWrite(t *testing.T, root, path string, b []byte) {
	t.Helper()
	p := filepath.Join(root, path)
	if e := os.MkdirAll(filepath.Dir(p), 0700); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(p, b, 0640); e != nil {
		t.Fatal(e)
	}
}
func recFixture(t *testing.T) (string, records.Lane) {
	t.Helper()
	root := t.TempDir()
	l := records.Lane{Kind: "lane", ID: "demo", Goal: "fixture", Class: "L-P", BaseCommit: strings.Repeat("a", 40), Rows: []string{"REC-1"}, OwnedPaths: []string{"ADR.md", "docs/intent/invariants.md", "docs/intent/futures.md", "docs/intent/incidents.md", ".fathom/observations/new.md", "docs/plans/demo.md", "new.md"}, ReadOnlyPaths: []string{}, ContractFiles: []string{"contract.md"}, ProofCommand: "test", Budget: "fixture", HarnessClass: "test", IDRanges: []records.IDRange{{Family: "adr", Prefix: "ADR-", First: 10000, Last: 10001}, {Family: "invariant", Prefix: "I", First: 10000, Last: 10001}, {Family: "future", Prefix: "F", First: 10000, Last: 10001}, {Family: "incident", Prefix: "INC-", First: 10000, Last: 10001}, {Family: "observation", Prefix: "O", First: 10000, Last: 10001}, {Family: "plan", Prefix: "DEMO-", First: 10000, Last: 10001}}}
	for _, p := range []string{"cue.mod/module.cue", "schema/records/schema.cue", "schema/records/lane.cue"} {
		b, e := os.ReadFile(filepath.Join("../..", p))
		if e != nil {
			t.Fatal(e)
		}
		recWrite(t, root, p, b)
	}
	b, _ := json.Marshal(l)
	recWrite(t, root, "docs/lanes/demo.cue", []byte("package lanes\nimport \"github.com/geoffmilleraz/fathom/schema/records\"\nlane: records.#Lane & "+string(b)+"\n"))
	for i, p := range l.OwnedPaths[:6] {
		body := "# Existing\n"
		if i == 5 {
			body = "- [ ] existing {id: DEMO-1}\n"
		}
		recWrite(t, root, p, []byte(body))
	}
	recGit(t, root, "init", "-q")
	recGit(t, root, "add", ".")
	recGit(t, root, "commit", "-qm", "test: trusted fixture")
	base := recGit(t, root, "rev-parse", "HEAD")
	t.Setenv("FATHOM_LANE_TRUSTED_BASE", base)
	t.Setenv("FATHOM_LANE_ID", "demo")
	return root, l
}
func recInvoke(root, family, path, title string) (int, string, string) {
	var out, err bytes.Buffer
	code := run([]string{"rec", "new", "--root", root, "--lane", "demo", "--family", family, "--path", path, "--title", title}, &out, &err)
	return code, out.String(), err.String()
}
func TestRecNewRejectsUnownedPathWithoutAllocating(t *testing.T) {
	root, l := recFixture(t)
	state, e := recordids.LoadTrusted(context.Background(), root, os.Getenv("FATHOM_LANE_TRUSTED_BASE"))
	if e != nil {
		t.Fatal(e)
	}
	before, e := recordids.Inventory(root, state.Policy)
	if e != nil {
		t.Fatal(e)
	}
	v, e := recordids.ValidateInventory(before, state.Policy, state.Inventory)
	if e != nil {
		t.Fatal(e)
	}
	next, e := records.AllocateRecordID(l, "adr", "ADR.md", v)
	if e != nil {
		t.Fatal(e)
	}
	code, out, diag := recInvoke(root, "adr", "outside.md", "title")
	if code != 1 || out != "" || diag == "" {
		t.Fatalf("%d %q %q", code, out, diag)
	}
	if _, e := os.Stat(filepath.Join(root, "outside.md")); !os.IsNotExist(e) {
		t.Fatalf("created target: %v", e)
	}
	after, e := recordids.Inventory(root, state.Policy)
	if e != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("consumed id: %v", e)
	}
	v, e = recordids.ValidateInventory(after, state.Policy, state.Inventory)
	if e != nil {
		t.Fatal(e)
	}
	again, e := records.AllocateRecordID(l, "adr", "ADR.md", v)
	if e != nil || again != next {
		t.Fatalf("candidate changed: %q %v", again, e)
	}
}
func TestRecNewTrustedRoundTrip(t *testing.T) {
	root, l := recFixture(t)
	payloads := []string{"## ADR-10000: title\n\n**Status.** Proposed\n", "| I10000 | title | stated | |\n", "## F10000. title\n", "## INC-10000: title\n", "### O10000: title\n", "- [ ] title {id: DEMO-10000}\n"}
	for i, want := range payloads {
		family := string(l.IDRanges[i].Family)
		p := l.OwnedPaths[i]
		before, e := os.ReadFile(filepath.Join(root, p))
		if e != nil {
			t.Fatal(e)
		}
		code, out, diag := recInvoke(root, family, p, "title")
		id := strings.Fields(out)
		if code != 0 || len(id) != 2 || out != fmt.Sprintf("%s %s\n", id[0], p) {
			t.Fatalf("%d %q %q", code, out, diag)
		}
		got, _ := os.ReadFile(filepath.Join(root, p))
		expected := strings.TrimRight(string(before), "\n") + "\n\n" + want
		if string(got) != expected {
			t.Fatalf("%s payload %q want %q", family, got, expected)
		}
		info, _ := os.Stat(filepath.Join(root, p))
		if info.Mode().Perm() != 0640 {
			t.Fatal("mode changed")
		}
		state, e := recordids.LoadTrusted(context.Background(), root, os.Getenv("FATHOM_LANE_TRUSTED_BASE"))
		if e != nil {
			t.Fatal(e)
		}
		defs, e := recordids.Inventory(root, state.Policy)
		if e != nil {
			t.Fatal(e)
		}
		n := 0
		for _, d := range defs {
			if d.ID == id[0] && string(d.Family) == family && d.At.Path == p {
				n++
			}
		}
		if n != 1 {
			t.Fatalf("roundtrip found %d definitions: %+v", n, defs)
		}
		if _, e := recordids.ValidateInventory(defs, state.Policy, state.Inventory); e != nil {
			t.Fatal(e)
		}
	}
	// Creation in an exact owned file is permitted; no splitting of aggregates.
	code, out, diag := recInvoke(root, "adr", "new.md", "new title")
	if code != 0 || out != "ADR-10001 new.md\n" {
		t.Fatalf("create: %d %q %q", code, out, diag)
	}
	got, _ := os.ReadFile(filepath.Join(root, "new.md"))
	if string(got) != "## ADR-10001: new title\n\n**Status.** Proposed\n" {
		t.Fatalf("%q", got)
	}
	for _, tc := range []struct {
		name   string
		change func()
	}{
		{"missing-trust", func() { t.Setenv("FATHOM_LANE_TRUSTED_BASE", "") }},
		{"unreadable-trust", func() { t.Setenv("FATHOM_LANE_TRUSTED_BASE", strings.Repeat("f", 40)) }},
		{"missing-lane", func() { t.Setenv("FATHOM_LANE_ID", "") }},
		{"mismatched-lane", func() { t.Setenv("FATHOM_LANE_ID", "other") }},
		{"broaden-manifest", func() {
			p := filepath.Join(root, "docs/lanes/demo.cue")
			b, _ := os.ReadFile(p)
			recWrite(t, root, "docs/lanes/demo.cue", bytes.ReplaceAll(b, []byte(`"new.md"`), []byte(`"outside.md"`)))
			t.Cleanup(func() { _ = os.WriteFile(p, b, 0640) })
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before, _ := os.ReadFile(filepath.Join(root, "ADR.md"))
			tc.change()
			code, out, diag := recInvoke(root, "adr", "ADR.md", "title")
			after, _ := os.ReadFile(filepath.Join(root, "ADR.md"))
			if code != 1 || out != "" || diag == "" || !bytes.Equal(before, after) {
				t.Fatalf("trust bypass: %d %q %q", code, out, diag)
			}
		})
	}
	t.Run("misuse", func(t *testing.T) {
		for _, title := range []string{"", "bad\nline", "bad|table", "bad`fence", "bad{field}", "bad\x00", "bad\\(cue)"} {
			code, _, _ := recInvoke(root, "adr", "ADR.md", title)
			if code != 2 {
				t.Fatalf("%q: %d", title, code)
			}
		}
	})
}
func TestRecNewRejectsSymlinkEscape(t *testing.T) {
	for _, parent := range []bool{false, true} {
		t.Run(fmt.Sprint(parent), func(t *testing.T) {
			root, _ := recFixture(t)
			outside := t.TempDir()
			marker := filepath.Join(outside, "marker")
			if e := os.WriteFile(marker, []byte("untouched"), 0600); e != nil {
				t.Fatal(e)
			}
			target := "ADR.md"
			if parent {
				target = "docs/intent/invariants.md"
				if e := os.RemoveAll(filepath.Join(root, "docs/intent")); e != nil {
					t.Fatal(e)
				}
				if e := os.Symlink(outside, filepath.Join(root, "docs/intent")); e != nil {
					t.Fatal(e)
				}
			} else {
				if e := os.Remove(filepath.Join(root, target)); e != nil {
					t.Fatal(e)
				}
				if e := os.Symlink(marker, filepath.Join(root, target)); e != nil {
					t.Fatal(e)
				}
			}
			family := "adr"
			if parent {
				family = "invariant"
			}
			code, out, _ := recInvoke(root, family, target, "title")
			if code != 1 || out != "" {
				t.Fatalf("symlink accepted: %d %q", code, out)
			}
			b, _ := os.ReadFile(marker)
			if string(b) != "untouched" {
				t.Fatal("outside mutated")
			}
			if _, e := os.Stat(filepath.Join(outside, "invariants.md")); !os.IsNotExist(e) {
				t.Fatal("outside created")
			}
		})
	}
}
func TestRecNewSerializedCreation(t *testing.T) {
	root, _ := recFixture(t)
	var wg sync.WaitGroup
	type result struct {
		code      int
		out, diag string
	}
	results := make(chan result, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c, o, e := recInvoke(root, "adr", "new.md", "title")
			results <- result{c, o, e}
		}()
	}
	wg.Wait()
	close(results)
	seen := map[string]bool{}
	for r := range results {
		if r.code != 0 || seen[r.out] {
			t.Fatalf("%+v", r)
		}
		seen[r.out] = true
	}
	b, _ := os.ReadFile(filepath.Join(root, "new.md"))
	want := "## ADR-10000: title\n\n**Status.** Proposed\n\n## ADR-10001: title\n\n**Status.** Proposed\n"
	if string(b) != want {
		t.Fatalf("partial/lost write: %q", b)
	}
}

func TestRecNewAuthorityAndGate(t *testing.T) {
	for _, name := range []string{"candidate-policy", "unreserved", "other-owned-file", "legacy-move", "symlink-manifest-parent", "schema-at-trusted-base", "untracked-manifest", "help"} {
		t.Run(name, func(t *testing.T) {
			root, _ := recFixture(t)
			target := "ADR.md"
			switch name {
			case "candidate-policy":
				recWrite(t, root, ".fathom/record-ids.json", []byte(`{"families":[],"cross_plan":false}`))
				recWrite(t, root, "ADR.md", []byte("## ADR-10000: first\n## ADR-10000: duplicate\n"))
			case "unreserved":
				recWrite(t, root, "ADR.md", []byte("## ADR-20000: unreserved\n"))
			case "other-owned-file":
				recWrite(t, root, "docs/intent/incidents.md", []byte("## ADR-10000: wrong family target ownership\n"))

				// Use a discovered but unowned default target to exercise exact ownership.
				recWrite(t, root, "docs/intent/futures/other.md", []byte("## F10000. wrong lane path\n"))
			case "legacy-move":
				recWrite(t, root, "docs/plans/demo.md", []byte("# moved\n"))
				recWrite(t, root, "docs/plans/other.md", []byte("- [ ] moved {id: DEMO-1}\n"))
			case "symlink-manifest-parent":
				if err := os.Rename(filepath.Join(root, "docs/lanes"), filepath.Join(root, "manifests")); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join(root, "manifests"), filepath.Join(root, "docs/lanes")); err != nil {
					t.Fatal(err)
				}
			case "schema-at-trusted-base":
				recWrite(t, root, "schema/records/lane.cue", []byte("package records\n#Lane: _\n"))
			case "untracked-manifest":
				b, _ := os.ReadFile(filepath.Join(root, "docs/lanes/demo.cue"))
				recWrite(t, root, "docs/lanes/other.cue", b)
			case "help":
				var out, diag bytes.Buffer
				code := run([]string{"rec", "new", "--help"}, &out, &diag)
				if code != 0 || out.Len() == 0 || diag.Len() != 0 {
					t.Fatalf("help: %d %q %q", code, out.String(), diag.String())
				}
				return
			}
			before, _ := os.ReadFile(filepath.Join(root, target))
			code, out, diag := recInvoke(root, "adr", target, "title")
			after, _ := os.ReadFile(filepath.Join(root, target))
			if name == "schema-at-trusted-base" {
				if code != 0 {
					t.Fatalf("trusted schema not used: %d %q %q", code, out, diag)
				}
				return
			}
			if code != 1 || out != "" || !bytes.Equal(before, after) {
				t.Fatalf("authority bypass: %d %q %q", code, out, diag)
			}
			var gateOut, gateErr bytes.Buffer
			if code := cmdGateRecordIDs([]string{"--root", root}, &gateOut, &gateErr); code != 1 {
				t.Fatalf("gate accepted: %d %q %q", code, gateOut.String(), gateErr.String())
			}
		})
	}
	t.Run("gate-valid-and-trust-required", func(t *testing.T) {
		root, _ := recFixture(t)
		var out, diag bytes.Buffer
		if code := cmdGateRecordIDs([]string{"--root", root}, &out, &diag); code != 0 {
			t.Fatalf("%d %q %q", code, out.String(), diag.String())
		}
		t.Setenv("FATHOM_LANE_TRUSTED_BASE", "")
		if code := cmdGateRecordIDs([]string{"--root", root}, &out, &diag); code != 1 {
			t.Fatal("gate accepted missing trust")
		}
	})
}

func TestRecNewMustRemainDiscoverable(t *testing.T) {
	for _, name := range []string{"disabled-family", "mismatched-pattern", "open-fence"} {
		t.Run(name, func(t *testing.T) {
			root, lane := recFixture(t)
			switch name {
			case "disabled-family":
				lane.IDRanges = lane.IDRanges[:1]
				manifest, _ := json.Marshal(lane)
				recWrite(t, root, "docs/lanes/demo.cue", []byte("package lanes\nimport \"github.com/geoffmilleraz/fathom/schema/records\"\nlane: records.#Lane & "+string(manifest)+"\n"))
				recWrite(t, root, ".fathom/record-ids.json", []byte(`{"families":[],"cross_plan":true}`))
				recGit(t, root, "add", ".")
				recGit(t, root, "commit", "-qm", "test: disabled family")
				t.Setenv("FATHOM_LANE_TRUSTED_BASE", recGit(t, root, "rev-parse", "HEAD"))
			case "mismatched-pattern":
				cfg := recordids.Defaults()
				for i := range cfg.Families {
					if cfg.Families[i].Name == "adr" {
						cfg.Families[i].Pattern = `^## (OTHER-[0-9]+)\b`
					}
				}
				b, _ := json.Marshal(cfg)
				recWrite(t, root, ".fathom/record-ids.json", b)
				recGit(t, root, "add", ".")
				recGit(t, root, "commit", "-qm", "test: changed trusted heading pattern")
				t.Setenv("FATHOM_LANE_TRUSTED_BASE", recGit(t, root, "rev-parse", "HEAD"))
			case "open-fence":
				recWrite(t, root, "ADR.md", []byte("# Existing\n```text\n"))
			}
			before, _ := os.ReadFile(filepath.Join(root, "ADR.md"))
			code, out, diag := recInvoke(root, "adr", "ADR.md", "title")
			after, _ := os.ReadFile(filepath.Join(root, "ADR.md"))
			if code != 1 || out != "" || diag == "" || !bytes.Equal(before, after) {
				t.Fatalf("undiscoverable allocation: %d %q %q, after=%q", code, out, diag, after)
			}
		})
	}
}

func TestRecNewRequiresExactTrustedPlanPrefix(t *testing.T) {
	root, lane := recFixture(t)
	lane.IDRanges[5].Prefix = "DEMO"
	manifest, _ := json.Marshal(lane)
	recWrite(t, root, "docs/lanes/demo.cue", []byte("package lanes\nimport \"github.com/geoffmilleraz/fathom/schema/records\"\nlane: records.#Lane & "+string(manifest)+"\n"))
	recGit(t, root, "add", ".")
	recGit(t, root, "commit", "-qm", "test: plan prefix mismatch")
	t.Setenv("FATHOM_LANE_TRUSTED_BASE", recGit(t, root, "rev-parse", "HEAD"))
	before, _ := os.ReadFile(filepath.Join(root, "docs/plans/demo.md"))
	code, out, diag := recInvoke(root, "plan", "docs/plans/demo.md", "title")
	after, _ := os.ReadFile(filepath.Join(root, "docs/plans/demo.md"))
	if code != 1 || out != "" || diag == "" || !bytes.Equal(before, after) {
		t.Fatalf("partial prefix accepted as author-owned: %d %q %q", code, out, diag)
	}
}

func TestRecNewTrustedRootMustBeRepositoryRoot(t *testing.T) {
	root, _ := recFixture(t)
	nested := filepath.Join(root, "nested")
	if err := os.MkdirAll(nested, 0700); err != nil {
		t.Fatal(err)
	}
	// Reproduce the same apparent inventory below the actual repository root.
	// The top-level trusted grants must not authorize nested/ADR.md.
	paths := strings.Split(recGit(t, root, "ls-files"), "\n")
	for _, path := range paths {
		body, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		recWrite(t, nested, path, body)
	}
	before, _ := os.ReadFile(filepath.Join(nested, "ADR.md"))
	code, out, diag := recInvoke(nested, "adr", "ADR.md", "title")
	after, _ := os.ReadFile(filepath.Join(nested, "ADR.md"))
	if code != 1 || out != "" || diag == "" || !bytes.Equal(before, after) {
		t.Fatalf("parent grants authorized a different root: %d %q %q", code, out, diag)
	}
}

func TestRecGateFix2ParseFirstAndDispatch(t *testing.T) {
	ix := loadHelpIndex(t)
	proof := ix.proveHelp("rec", "new")
	if proof.Kind != helpDelegated {
		t.Fatalf("rec new must parse first: %v %v", proof.Kind, proof.Problems)
	}
	got := ix.subverbLiterals("rec")
	if len(got) != 1 || got[0] != "new" {
		t.Fatalf("rec dispatch literals %v, want only new", got)
	}
}

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	skillsdist "github.com/geoffmilleraz/fathom/dist/agents/skills"
	"github.com/geoffmilleraz/fathom/internal/custodian"
)

// legacyCheckLefthookGates and legacyCheckStanzaPresent are the bespoke
// doctor checks CC-13 retired, kept verbatim as the parity oracle: the
// converged rows must say exactly what these said.
func legacyCheckLefthookGates(root string) doctorCheck {
	const name = "lefthook_gates"
	remedy := "add a `fathom verify` (and optionally `fathom review`) step to lefthook.yml's pre-push — docs/adoption/claude-code.md §7.4"

	p := filepath.Join(root, "lefthook.yml")
	data, err := os.ReadFile(p)
	if err != nil {
		alt := filepath.Join(root, "lefthook.yaml")
		data, err = os.ReadFile(alt)
		if err != nil {
			return doctorCheck{Name: name, Status: doctorFail, Reason: fmt.Sprintf("no lefthook.yml/lefthook.yaml at %s", root), Remedy: remedy}
		}
		p = alt
	}
	content := string(data)
	hasVerify := lefthookInvokes(content, "verify")
	hasReview := lefthookInvokes(content, "review")
	switch {
	case hasVerify && hasReview:
		return doctorCheck{Name: name, Status: doctorPass, Reason: fmt.Sprintf("%s has both a fathom verify and a fathom review step", p)}
	case hasVerify:
		return doctorCheck{Name: name, Status: doctorPass, Reason: fmt.Sprintf("%s has a fathom verify step (no fathom review step present — optional, C3-1)", p)}
	default:
		return doctorCheck{Name: name, Status: doctorFail, Reason: fmt.Sprintf("%s has no \"fathom verify\" step in its pre-push commands", p), Remedy: remedy}
	}
}

func legacyCheckStanzaPresent(root string) doctorCheck {
	const name = "stanza_present"
	remedy := "fathom init --path " + root

	var problems []string
	for _, fname := range []string{"CLAUDE.md", "AGENTS.md"} {
		p := filepath.Join(root, fname)
		data, err := os.ReadFile(p)
		if err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", fname, err))
			continue
		}
		content := string(data)
		beginIdx := strings.Index(content, fathomStanzaBeginPrefix)
		endIdx := strings.Index(content, fathomStanzaEnd)
		switch {
		case beginIdx >= 0 && endIdx > beginIdx:
			// well-formed
		case beginIdx < 0 && endIdx < 0:
			problems = append(problems, fmt.Sprintf("%s: never wired (no fathom markers)", fname))
		default:
			problems = append(problems, fmt.Sprintf("%s: corrupted marker pair (begin=%t end=%t)", fname, beginIdx >= 0, endIdx >= 0))
		}
	}
	if len(problems) > 0 {
		return doctorCheck{Name: name, Status: doctorFail, Reason: strings.Join(problems, "; "), Remedy: remedy}
	}
	return doctorCheck{Name: name, Status: doctorPass, Reason: "fathom stanza well-formed in both CLAUDE.md and AGENTS.md"}
}

// TestDoctorConvergence_RowsUnchanged is CC-13's contract: the
// lefthook_gates and stanza_present rows are now rendered from the
// lefthook / claude-code / codex packs' findings (doctorPackFindings →
// custodian.Audit), and over every fixture the retired bespoke checks
// were ever asked about — absent, verify-only, both steps, no verify,
// .yaml-only, .yml shadowing .yaml, variable spellings; never-wired,
// begin-only, end-only, end-before-begin, v3 markers, one file missing —
// the rows are identical, field for field. Doctor renders what the audit
// knows; users see no change.
func TestDoctorConvergence_RowsUnchanged(t *testing.T) {
	good := fathomStanza("svc", false) + "\n"
	lefthookCases := map[string]map[string]string{
		"absent":             {},
		"both_steps":         {"lefthook.yml": "pre-push:\n  commands:\n    v:\n      run: bin/fathom verify --service s\n    r:\n      run: bin/fathom review --since origin/main\n"},
		"verify_only":        {"lefthook.yml": "pre-push:\n  commands:\n    verify:\n      run: bin/fathom verify --service x\n"},
		"no_verify":          {"lefthook.yml": "pre-commit:\n  commands:\n    fmt:\n      run: gofmt -l .\n"},
		"yaml_only":          {"lefthook.yaml": "pre-push:\n  commands:\n    gate:\n      run: fathom verify\n"},
		"yml_shadows_yaml":   {"lefthook.yml": "pre-push: {}\n", "lefthook.yaml": "pre-push:\n  commands:\n    gate:\n      run: fathom verify\n"},
		"dollar_var_quoted":  {"lefthook.yml": "pre-push:\n  commands:\n    g:\n      run: |\n        FATHOM=/x/bin/fathom\n        \"$FATHOM\" verify --plan p.md\n"},
		"braced_var":         {"lefthook.yml": "pre-push:\n  commands:\n    g:\n      run: ${FATHOM} verify --service s\n"},
		"verify_not_fathom":  {"lefthook.yml": "pre-push:\n  commands:\n    g:\n      run: npm run verify\n"},
		"empty":              {"lefthook.yml": ""},
		"review_without_ver": {"lefthook.yml": "pre-push:\n  commands:\n    r:\n      run: fathom review\n"},
	}
	stanzaCases := map[string]map[string]string{
		"absent":           {},
		"both_good":        {"CLAUDE.md": good, "AGENTS.md": good},
		"agents_missing":   {"CLAUDE.md": good},
		"never_wired":      {"CLAUDE.md": "# hand-written\n", "AGENTS.md": good},
		"begin_only":       {"CLAUDE.md": fathomStanzaBegin + "\nno end\n", "AGENTS.md": good},
		"end_only":         {"CLAUDE.md": good, "AGENTS.md": "text\n" + fathomStanzaEnd + "\n"},
		"end_before_begin": {"CLAUDE.md": fathomStanzaEnd + "\n" + fathomStanzaBegin + "\n", "AGENTS.md": "# none\n"},
		"v3_markers":       {"CLAUDE.md": "<!-- fathom:begin v3 sha256:abc -->\nx\n" + fathomStanzaEnd + "\n", "AGENTS.md": good},
	}
	run := func(kind string, cases map[string]map[string]string, now, legacy func(string) doctorCheck) {
		for name, files := range cases {
			root := t.TempDir()
			for f, body := range files {
				if err := os.WriteFile(filepath.Join(root, f), []byte(body), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			got, want := now(root), legacy(root)
			if !reflect.DeepEqual(got, want) {
				t.Errorf("%s/%s: converged row differs from the retired check\n got  %+v\n want %+v", kind, name, got, want)
			}
		}
	}
	run("lefthook_gates", lefthookCases, checkLefthookGates, legacyCheckLefthookGates)
	run("stanza_present", stanzaCases, checkStanzaPresent, legacyCheckStanzaPresent)
}

// TestLefthookPackPatternMatchesDoctor: the lefthook pack ships, as data,
// the exact pattern lefthookInvokes applies for "verify" (the capability
// probe's `.lefthook.yml` fallback still calls lefthookInvokes) — one
// definition of "runs fathom verify", held equal — scoped to pre-push.
func TestLefthookPackPatternMatchesDoctor(t *testing.T) {
	c, err := custodian.PackConcern("lefthook", "lefthook_pre_push_verify")
	if err != nil {
		t.Fatal(err)
	}
	if c.Check.Pattern != lefthookInvokesPattern("verify") {
		t.Fatalf("pack pattern %q drifted from doctor's %q", c.Check.Pattern, lefthookInvokesPattern("verify"))
	}
	if c.Check.Under != "pre-push" {
		t.Fatalf("the verify entry must read the pre-push hook only (I17), got under %q", c.Check.Under)
	}
}

// TestDoctorLefthookUnreadableIsNamed is CC-13's one deliberate change
// (I38): a lefthook.yml that exists and cannot be read is named as
// unreadable — the retired check read past it and reported "no
// lefthook.yml/lefthook.yaml", an unreadable file presented as absent.
func TestDoctorLefthookUnreadableIsNamed(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads through mode 000")
	}
	root := t.TempDir()
	p := filepath.Join(root, "lefthook.yml")
	if err := os.WriteFile(p, []byte("pre-push:\n  commands:\n    v:\n      run: fathom verify\n"), 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(p, 0o644) })

	got := checkLefthookGates(root)
	if got.Status != doctorFail || !strings.Contains(got.Reason, "lefthook.yml unreadable") || strings.Contains(got.Reason, "no lefthook.yml") {
		t.Fatalf("an unreadable lefthook.yml must be named unreadable, never absent: %+v", got)
	}
	if legacy := legacyCheckLefthookGates(root); !strings.Contains(legacy.Reason, "no lefthook.yml") {
		t.Fatalf("the retired check's I38 failure this replaces: %+v", legacy)
	}
}

// TestDoctorLefthookGatesReadsOnlyPrePushCommands is the row's second
// deliberate difference from the retired check (I17): its reason has
// always said "in its pre-push commands", and now the evidence agrees — a
// commented-out verify, or a verify that runs only in pre-commit, gates no
// push and fails the row, where the retired raw-line check passed it. The
// capability probe reads the same pack entry, so the two never disagree.
func TestDoctorLefthookGatesReadsOnlyPrePushCommands(t *testing.T) {
	for name, body := range map[string]string{
		"commented":       "pre-push:\n  commands:\n    gate:\n      run: task validate\n#   verify:\n#     run: bin/fathom verify --service x\n",
		"pre_commit_only": "pre-commit:\n  commands:\n    v:\n      run: bin/fathom verify --service x\npre-push:\n  commands:\n    gate:\n      run: task validate\n",
	} {
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "lefthook.yml"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		got := checkLefthookGates(root)
		if got.Status != doctorFail || !strings.Contains(got.Reason, `has no "fathom verify" step in its pre-push commands`) {
			t.Errorf("%s: a verify that gates no push must fail the row: %+v", name, got)
		}
		if legacy := legacyCheckLefthookGates(root); legacy.Status != doctorPass {
			t.Errorf("%s: the retired check's false pass this replaces: %+v", name, legacy)
		}
		if lefthookRunsVerify(root) {
			t.Errorf("%s: the capability probe must agree with the row", name)
		}
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "lefthook.yml"), []byte("pre-push:\n  commands:\n    v:\n      run: bin/fathom verify --service x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := checkLefthookGates(root); got.Status != doctorPass || !lefthookRunsVerify(root) {
		t.Fatalf("a pre-push verify passes the row and the probe: %+v", got)
	}
}

// TestCustodianSkillMatchesTheBinary is "instructed means true" for the
// custodian's own skill: the SKILL.md `fathom init --skills` installs (and
// this repo mirrors under .claude/ and .agents/) names every check kind the
// loader accepts, every shipped pack, and the propose verb — and never says
// the verb has not shipped. A session that loads a stale skill is told the
// binary does less than it does.
func TestCustodianSkillMatchesTheBinary(t *testing.T) {
	data, err := skillsdist.FS.ReadFile("fathom-custodian/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	body := string(data)
	for _, k := range custodian.KnownKinds {
		if !strings.Contains(body, "`"+string(k)+"`") {
			t.Errorf("the custodian skill never names check kind %q", k)
		}
	}
	for _, p := range custodian.KnownPacks() {
		if !strings.Contains(body, `"`+p+`"`) {
			t.Errorf("the custodian skill never names pack %q", p)
		}
	}
	if !strings.Contains(body, "fathom config propose") || strings.Contains(body, "not** shipped") {
		t.Error("the custodian skill must teach `fathom config propose`, never call it unshipped")
	}
	for _, mirror := range []string{"../../.claude/skills/fathom-custodian/SKILL.md", "../../.agents/skills/fathom-custodian/SKILL.md"} {
		if got, err := os.ReadFile(mirror); err != nil || string(got) != body {
			t.Errorf("%s differs from the shipped skill (err %v)", mirror, err)
		}
	}
}

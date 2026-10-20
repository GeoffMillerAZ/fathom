package custodian

import (
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/projectdir"
)

// TestPacksAreCuratedData holds every shipped pack to vision 22 §7's
// admission rule: it is data the one loader parses (no Go per pack), its
// block names it and its version, every entry is a complete falsifiable
// want that names in `why` the real failure it catches, and an entry two
// packs share is defined identically in both (so enabling both is never
// ambiguous).
func TestPacksAreCuratedData(t *testing.T) {
	if got := KnownPacks(); !reflect.DeepEqual(got, []string{"claude-code", "codex", "lefthook", "taskfile"}) {
		t.Fatalf("shipped packs = %v", got)
	}
	shared := map[string]Concern{}
	for _, name := range KnownPacks() {
		p, err := LoadPack(name)
		if err != nil {
			t.Fatalf("pack %s does not load: %v", name, err)
		}
		if p.Name != name || p.Version < 1 || p.About == "" || len(p.Concerns) == 0 {
			t.Errorf("pack %s: identity incomplete: %+v", name, p)
		}
		for _, c := range p.Concerns {
			if c.Scope != ScopePack || c.Pack != name {
				t.Errorf("%s/%s: provenance = %s %q", name, c.ID, c.Scope, c.Pack)
			}
			if strings.TrimSpace(c.Why) == "" {
				t.Errorf("%s/%s: a pack entry must name the failure it catches in why", name, c.ID)
			}
			if prev, ok := shared[c.ID]; ok && !sameWant(prev, c) {
				t.Errorf("%s: shared by two packs with different definitions", c.ID)
			}
			shared[c.ID] = c
		}
	}
	if _, err := LoadPack("jenkins"); err == nil || !strings.Contains(err.Error(), "claude-code, codex, lefthook, taskfile") {
		t.Fatalf("an unknown pack must be refused naming the shipped ones, got %v", err)
	}
	// The chain the row names is covered: docs → Taskfile, lefthook →
	// Taskfile/scripts, Taskfile → scripts, each by commands_resolve.
	for id, where := range map[string]string{
		"agent_config_commands_resolve": CorpusAgentConfig,
		"lefthook_commands_resolve":     CorpusLefthook,
		"taskfile_commands_resolve":     CorpusTaskfile,
	} {
		if c, ok := shared[id]; !ok || c.Check.Kind != KindCommandsResolve || c.Check.Where != where {
			t.Errorf("chain link %s missing or mis-shaped: %+v", id, c)
		}
	}
}

// TestPacks_EnabledEntryOverridable is CC-12's law-13 contract: a scope
// enables packs (the union of both scopes' lists), a same-ID entry in
// either file replaces the pack's wholesale, `off: true` disables one and
// the audit still names it, two packs' shared entry collapses to one
// naming both, and a misspelled pack or field is refused by name rather
// than enabling nothing in silence.
func TestPacks_EnabledEntryOverridable(t *testing.T) {
	dir := t.TempDir()
	userPath := filepath.Join(dir, "user.cue")
	projPath := filepath.Join(dir, "project.cue")
	writeTestFile(t, userPath, `packs: ["claude-code"]`+"\n")
	writeTestFile(t, projPath, `packs: ["codex", "lefthook", "codex"]

concerns: {
	lefthook_runs_gate: {
		want:  "pre-push runs this repo's gate, task check"
		check: {kind: "command_documented_and_exists", where: "lefthook", command: "task check"}
	}
	lefthook_commands_resolve: {off: true}
}
`)
	user, err := LoadScopeFile(userPath, ScopeUser)
	if err != nil {
		t.Fatal(err)
	}
	project, err := LoadScopeFile(projPath, ScopeProject)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(project.Packs, []string{"codex", "lefthook"}) {
		t.Fatalf("pack list must dedupe and sort, got %v", project.Packs)
	}
	resolved, err := Resolve(user, project)
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]Concern{}
	var ids []string
	for _, c := range resolved {
		byID[c.ID] = c
		ids = append(ids, c.ID)
	}
	wantIDs := []string{
		"agent_config_commands_resolve", "agent_config_pointers_resolve",
		"claude_code_stanza_present", "codex_agents_md_fits", "codex_stanza_present",
		"lefthook_commands_resolve", "lefthook_pre_push_verify", "lefthook_runs_gate",
	}
	if !reflect.DeepEqual(ids, wantIDs) {
		t.Fatalf("resolved ids = %v\nwant %v", ids, wantIDs)
	}
	if c := byID["agent_config_pointers_resolve"]; c.Scope != ScopePack || c.Pack != "claude-code+codex" {
		t.Errorf("a shared entry collapses to one naming both packs, got %s %q", c.Scope, c.Pack)
	}
	if c := byID["lefthook_runs_gate"]; c.Scope != ScopeProject || c.Check.Command != "task check" || c.Why != "" {
		t.Errorf("a project entry replaces the pack's WHOLESALE (no inherited why), got %+v", c)
	}
	if c := byID["lefthook_pre_push_verify"]; c.Provenance() != "pack lefthook" {
		t.Errorf("provenance = %q", c.Provenance())
	}

	// Audit names the disabled entry instead of hiding it.
	root := t.TempDir()
	var off Finding
	for _, f := range Audit(root, resolved) {
		if f.Concern.ID == "lefthook_commands_resolve" {
			off = f
		}
	}
	if off.Status != StatusOff || off.Detail != "off (this repo)" {
		t.Errorf("an entry disabled in the project file is named off, got %+v", off)
	}

	// Refusals, by name.
	for _, tc := range []struct{ body, want string }{
		{`packs: ["lefthok"]`, `unknown pack "lefthok" (known packs: claude-code, codex, lefthook, taskfile)`},
		{`pack: ["lefthook"]`, `unknown top-level field "pack" (known: concerns, packs)`},
		{`packs?: ["lefthook"]`, `OPTIONAL`},
		{`packs: "lefthook"`, `packs must be a list`},
	} {
		p := filepath.Join(t.TempDir(), "concerns.cue")
		writeTestFile(t, p, tc.body+"\n")
		if _, err := LoadScopeFile(p, ScopeProject); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: want refusal containing %q, got %v", tc.body, tc.want, err)
		}
	}
}

// TestPackConcernLookup: a surface that renders a pack entry asks for it by
// pack and id; a missing entry is an error, never a zero Concern.
func TestPackConcernLookup(t *testing.T) {
	c, err := PackConcern("lefthook", "lefthook_pre_push_verify")
	if err != nil || c.Check.Kind != KindRequiredPattern || c.Check.Where != CorpusLefthook {
		t.Fatalf("got %+v, %v", c, err)
	}
	if _, err := PackConcern("lefthook", "nope"); err == nil {
		t.Fatal("a missing entry must be an error")
	}
}

// The shipped packs are the built-in tech packs (K10): each validates against
// schema/techpack's #Pack and reads to exactly what the concerns grammar
// alone reads from the same bytes, so `fathom config audit` cannot move.
func TestBuiltInPacksValidateAsTechPacksByteIdentical(t *testing.T) {
	for _, name := range KnownPacks() {
		file := "packs/" + name + ".cue"
		data, err := packFS.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if verr := projectdir.ValidatePack(data, file); verr != nil {
			t.Errorf("%s is not a valid tech pack: %v", name, verr)
		}
		got, err := LoadPack(name)
		if err != nil {
			t.Fatalf("LoadPack(%s): %v", name, err)
		}
		doc, errs := parseDocument(data, file, ScopePack, true)
		if len(errs) > 0 {
			t.Fatalf("%s: legacy grammar: %v", name, errs)
		}
		for i := range doc.concerns {
			doc.concerns[i].Pack = name
		}
		want := Pack{Name: name, Version: doc.meta.version, About: doc.meta.about, Concerns: doc.concerns}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: LoadPack drifted from the concerns grammar's reading", name)
		}
	}
	// A pack that is not a tech pack is refused by name, never read leniently.
	for label, src := range map[string]string{
		"layer_rules": "pack: {name: \"codex\", version: 1, about: \"a\"}\nlayer_rules: []\nconcerns: {}\n",
		"orientation": "pack: {name: \"codex\", version: 1, about: \"a\"}\norientation: [{anchor: \"a\", why: \"b\"}]\nconcerns: {}\n",
	} {
		_, err := loadPackData("codex", "packs/codex.cue", []byte(src))
		var le *LoadError
		if !errors.As(err, &le) {
			t.Errorf("%s: want *LoadError, got %v", label, err)
		}
	}
}

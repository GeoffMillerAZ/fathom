package main

// fleet_agentsmd_test.go — AG-16 (docs/plans/agents-md.md, ADR-059): the
// console and the fleet read the stanza where it lives now, AGENTS.md, and
// still count the older CLAUDE-file layout as wired.

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/console"
)

func TestFleetWiringNotesReadAgentsMDStanza(t *testing.T) {
	cases := []struct {
		name    string
		files   map[string]string
		want    string
		present bool
	}{
		{"AGENTS.md region", map[string]string{"AGENTS.md": fathomStanza("svc", false) + "\n"}, "present", true},
		{"legacy CLAUDE.md only", map[string]string{"CLAUDE.md": fathomStanza("svc", false) + "\n"}, "present in CLAUDE.md only", true},
		{"import only", map[string]string{"AGENTS.md": "# rules\n", "CLAUDE.md": "@AGENTS.md\n"}, "AGENTS.md present, no fathom stanza", false},
		{"nothing", map[string]string{}, "no AGENTS.md", false},
	}
	for _, c := range cases {
		root := t.TempDir()
		for f, body := range c.files {
			writeRepoFile(t, root, f, body)
		}
		_, got := fleetWiringNotes(root)
		if !strings.HasPrefix(got, c.want) || stanzaNotePresent(got) != c.present {
			t.Errorf("%s: stanza note %q (present=%v), want prefix %q present=%v", c.name, got, stanzaNotePresent(got), c.want, c.present)
		}
	}

	root := t.TempDir()
	if repoHasGuide(root) {
		t.Fatal("an empty root has a guide")
	}
	writeRepoFile(t, root, ".claude/AGENTS.md", "# rules\n")
	if !repoHasGuide(root) || !isGoverningGuide(".claude/AGENTS.md") || !isGoverningGuide("internal/AGENTS.md") {
		t.Error("the .claude/ and nested AGENTS.md forms are not governing guides")
	}

	l := console.RealizationLadder(console.LadderFacts{Service: "svc", StanzaNote: "present"})
	if len(l.Rungs) == 0 || !strings.HasPrefix(l.Rungs[0].Evidence, "AGENTS.md stanza present") {
		t.Errorf("wired rung evidence = %+v, want it to name AGENTS.md", l.Rungs)
	}
}

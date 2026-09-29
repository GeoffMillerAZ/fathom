package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// EP-22: the stanza gains the intent sentence ONLY where the edit-time
// channel is actually wired. A stanza that promises matched invariants in a
// repo whose guard.cue carries none teaches the agent to expect something
// that never arrives — worse than saying nothing at all (law 3).
func TestStanzaIntentSentenceConditional(t *testing.T) {
	plain := fathomStanzaBody("svc", false)
	withIntent := fathomStanzaBody("svc", true)

	if strings.Contains(plain, "docs/intent/") {
		t.Fatalf("the unwired stanza must not mention the corpus:\n%s", plain)
	}
	if !strings.Contains(withIntent, "matched invariants") {
		t.Fatalf("the wired stanza must name what it adds:\n%s", withIntent)
	}
	// Same physical line count: this is always-loaded context, and the
	// sentence is a rewording, not an addition.
	if a, b := strings.Count(plain, "\n"), strings.Count(withIntent, "\n"); a != b {
		t.Fatalf("line count changed: %d vs %d", a, b)
	}
	for _, s := range []string{plain, withIntent} {
		if !strings.Contains(s, "Treat them as binding, not advisory.") &&
			!strings.Contains(s, "as binding, not advisory.") {
			t.Fatalf("both wordings keep the binding sentence:\n%s", s)
		}
	}
}

// intentChannelWired is the predicate, and it is strict: a router alone is
// not the channel. This repo adopted the corpus and left the edit-time
// channel unwired, which is exactly the case the predicate must return
// false for.
func TestIntentChannelWiredNeedsGuardCoverageNotJustARouter(t *testing.T) {
	if intentChannelWired(t.TempDir()) {
		t.Fatal("a repo with no intent corpus is not wired")
	}

	root := t.TempDir()
	writeCorpusFixture(t, root) // router + a sensitive surface, no guard.cue
	if intentChannelWired(root) {
		t.Fatal("a router WITHOUT guard coverage must not claim the channel")
	}

	// Paste the proposal fathom printed: now the channel exists.
	var out, errBuf strings.Builder
	if rc := cmdIntent([]string{"export", "--root", root, "--guard"}, &out, &errBuf); rc != 0 {
		t.Fatalf("export rc=%d: %s", rc, errBuf.String())
	}
	body := out.String()
	body = body[strings.Index(body, "guards: ["):]
	if err := os.MkdirAll(filepath.Join(root, ".fathom"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".fathom", "guard.cue"), []byte(body+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !intentChannelWired(root) {
		t.Fatal("after a human pastes the proposal, the channel IS wired")
	}
}

// This repository is the unwired case today — 14 sensitive surfaces, 0
// covered — so its own stanza must not carry the sentence. The check exists
// because the tempting bug is to wire the predicate and then ship the
// sentence anyway.
func TestThisRepoStanzaStaysPlainWhileItsChannelIsUnwired(t *testing.T) {
	root := repoRootForDocs(t)
	if intentChannelWired(root) {
		t.Skip("this repo's guard.cue now carries intent provenance — the sentence is expected")
	}
	for _, name := range []string{"AGENTS.md"} {
		b, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), "matched invariants") {
			t.Fatalf("%s claims an unwired channel", name)
		}
	}
}

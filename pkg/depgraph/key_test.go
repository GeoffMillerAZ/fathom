package depgraph

import (
	"io/fs"
	"testing"
	"testing/fstest"
	"time"
)

// The fixture tree behind every golden. Invented names only (acme).
// One ordinary file, one exec script, one symlink, one explicit empty
// directory — each carries exactly one canonicalization rule.
func fixtureFS() fstest.MapFS {
	return fstest.MapFS{
		"src/main.go": &fstest.MapFile{Data: []byte("package main\n"), Mode: 0o644, ModTime: time.Unix(1000, 0)},
		"src/run.sh":  &fstest.MapFile{Data: []byte("#!/bin/sh\necho acme\n"), Mode: 0o755, ModTime: time.Unix(1000, 0)},
		"src/link":    &fstest.MapFile{Mode: fs.ModeSymlink, Data: []byte("main.go")},
		"empty":       &fstest.MapFile{Mode: fs.ModeDir | 0o755},
	}
}

func baseInputs(t *testing.T, fsys fs.FS) KeyInputs {
	t.Helper()
	srcs, err := Materialize(fsys, []string{"."})
	if err != nil {
		t.Fatalf("Materialize: %v", err)
	}
	return KeyInputs{
		Domain:              "cikit/nodekey/v1",
		Epoch:               1,
		Task:                "build",
		Platform:            "linux/amd64",
		Sources:             srcs,
		Deps:                []string{"bbbb", "aaaa"},
		ToolchainLockDigest: "lock-digest-1",
		CueLang:             "v0.16.1",
		ToolchainPackages:   map[string]string{"go": "nix-go-1", "cue": "nix-cue-1"},
		Env:                 map[string]string{"CGO_ENABLED": "0", "GOFLAGS": "-trimpath"},
		Args:                []string{"./...", "-count=1"},
		SecretNames:         []string{"registry-token", "api-key"},
	}
}

// TestInputKeyCanonicalizationGoldens pins the canonicalization as hex
// literals (print-then-pin): any change to the hash framing, the field
// order, a sort rule, or Materialize's file semantics breaks a golden
// loudly instead of silently rekeying (or worse, silently NOT rekeying)
// a fleet. Each golden's comment names the rule it pins.
func TestInputKeyCanonicalizationGoldens(t *testing.T) {
	const (
		// The reference fixture under its original domain at epoch 1.
		goldenBase = "e4c7c6231e869032c7599b844d7eca22bf28205ae84cebd11cd1eb992a97a662"
		// CRLF is a change: same file with \r\n line ending.
		goldenCRLF = "93669cbba9c0c18b7f2ef8d51114aa4269e9f92d89915c926e8e67e8672eebea"
		// The exec bit is semantic: run.sh with 0o644 instead of 0o755.
		goldenNoExec = "1d1cdd2dc13bb6fd60471489eeb1576393b13b839d427badc901ca0e71480189"
		// A symlink hashes its target path, never the bytes behind it:
		// link as a REGULAR file whose content equals the target string.
		goldenLinkAsFile = "cae0a1da5a941ba2b39579e4ecd482ee359913461646e835c58318eab2367acd"
		// The domain separator partitions key families.
		goldenOtherDomain = "b331e2bab703ecd85fda5bbff07cceb9e37a31f44fa177d47ca4e51d0ad374ed"
		// An epoch bump rekeys everything under the same domain.
		goldenEpoch2 = "eb4ed30e11b5f4f784dcfa2d341c12adcb60d3956805c0353f3f79ac90830cc9"
	)

	base := InputKey(baseInputs(t, fixtureFS()))
	if got := string(base); got != goldenBase {
		t.Errorf("base key = %s, want %s", got, goldenBase)
	}

	// Determinism: the same inputs twice are byte-identical.
	if again := InputKey(baseInputs(t, fixtureFS())); again != base {
		t.Errorf("same inputs twice: %s vs %s", base, again)
	}

	t.Run("content digests only, never mtime", func(t *testing.T) {
		fsys := fixtureFS()
		for _, f := range fsys {
			f.ModTime = time.Unix(999999, 0)
		}
		if got := InputKey(baseInputs(t, fsys)); got != base {
			t.Errorf("an mtime change moved the key: %s", got)
		}
	})

	t.Run("empty directories are invisible", func(t *testing.T) {
		fsys := fixtureFS()
		delete(fsys, "empty")
		if got := InputKey(baseInputs(t, fsys)); got != base {
			t.Errorf("an empty directory entered the key: %s", got)
		}
	})

	t.Run("CRLF is a change", func(t *testing.T) {
		fsys := fixtureFS()
		fsys["src/main.go"] = &fstest.MapFile{Data: []byte("package main\r\n"), Mode: 0o644}
		got := InputKey(baseInputs(t, fsys))
		if string(got) != goldenCRLF {
			t.Errorf("CRLF key = %s, want %s", got, goldenCRLF)
		}
		if got == base {
			t.Error("a line-ending change did not move the key")
		}
	})

	t.Run("the exec bit is semantic", func(t *testing.T) {
		fsys := fixtureFS()
		fsys["src/run.sh"] = &fstest.MapFile{Data: []byte("#!/bin/sh\necho acme\n"), Mode: 0o644}
		got := InputKey(baseInputs(t, fsys))
		if string(got) != goldenNoExec {
			t.Errorf("no-exec key = %s, want %s", got, goldenNoExec)
		}
		if got == base {
			t.Error("an exec-bit flip did not move the key")
		}
	})

	t.Run("a symlink hashes its target, not equivalent bytes", func(t *testing.T) {
		fsys := fixtureFS()
		fsys["src/link"] = &fstest.MapFile{Data: []byte("main.go"), Mode: 0o644}
		got := InputKey(baseInputs(t, fsys))
		if string(got) != goldenLinkAsFile {
			t.Errorf("link-as-file key = %s, want %s", got, goldenLinkAsFile)
		}
		if got == base {
			t.Error("a symlink and a regular file with the same bytes share a key")
		}
	})

	t.Run("env and source order are irrelevant, sorted inside", func(t *testing.T) {
		in := baseInputs(t, fixtureFS())
		// Reverse the materialized source order and rebuild env in a
		// different insertion order: the sort inside InputKey is the
		// canonical order, not the caller's.
		for i, j := 0, len(in.Sources)-1; i < j; i, j = i+1, j-1 {
			in.Sources[i], in.Sources[j] = in.Sources[j], in.Sources[i]
		}
		in.Env = map[string]string{"GOFLAGS": "-trimpath", "CGO_ENABLED": "0"}
		in.Deps = []string{"aaaa", "bbbb"}
		in.SecretNames = []string{"api-key", "registry-token"}
		if got := InputKey(in); got != base {
			t.Errorf("caller ordering leaked into the key: %s", got)
		}
	})

	t.Run("the domain separator partitions families", func(t *testing.T) {
		in := baseInputs(t, fixtureFS())
		in.Domain = "acme/otherkey/v1"
		got := InputKey(in)
		if string(got) != goldenOtherDomain {
			t.Errorf("other-domain key = %s, want %s", got, goldenOtherDomain)
		}
		if got == base {
			t.Error("a domain change did not move the key")
		}
	})

	t.Run("an epoch bump rekeys", func(t *testing.T) {
		in := baseInputs(t, fixtureFS())
		in.Epoch = 2
		got := InputKey(in)
		if string(got) != goldenEpoch2 {
			t.Errorf("epoch-2 key = %s, want %s", got, goldenEpoch2)
		}
		if got == base {
			t.Error("an epoch bump did not move the key")
		}
	})
}

// TestGeneratedFileCountsOncePerConsumer pins the double-count rule: a
// generated file is EXCLUDED from its generator's key (the generator's
// inputs are what produced it) and INCLUDED in every consumer's — else
// bytes count twice and regeneration self-invalidates the generator.
func TestGeneratedFileCountsOncePerConsumer(t *testing.T) {
	tree := func(generated string) fstest.MapFS {
		return fstest.MapFS{
			"schema.cue":      &fstest.MapFile{Data: []byte("#Acme: {name: string}\n"), Mode: 0o644},
			"zz_generated.go": &fstest.MapFile{Data: []byte(generated), Mode: 0o644},
			"consumer.go":     &fstest.MapFile{Data: []byte("package acme\n"), Mode: 0o644},
		}
	}
	keys := func(t *testing.T, fsys fs.FS) (generator, consumer Key) {
		t.Helper()
		schema, err := Materialize(fsys, []string{"schema.cue"})
		if err != nil {
			t.Fatalf("Materialize schema: %v", err)
		}
		consumed, err := Materialize(fsys, []string{"consumer.go", "zz_generated.go"})
		if err != nil {
			t.Fatalf("Materialize consumer set: %v", err)
		}
		gen := KeyInputs{Domain: "cikit/nodekey/v1", Epoch: 1, Task: "generate", Platform: "linux/amd64", Sources: schema}
		con := KeyInputs{Domain: "cikit/nodekey/v1", Epoch: 1, Task: "build", Platform: "linux/amd64", Sources: consumed}
		return InputKey(gen), InputKey(con)
	}

	gen1, con1 := keys(t, tree("package acme // generated: v1\n"))

	// Regenerating byte-identical output changes neither key.
	gen2, con2 := keys(t, tree("package acme // generated: v1\n"))
	if gen2 != gen1 || con2 != con1 {
		t.Errorf("identical regeneration moved a key: gen %s→%s, con %s→%s", gen1, gen2, con1, con2)
	}

	// Different generated bytes rekey the consumer and ONLY the
	// consumer — the generator keys on its inputs, not its output.
	gen3, con3 := keys(t, tree("package acme // generated: v2\n"))
	if gen3 != gen1 {
		t.Errorf("the generated file entered its own generator's key: %s vs %s", gen1, gen3)
	}
	if con3 == con1 {
		t.Error("a generated-file change did not rekey its consumer")
	}
}

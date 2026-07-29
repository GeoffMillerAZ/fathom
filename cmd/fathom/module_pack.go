package main

// module_pack.go — `fathom module pack`: the thin serializer (product-
// vision H2's scoped deliverable, docs/vision/13-expert-suite.md §4 step 4
// / §9 row J): assemble a portable module dir {module.sqlite, module.cue,
// eval evidence, README stub} and verify the copy's conformance markers
// post-copy. No network, no registry.
//
// Safe-copy discipline (docs/spec/store-format-v0.md §1's 2a, the WAL
// caveat): a WAL-mode store open in a writer keeps recent commits in a
// -wal sidecar; a bare file copy while a writer holds it open risks
// silently dropping un-checkpointed writes. This verb runs
// `PRAGMA wal_checkpoint(TRUNCATE)` against the source store through a
// short-lived raw connection (never through internal/store's write-schema
// path — MountReadOnly's genuinely-read-only contract stays exactly as it
// is; module pack talks to the source file directly and briefly, before
// ever opening it read-only) before copying, then verifies the COPY's
// conformance markers via store.MountReadOnly — a pack that fails its own
// post-copy check is refused loudly, never shipped silently broken.

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"github.com/geoffmilleraz/fathom/internal/modspec"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// PackReport is `module pack`'s summary output.
type PackReport struct {
	Module             string `json:"module"`
	OutDir             string `json:"out_dir"`
	StoreBytes         int64  `json:"store_bytes"`
	ConformanceLevel   string `json:"conformance_level"`
	EvalEvidenceCopied bool   `json:"eval_evidence_copied"`
}

func cmdModulePack(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("module pack", stderr)
	manifestPath := fs.String("manifest", "", "path to module.cue (required)")
	storePath := fs.String("store", "", "the module's own compiled SQLite store (required)")
	outDir := fs.String("out", "", "output directory (required)")
	force := fs.Bool("force", false, "overwrite an existing --out directory's contents")
	jsonOut := fs.Bool("json", false, "emit JSON instead of human text")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *manifestPath == "" || *storePath == "" || *outDir == "" {
		fmt.Fprintln(stderr, "error: --manifest, --store, and --out are required")
		fs.Usage()
		return 1
	}

	m, err := modspec.LoadManifest(*manifestPath)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	if _, statErr := os.Stat(*outDir); statErr == nil {
		entries, _ := os.ReadDir(*outDir)
		if len(entries) > 0 && !*force {
			fmt.Fprintf(stderr, "error: %s already exists and is non-empty — pass --force to overwrite\n", *outDir)
			return 1
		}
	}
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	if err := checkpointWAL(*storePath); err != nil {
		fmt.Fprintf(stderr, "error: checkpoint %s before copy: %v\n", *storePath, err)
		return 1
	}

	sqliteOut := filepath.Join(*outDir, "module.sqlite")
	n, err := copyFile(*storePath, sqliteOut)
	if err != nil {
		fmt.Fprintf(stderr, "error: copy store: %v\n", err)
		return 1
	}

	// Verify the COPY's conformance markers post-copy — never assume the
	// copy is faithful.
	mounted, err := store.MountReadOnly(sqliteOut, store.LevelL0)
	level := "L0"
	if err != nil {
		fmt.Fprintf(stderr, "error: the packed copy %s failed its own conformance check — refusing to ship it: %v\n", sqliteOut, err)
		return 1
	}
	if _, werr := store.MountReadOnly(sqliteOut, store.LevelWriter); werr == nil {
		level = "Writer"
	}
	mounted.Close()

	cueOut := filepath.Join(*outDir, "module.cue")
	if _, err := copyFile(*manifestPath, cueOut); err != nil {
		fmt.Fprintf(stderr, "error: copy manifest: %v\n", err)
		return 1
	}

	evalCopied := false
	if m.EvalEvidence.Evaled && m.EvalEvidence.Path != "" {
		src := m.EvalEvidence.Path
		if !filepath.IsAbs(src) {
			src = filepath.Join(filepath.Dir(*manifestPath), src)
		}
		if _, statErr := os.Stat(src); statErr == nil {
			evalDir := filepath.Join(*outDir, "eval")
			if err := os.MkdirAll(evalDir, 0o755); err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 1
			}
			if _, err := copyFile(src, filepath.Join(evalDir, filepath.Base(src))); err != nil {
				fmt.Fprintf(stderr, "error: copy eval evidence: %v\n", err)
				return 1
			}
			evalCopied = true
		}
	}

	readme := packReadme(m, level, evalCopied)
	if err := os.WriteFile(filepath.Join(*outDir, "README.md"), []byte(readme), 0o644); err != nil {
		fmt.Fprintf(stderr, "error: write README.md: %v\n", err)
		return 1
	}

	report := PackReport{
		Module:             m.Name,
		OutDir:             *outDir,
		StoreBytes:         n,
		ConformanceLevel:   level,
		EvalEvidenceCopied: evalCopied,
	}
	if *jsonOut {
		jsonPrint(stdout, report)
		return 0
	}
	fmt.Fprintf(stdout, "packed %s -> %s\n", m.Name, *outDir)
	fmt.Fprintf(stdout, "  module.sqlite      %d bytes, conformance %s\n", n, level)
	fmt.Fprintf(stdout, "  module.cue         copied\n")
	fmt.Fprintf(stdout, "  eval evidence      copied=%v\n", evalCopied)
	fmt.Fprintf(stdout, "  README.md          written\n")
	return 0
}

// checkpointWAL runs PRAGMA wal_checkpoint(TRUNCATE) against path through a
// short-lived raw connection — a bare file copy of a WAL-mode store open in
// a writer risks silently dropping un-checkpointed commits
// (docs/spec/store-format-v0.md §1's 2a). A store with no pending WAL
// content (already closed, or never opened as a writer) checkpoints as a
// no-op.
func checkpointWAL(path string) error {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer db.Close()
	_, err = db.Exec("PRAGMA wal_checkpoint(TRUNCATE);")
	return err
}

// copyFile copies src to dst (truncating any existing dst), returning the
// byte count copied.
func copyFile(src, dst string) (int64, error) {
	in, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer out.Close()
	n, err := io.Copy(out, in)
	if err != nil {
		return n, err
	}
	return n, out.Close()
}

func packReadme(m modspec.Manifest, conformanceLevel string, evalCopied bool) string {
	evalLine := "not evaled"
	if m.EvalEvidence.Evaled {
		if evalCopied {
			evalLine = "evaled — evidence copied into eval/"
		} else {
			evalLine = fmt.Sprintf("evaled — evidence path %q declared but not found at pack time", m.EvalEvidence.Path)
		}
	}
	return fmt.Sprintf(`# %s

%s

- version:           %s
- watermark:          %s
- tier_profile:       %s
- license:            %s
- eval:               %s
- store conformance:  %s
- packed_at:          %s

This directory is a portable expert module (docs/spec/expert-module-v0.md):
one conforming store (module.sqlite) plus this manifest (module.cue). Move
it anywhere; if anchors need re-pointing at a new corpus location, use
`+"`fathom remap`"+`.
`, m.Name, m.Description, m.Version, m.Watermark, m.TierProfile, m.License, evalLine, conformanceLevel, time.Now().UTC().Format(time.RFC3339))
}

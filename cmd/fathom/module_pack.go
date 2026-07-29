package main

// module_pack.go — `fathom module pack`: the thin serializer (product-
// vision H2's scoped deliverable, docs/vision/13-expert-suite.md §4 step 4
// / §9 row J): assemble a portable module dir {module.sqlite, module.cue,
// eval evidence, README stub} and verify the copy's conformance markers
// post-copy. No network, no registry.
//
// Safe-copy discipline (docs/spec/store-format-v0.md §1's 2a, the WAL
// caveat), Phase J critic finding 6: `PRAGMA wal_checkpoint(TRUNCATE)`
// returns (busy, log, checkpointed) and a caller that discards it can lose
// WAL frames SILENTLY when busy=1 — the original implementation did
// exactly that. `VACUUM INTO '<dest>'` replaces it: a single, atomic SQLite
// statement that reads a fully consistent snapshot of the source database
// (live WAL content included, regardless of checkpoint state) and writes it
// directly to a brand-new file with no -wal/-shm sidecars at all — there is
// no silent-loss window to have, and no separate checkpoint+copy race to
// get wrong. It fails loudly (propagated, never swallowed) when the source
// is locked in a way VACUUM INTO cannot read past.

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"

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
	Lane               string `json:"lane"`
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

	sqliteOut := filepath.Join(*outDir, "module.sqlite")
	if *force {
		// VACUUM INTO refuses to write over an existing file — a --force
		// re-pack must clear the previous artifact first.
		_ = os.Remove(sqliteOut)
	}
	n, err := vacuumInto(*storePath, sqliteOut)
	if err != nil {
		fmt.Fprintf(stderr, "error: safe-copy %s via VACUUM INTO: %v\n", *storePath, err)
		return 1
	}

	// Verify the COPY's conformance markers post-copy — never assume the
	// copy is faithful. Both probes' handles are closed explicitly
	// (finding 6: the LevelWriter probe used to leak, which — combined with
	// VACUUM INTO's own guarantee of never emitting -wal/-shm — matters even
	// more now that a clean artifact is the whole point).
	mounted, err := store.MountReadOnly(sqliteOut, store.LevelL0)
	if err != nil {
		fmt.Fprintf(stderr, "error: the packed copy %s failed its own conformance check — refusing to ship it: %v\n", sqliteOut, err)
		return 1
	}
	mounted.Close()
	level := "L0"
	if writerMounted, werr := store.MountReadOnly(sqliteOut, store.LevelWriter); werr == nil {
		level = "Writer"
		writerMounted.Close()
	}

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

	// Name the compile lane honestly in the README (finding 2) — read from
	// the SOURCE store's own claims (the packed copy is byte-identical),
	// never assumed from the manifest.
	lane := packLane(*storePath, m.Name)

	readme := packReadme(m, level, evalCopied, lane)
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
		Lane:               lane,
	}
	if *jsonOut {
		jsonPrint(stdout, report)
		return 0
	}
	fmt.Fprintf(stdout, "packed %s -> %s\n", m.Name, *outDir)
	fmt.Fprintf(stdout, "  module.sqlite      %d bytes, conformance %s\n", n, level)
	fmt.Fprintf(stdout, "  module.cue         copied\n")
	fmt.Fprintf(stdout, "  eval evidence      copied=%v\n", evalCopied)
	fmt.Fprintf(stdout, "  lane               %s\n", lane)
	fmt.Fprintf(stdout, "  README.md          written\n")
	return 0
}

// packLane reads storePath's own claims for layer (service ID == layer
// name, this phase's own convention) and classifies them via
// classifyCompileLane — best-effort: a read failure reports "unknown"
// rather than aborting the pack (the README should never claim MORE
// confidence than it has, but a lane-classification failure alone is not a
// reason to refuse packing what's otherwise a conforming store).
func packLane(storePath, layer string) string {
	mounted, err := store.MountReadOnly(storePath, store.LevelL0)
	if err != nil {
		return "unknown (could not mount source store to classify: " + err.Error() + ")"
	}
	defer mounted.Close()
	claims, err := mounted.ClaimsForService(context.Background(), layer, layer)
	if err != nil {
		return "unknown (could not read claims to classify: " + err.Error() + ")"
	}
	lane := classifyCompileLane(claims)
	return fmt.Sprintf("%s (%s)", lane.Lane, lane.Detail)
}

// vacuumInto runs `VACUUM INTO ?` against src, writing an atomic, fully
// consistent, sidecar-free snapshot to dest. dest must not already exist —
// VACUUM INTO refuses to overwrite (the caller is responsible for clearing
// a previous artifact under --force). Returns the resulting file's size.
func vacuumInto(src, dest string) (int64, error) {
	db, err := sql.Open("sqlite", src)
	if err != nil {
		return 0, err
	}
	defer db.Close()
	if _, err := db.ExecContext(context.Background(), "VACUUM INTO ?", dest); err != nil {
		return 0, fmt.Errorf("VACUUM INTO %s: %w (a busy/locked source store refuses this — close any writer holding it open and retry)", dest, err)
	}
	fi, err := os.Stat(dest)
	if err != nil {
		return 0, err
	}
	return fi.Size(), nil
}

// copyFile copies src to dst (truncating any existing dst), returning the
// byte count copied. Used for module.cue and eval-evidence files — plain
// text, never a live SQLite store (that path is vacuumInto).
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

func packReadme(m modspec.Manifest, conformanceLevel string, evalCopied bool, lane string) string {
	evalLine := "not evaled"
	if m.EvalEvidence.Evaled {
		if evalCopied {
			evalLine = "evaled — evidence copied into eval/"
		} else {
			evalLine = fmt.Sprintf("evaled — evidence path %q declared but not found at pack time", m.EvalEvidence.Path)
		}
	}
	// No wall-clock "packed_at" (finding 11): the manifest's own
	// provenance.compiled_at is already the deterministic timestamp that
	// matters, and dropping a fresh time.Now() here makes this README
	// byte-reproducible across repeated packs of an unchanged store.
	return fmt.Sprintf(`# %s

%s

- version:           %s
- watermark:         %s
- tier_profile:      %s
- license:            %s
- eval:               %s
- store conformance:  %s
- compile lane:       %s
- compiled_at:        %s

This directory is a portable expert module (docs/spec/expert-module-v0.md):
one conforming store (module.sqlite) plus this manifest (module.cue). Move
it anywhere; if anchors need re-pointing at a new corpus location, use
`+"`fathom remap`"+`.
`, m.Name, m.Description, m.Version, m.Watermark, m.TierProfile, m.License, evalLine, conformanceLevel, lane, m.Provenance.CompiledAt)
}

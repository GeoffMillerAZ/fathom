// module.go — the `fathom module` verb family (Phase J, docs/vision/13-
// expert-suite.md §4/§9 row J): init, sources, map, compile, eval, doctor,
// pack. Every verb is a plain, scriptable CLI verb with --json for machine
// output (law 10, "verb-first surfaces" — no TUI/web/advisor in this
// phase); doctor never auto-promotes a module's readiness level (law 11,
// "the advisor recommends; the gate decides" — there is no advisor here at
// all, only deterministic criteria).
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/filter"
	"github.com/geoffmilleraz/fathom/internal/modspec"
)

// nowFunc is the injected wall-clock `module` verbs use for anything
// requiring a timestamp (module init's provenance.compiled_at) — repo
// convention, never call time.Now() inline in logic (internal/modspec/
// template.go's own doc comment on TemplateInput.CompiledAt: "RFC3339,
// caller-injected clock"). Tests may override this var.
var nowFunc = time.Now

// cmdModule dispatches to the module verb named by args[0].
func cmdModule(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "error: module requires a verb: init | sources | map | compile | eval | doctor | pack | runs")
		printModuleUsage(stderr)
		return 1
	}
	switch args[0] {
	case "init":
		return cmdModuleInit(args[1:], stdout, stderr)
	case "sources":
		return cmdModuleSources(args[1:], stdout, stderr)
	case "map":
		return cmdModuleMap(args[1:], stdout, stderr)
	case "compile":
		return cmdModuleCompile(args[1:], stdout, stderr)
	case "eval":
		return cmdModuleEval(args[1:], stdout, stderr)
	case "doctor":
		return cmdModuleDoctor(args[1:], stdout, stderr)
	case "pack":
		return cmdModulePack(args[1:], stdout, stderr)
	case "runs":
		return cmdModuleRuns(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "error: unknown module verb %q (want init | sources | map | compile | eval | doctor | pack | runs)\n", args[0])
		printModuleUsage(stderr)
		return 1
	}
}

func printModuleUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  fathom module init    --name NAME --description TEXT --include GLOB[,GLOB...] [--exclude GLOB[,GLOB...]] [--dir DIR] [--root DIR] [--tier-profile NAME] [--license NAME] [--force] [--json]")
	fmt.Fprintln(w, "  fathom module sources --manifest module.cue [--root DIR] [--json]")
	fmt.Fprintln(w, "  fathom module map     --manifest module.cue [--root DIR] [--store PATH --service NAME] [--json]")
	fmt.Fprintln(w, "  fathom module compile --manifest module.cue --store PATH [--service NAME] [--dry-run=true|false] [--profile NAME | --mock --mock-entries-file PATH] [--force] [--unsafe-skip-eval-gate] [--json]")
	fmt.Fprintln(w, "  fathom module eval    --manifest module.cue [--fixtures DIR] [--profile NAME | --mock --mock-entries-file PATH] [--record PATH] [--json]")
	fmt.Fprintln(w, "  fathom module doctor  --manifest module.cue [--root DIR] [--store PATH --service NAME] [--pack-dir DIR] [--require-level LEVEL] [--json]")
	fmt.Fprintln(w, "  fathom module pack    --manifest module.cue --store PATH --out DIR [--force] [--json]")
	fmt.Fprintln(w, "  fathom module runs    --manifest module.cue --store PATH [--service NAME] [--limit N] [--json]")
}

// moduleServiceRoot resolves the corpus root a module's manifest.cue is
// discovered against: --root when given, else the manifest file's own
// directory — the convention `fathom module init` itself uses when writing
// module.cue next to the corpus it describes.
func moduleServiceRoot(manifestPath, rootFlag string) string {
	if rootFlag != "" {
		return rootFlag
	}
	return filepath.Dir(manifestPath)
}

// jsonPrint encodes v as indented JSON to w.
func jsonPrint(w io.Writer, v any) {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}

// ─── init ───────────────────────────────────────────────────────────────

func cmdModuleInit(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("module init", stderr)
	dir := fs.String("dir", ".", "target directory for module.cue")
	root := fs.String("root", "", "corpus root to compute the initial watermark from (default: --dir)")
	name := fs.String("name", "", "module name, ^[a-z][a-z0-9-]*$ (required)")
	description := fs.String("description", "", "human-readable scope statement (required)")
	include := fs.String("include", "", "comma-separated corpus include globs (required, non-empty)")
	exclude := fs.String("exclude", "", "comma-separated corpus exclude globs")
	tierProfile := fs.String("tier-profile", "", "tier vocabulary name (default: engineering)")
	license := fs.String("license", "", "license name (default: unspecified)")
	force := fs.Bool("force", false, "overwrite an existing module.cue")
	jsonOut := fs.Bool("json", false, "emit JSON instead of human text")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *name == "" || *description == "" || *include == "" {
		fmt.Fprintln(stderr, "error: --name, --description, and --include are required")
		fs.Usage()
		return 1
	}

	target := filepath.Join(*dir, "module.cue")
	if _, err := os.Stat(target); err == nil && !*force {
		fmt.Fprintf(stderr, "error: %s already exists — pass --force to overwrite\n", target)
		return 1
	}

	corpusRoot := *root
	if corpusRoot == "" {
		corpusRoot = *dir
	}
	includeGlobs := splitCSV(*include)
	excludeGlobs := splitCSV(*exclude)

	// Finding 7 (Phase J critic round): a selector broad enough to match
	// module.cue itself (or the conventional eval/ fixtures location) makes
	// the watermark a MOVING TARGET — module.cue's own bytes depend on the
	// watermark value it embeds, so no single computation can ever be
	// simultaneously "before" and "after" its own effect. Warned, never
	// silently accepted as if it were fine.
	warnIfSelectorMatchesModuleArtifacts(stderr, *dir, corpusRoot, includeGlobs, excludeGlobs)

	// modspec.Manifest carries only what moduleWatermark needs (Name,
	// Corpus) — module.cue itself does not exist on disk yet at this point
	// (that's what this function is about to write), so LoadManifest isn't
	// available; a throwaway value with just the two fields is sufficient.
	throwaway := modspec.Manifest{Name: *name, Corpus: modspec.Corpus{Include: includeGlobs, Exclude: excludeGlobs}}
	watermark, err := moduleWatermark(context.Background(), corpusRoot, throwaway)
	if err != nil {
		fmt.Fprintf(stderr, "error: compute initial watermark for %q: %v\n", corpusRoot, err)
		return 1
	}
	if watermark == "" {
		fmt.Fprintf(stderr, "error: could not establish a watermark for %q (not a git work tree and the manifest content hash came back empty) — the freshness doctrine (docs/spec/expert-module-v0.md §3.3) refuses to scaffold a module with no watermark\n", corpusRoot)
		return 1
	}
	compiledAt := nowFunc().UTC().Format(time.RFC3339)

	// renderAndValidate builds module.cue's text for the given watermark and
	// proves it passes the strict authoring gate BEFORE anything touches
	// target — a scaffold that would fail its own gate is never left on
	// disk (a caller retrying `module init` sees a clean absence, not a
	// half-written file to delete first).
	renderAndValidate := func(wm string) (string, error) {
		src := modspec.Template(modspec.TemplateInput{
			Name:        *name,
			Description: *description,
			Include:     includeGlobs,
			Exclude:     excludeGlobs,
			Watermark:   wm,
			TierProfile: *tierProfile,
			License:     *license,
			CompiledAt:  compiledAt,
		})
		tmp, err := os.CreateTemp("", "module-init-*.cue")
		if err != nil {
			return "", err
		}
		tmpPath := tmp.Name()
		defer os.Remove(tmpPath)
		if _, err := tmp.WriteString(src); err != nil {
			tmp.Close()
			return "", err
		}
		tmp.Close()
		if err := modspec.ValidateStrict(tmpPath); err != nil {
			return "", fmt.Errorf("scaffolded module.cue failed its own authoring gate (this is a bug in `fathom module init`, not your input): %w", err)
		}
		return src, nil
	}

	src, err := renderAndValidate(watermark)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	if err := os.MkdirAll(*dir, 0o755); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	if err := os.WriteFile(target, []byte(src), 0o644); err != nil {
		fmt.Fprintf(stderr, "error: write %s: %v\n", target, err)
		return 1
	}

	// Finding 7: the watermark above was computed BEFORE module.cue existed
	// on disk. Recompute it now that the file is really there — for the
	// common case (selectors that don't match module.cue itself) this is a
	// no-op; when a selector DOES catch it (warned above), this is the
	// closest a single corrective pass can get to the moving target, and
	// module.cue ends up describing the state it was ACTUALLY written into,
	// not a pre-image of it.
	corrected, cErr := moduleWatermark(context.Background(), corpusRoot, throwaway)
	if cErr == nil && corrected != "" && corrected != watermark {
		watermark = corrected
		src, err = renderAndValidate(watermark)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		if err := os.WriteFile(target, []byte(src), 0o644); err != nil {
			fmt.Fprintf(stderr, "error: rewrite %s with corrected watermark: %v\n", target, err)
			return 1
		}
	}

	if *jsonOut {
		jsonPrint(stdout, map[string]any{
			"path":      target,
			"name":      *name,
			"watermark": watermark,
		})
		return 0
	}
	fmt.Fprintf(stdout, "wrote %s (name=%q watermark=%s)\n", target, *name, watermark)
	return 0
}

// warnIfSelectorMatchesModuleArtifacts warns (stderr) when a declared
// corpus selector would match module.cue itself, or the conventional eval/
// fixtures location — both commonly live inside the same root the corpus
// is discovered from. A warning, never a refusal: a caller may have
// deliberately accepted the tradeoff, or already excluded these paths.
func warnIfSelectorMatchesModuleArtifacts(stderr io.Writer, dir, root string, include, exclude []string) {
	rel, err := filepath.Rel(root, filepath.Join(dir, "module.cue"))
	if err != nil {
		return
	}
	rel = filepath.ToSlash(rel)
	if strings.HasPrefix(rel, "../") {
		return // module.cue lives outside root entirely — no risk
	}
	inc := filter.New(include)
	var excl *filter.Matcher
	if len(exclude) > 0 {
		excl = filter.New(exclude)
	}
	matches := func(p string) bool {
		return inc.Match(p) && !(excl != nil && excl.Match(p))
	}
	if matches(rel) {
		fmt.Fprintf(stderr, "warning: a corpus selector matches %s itself — the module's watermark becomes a moving target (module.cue's own content depends on the watermark value it embeds); consider excluding %q\n", rel, rel)
	}
	if matches("eval") || matches("eval/labels.json") {
		fmt.Fprintf(stderr, "warning: a corpus selector matches the conventional \"eval/\" fixtures path — if `module eval --record` writes its artifact under the same tree, the watermark will move every time eval evidence is (re)recorded\n")
	}
}

// moduleWatermark computes THIS MODULE's corpus freshness token: the bare
// git HEAD SHA when root is inside a git work tree (ADR-017's convention,
// unchanged — a git-backed module's freshness tracks the repo, matching
// every other git-backed service in fathom), or — for a non-git corpus —
// a content hash over exactly the manifest's SELECTED files (include minus
// exclude, via resolveSourceInventory), never the whole directory.
//
// This scoping matters specifically for a module: module.cue itself (and
// any eval/pack artifacts a workflow drops beside it) typically lives
// INSIDE the same root directory the corpus is discovered from, but is not
// itself part of the declared corpus. internal/connector's own
// ManifestWatermark hashes the FULL Discover result — reusing it directly
// here would make a module's watermark self-invalidate the instant `module
// init` writes module.cue next to the corpus it just measured (found via
// scripts/module-cold-run.sh's own end-to-end run: a real, non-git module
// directory drifted on every subsequent `module map`/`doctor` call purely
// because its own manifest file existed on disk).
func moduleWatermark(ctx context.Context, root string, m modspec.Manifest) (string, error) {
	if head := moduleGitHead(root); head != "" {
		return head, nil
	}
	inv, err := resolveSourceInventory(ctx, root, m)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	for _, f := range inv.Files { // resolveSourceInventory sorts by Path
		data, rerr := os.ReadFile(filepath.Join(root, filepath.FromSlash(f.Path)))
		if rerr != nil {
			return "", fmt.Errorf("read %s for watermark: %w", f.Path, rerr)
		}
		fh := sha256.Sum256(data)
		fmt.Fprintf(h, "%s\x00%s\n", f.Path, hex.EncodeToString(fh[:]))
	}
	return "manifest:" + hex.EncodeToString(h.Sum(nil)), nil
}

// moduleGitHead returns dir's git HEAD commit, or "" when dir is not inside
// a git work tree (or git is unavailable) — mirrors internal/connector's
// unexported gitHead exactly (that package's own doc comment names the
// identical algorithm); duplicated here rather than exported from
// internal/connector to avoid growing that package's public surface for a
// one-line helper only cmd/fathom needs.
func moduleGitHead(dir string) string {
	//fathom:tenancy-allow exec-noctx one-shot CLI verb with no request context; the git/child call ends with the process
	out, err := exec.Command("git", "-C", dir, "rev-parse", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// splitCSV splits a comma-separated flag value into a trimmed, non-empty
// slice; an empty input yields nil.
func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	for _, part := range strings.Split(s, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

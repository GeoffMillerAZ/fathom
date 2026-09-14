// freshness.go is VB-17 (docs/plans/verbs.md; design docs/vision/29-verbs-
// not-lookups.md §3 wave D; the M7 row in docs/vision/27-strengths-bench.md
// §1/§6): the L0 freshness probe. graphify's own docs name the gap this
// closes (docs/bench/competitors.md "Staleness model"): a precomputed
// artifact answers from BEFORE the edit until the next `--update`/`--watch`
// refresh. This probe measures, for free and on every commit, whether
// fathom's own index has the same gap.
//
// Method, LLM-free and deterministic: stage a MUTABLE copy of one fixture
// service under a temp dir, ingest it once, then apply N scripted mechanical
// edits in sequence — rename a function, add a parameter (with a new call
// site), delete a function, move a function to another file, change a
// return-value literal. Each edit is paired with one question whose correct
// answer is different before and after the edit — an answer only the
// CURRENT on-disk state (HEAD) can give — asked through the same in-process
// query surface fathom's MCP tools call (pkg/client.Client: FindSymbol
// backs find_symbol, FindCallers backs find_callers, Sources backs
// references' occurrence half and read_source), once right after the edit
// (BEFORE re-ingest) and once after the hash-gated re-ingest (AFTER).
//
// FindSymbol/FindCallers answer from internal/store, populated only at
// Ingest — genuinely stale until the re-ingest lands, which is what this
// probe is measuring. Sources() re-reads the connector live on every call
// (pkg/client.Client.Sources), so the one question answered through it (the
// changed-literal edit, Q5 below) is ALREADY correct before the explicit
// re-ingest step, exactly like read_source and grep are in
// docs/bench/competitors.md's own staleness table — a disclosed, expected
// asymmetry, not a probe defect: fathom's index has no field for a const or
// var's LITERAL VALUE (internal/extractor/goextract.go sets Signature only
// for func/method nodes), so the only in-process path that can answer "what
// value does X hold" is the live source read every other tool with that
// question shares.
package indexbench

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// freshnessServiceID is the probe's throwaway service id — never a real
// fixture service name, so a stray leftover store row is unmistakable.
const freshnessServiceID = "freshness-probe"

// FreshnessResult is one scripted edit's scored row.
type FreshnessResult struct {
	// ID names the edit (stable across runs — the golden's key).
	ID string `json:"id"`
	// Kind classifies the mechanical change: rename, add_param, delete,
	// move, or change_literal.
	Kind string `json:"kind"`
	// Question is the HEAD-only question this edit's Ask answers.
	Question string `json:"question"`
	// Want is the correct answer once the edit has landed (HEAD, after
	// Apply — independent of whether the index has caught up yet).
	Want string `json:"want"`
	// Before/After are the answers Ask actually returned, right after Apply
	// (before this edit's re-ingest) and after it.
	Before        string `json:"before"`
	CorrectBefore bool   `json:"correct_before"`
	After         string `json:"after"`
	CorrectAfter  bool   `json:"correct_after"`
	// MSToCorrect is wall-clock ms from immediately after Apply to the first
	// moment Ask returned Want — the before answer when it was already
	// correct (Q5's live-read case), else the after answer, which includes
	// the re-ingest's cost. Non-deterministic; TestFreshnessProbe gates it
	// against a generous committed ceiling, never an exact value.
	MSToCorrect int64 `json:"ms_to_correct"`
}

// FreshnessReport is the whole scored run.
type FreshnessReport struct {
	Edits         int               `json:"edits"`
	CorrectBefore int               `json:"correct_before"`
	CorrectAfter  int               `json:"correct_after"`
	Results       []FreshnessResult `json:"results"`
}

// freshnessAsk answers one edit's question through the in-process query
// surface, returning a comparable string ("<missing>" for a symbol/file
// that does not resolve).
type freshnessAsk func(ctx context.Context, c *client.Client, service string) (string, error)

// freshnessEdit is one scripted mechanical change plus the question that
// only its result (HEAD) can answer.
type freshnessEdit struct {
	ID       string
	Kind     string
	Question string
	Want     string
	// Apply mutates the tree rooted at dir to perform the edit.
	Apply func(dir string) error
	Ask   freshnessAsk
}

// FreshnessProbe (VB-17) runs the L0 freshness probe over a mutable copy of
// the fauxtown fixture fleet's ledger-service, staged under root (the
// caller's scratch directory — a test passes t.TempDir()).
func FreshnessProbe(ctx context.Context, root string) (FreshnessReport, error) {
	dir := filepath.Join(root, "freshness-svc")
	if err := stageFreshnessFixture(dir); err != nil {
		return FreshnessReport{}, err
	}

	st := store.NewMemoryStore()
	c := client.New(connector.NewLocalConnector(), st)
	ref := client.ServiceRef{ID: freshnessServiceID, Path: dir}
	if _, err := c.Ingest(ctx, ref); err != nil {
		return FreshnessReport{}, fmt.Errorf("indexbench: freshness baseline ingest: %w", err)
	}

	edits := freshnessEdits()
	rep := FreshnessReport{Edits: len(edits)}
	for _, e := range edits {
		res, err := runFreshnessEdit(ctx, c, ref, dir, e)
		if err != nil {
			return FreshnessReport{}, err
		}
		rep.Results = append(rep.Results, res)
		if res.CorrectBefore {
			rep.CorrectBefore++
		}
		if res.CorrectAfter {
			rep.CorrectAfter++
		}
	}
	return rep, nil
}

// runFreshnessEdit applies one edit and scores it. The re-ingest always
// runs (even when the before-answer was already correct) so the store
// stays in sync for the NEXT edit in the sequence — freshnessEdits() builds
// each edit on the tree the previous one left behind.
func runFreshnessEdit(ctx context.Context, c *client.Client, ref client.ServiceRef, dir string, e freshnessEdit) (FreshnessResult, error) {
	t0 := time.Now()
	if err := e.Apply(dir); err != nil {
		return FreshnessResult{}, fmt.Errorf("indexbench: freshness edit %s: apply: %w", e.ID, err)
	}
	before, err := e.Ask(ctx, c, freshnessServiceID)
	if err != nil {
		return FreshnessResult{}, fmt.Errorf("indexbench: freshness edit %s: ask before: %w", e.ID, err)
	}
	res := FreshnessResult{ID: e.ID, Kind: e.Kind, Question: e.Question, Want: e.Want, Before: before, CorrectBefore: before == e.Want}
	if res.CorrectBefore {
		res.MSToCorrect = time.Since(t0).Milliseconds()
	}

	if _, err := c.Ingest(ctx, ref); err != nil {
		return FreshnessResult{}, fmt.Errorf("indexbench: freshness edit %s: re-ingest: %w", e.ID, err)
	}
	after, err := e.Ask(ctx, c, freshnessServiceID)
	if err != nil {
		return FreshnessResult{}, fmt.Errorf("indexbench: freshness edit %s: ask after: %w", e.ID, err)
	}
	res.After = after
	res.CorrectAfter = after == e.Want
	if !res.CorrectBefore {
		res.MSToCorrect = time.Since(t0).Milliseconds()
	}
	return res, nil
}

// stageFreshnessFixture copies the fauxtown fixture fleet's ledger-service
// (internal/connector/testdata/fauxtown/ledger-service) into dir — a
// MUTABLE working copy the probe's edits write to, so the committed fixture
// itself is never touched.
func stageFreshnessFixture(dir string) error {
	src := filepath.Join(FauxtownRoot(), "ledger-service")
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("indexbench: stage freshness fixture: %w", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("indexbench: stage freshness fixture: %w", err)
	}
	for _, ent := range entries {
		if ent.IsDir() || !strings.HasSuffix(ent.Name(), ".go") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(src, ent.Name()))
		if err != nil {
			return fmt.Errorf("indexbench: stage freshness fixture %s: %w", ent.Name(), err)
		}
		if err := os.WriteFile(filepath.Join(dir, ent.Name()), data, 0o644); err != nil {
			return fmt.Errorf("indexbench: stage freshness fixture %s: %w", ent.Name(), err)
		}
	}
	return nil
}

// ─── the five scripted edits ────────────────────────────────────────────

// freshnessEdits returns the N=5 scripted mechanical edits, in the order
// they are applied — later edits build on the tree earlier ones left
// behind (edit 4 moves the function edit 3 already rewrote; edit 5 rewrites
// the file edit 4 just created).
func freshnessEdits() []freshnessEdit {
	return []freshnessEdit{
		{
			ID:       "rename",
			Kind:     "rename",
			Question: "what does find_symbol return for checkEntry (validateEntry, renamed)?",
			Want:     "ledger.go:GoFunc",
			Apply:    applyRenameEdit,
			Ask:      askFindSymbolFile("checkEntry"),
		},
		{
			ID:       "add_param",
			Kind:     "add_param",
			Question: "how many callers does applyEntry have now (a dryRun param plus a new caller, PostDryRun)?",
			Want:     "2",
			Apply:    applyAddParamEdit,
			Ask:      askCallerCount("applyEntry"),
		},
		{
			ID:       "delete",
			Kind:     "delete",
			Question: "does computeBalance still exist?",
			Want:     "<missing>",
			Apply:    applyDeleteEdit,
			Ask:      askFindSymbolFile("computeBalance"),
		},
		{
			ID:       "move",
			Kind:     "move",
			Question: "which file defines Balance now?",
			Want:     "query.go:GoFunc",
			Apply:    applyMoveEdit,
			Ask:      askFindSymbolFile("Balance"),
		},
		{
			ID:       "change_literal",
			Kind:     "change_literal",
			Question: "what value does Balance's non-empty branch return now?",
			Want:     "7",
			Apply:    applyChangeLiteralEdit,
			Ask:      askBalanceLiteral,
		},
	}
}

// applyRenameEdit renames validateEntry -> checkEntry, including its one
// call site in Post.
func applyRenameEdit(dir string) error {
	p := filepath.Join(dir, "ledger.go")
	if err := replaceOnce(p, "func validateEntry(e Entry) error {", "func checkEntry(e Entry) error {"); err != nil {
		return err
	}
	return replaceOnce(p, "if err := validateEntry(e); err != nil {", "if err := checkEntry(e); err != nil {")
}

// applyAddParamEdit adds a dryRun parameter to applyEntry, updates Post's
// call site, and adds a brand new caller (PostDryRun) — the edit that
// changes applyEntry's caller COUNT, not just its signature.
func applyAddParamEdit(dir string) error {
	p := filepath.Join(dir, "ledger.go")
	if err := replaceOnce(p, "func applyEntry(e Entry) error {", "func applyEntry(e Entry, dryRun bool) error {"); err != nil {
		return err
	}
	if err := replaceOnce(p, "\treturn applyEntry(e)\n", "\treturn applyEntry(e, false)\n"); err != nil {
		return err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return fmt.Errorf("indexbench: add_param edit: %w", err)
	}
	data = append(data, []byte("\n// PostDryRun validates and simulates an entry without persisting it.\nfunc PostDryRun(ctx context.Context, e Entry) error {\n\treturn applyEntry(e, true)\n}\n")...)
	return os.WriteFile(p, data, 0o644)
}

// applyDeleteEdit deletes computeBalance outright, inlining its one call
// site (in Balance) to the literal it always returned.
func applyDeleteEdit(dir string) error {
	p := filepath.Join(dir, "ledger.go")
	if err := replaceOnce(p, "\treturn computeBalance(accountID), nil\n", "\treturn 0, nil\n"); err != nil {
		return err
	}
	return replaceOnce(p,
		"\n// computeBalance sums all entries for an account.\nfunc computeBalance(accountID string) int64 {\n\t_ = accountID\n\treturn 0\n}\n",
		"\n")
}

// balanceBlock is Balance's full declaration (doc comment included) AFTER
// applyDeleteEdit has inlined its computeBalance call — applyMoveEdit cuts
// exactly this block out of ledger.go and into a new file.
const balanceBlock = "\n// Balance returns the net balance for accountID.\nfunc Balance(ctx context.Context, accountID string) (int64, error) {\n\tif accountID == \"\" {\n\t\treturn 0, errMissingAccount\n\t}\n\treturn 0, nil\n}\n"

// applyMoveEdit moves Balance out of ledger.go into a new file, query.go,
// in the same package.
func applyMoveEdit(dir string) error {
	p := filepath.Join(dir, "ledger.go")
	if err := replaceOnce(p, balanceBlock, "\n"); err != nil {
		return err
	}
	query := "// Package ledger — balance queries, split out of ledger.go.\npackage ledger\n\nimport \"context\"\n" + balanceBlock
	return os.WriteFile(filepath.Join(dir, "query.go"), []byte(query), 0o644)
}

// applyChangeLiteralEdit changes the literal Balance's non-empty branch
// returns, in query.go (where applyMoveEdit just put it).
func applyChangeLiteralEdit(dir string) error {
	return replaceOnce(filepath.Join(dir, "query.go"), "return 0, nil", "return 7, nil")
}

// replaceOnce reads path, replaces EXACTLY one occurrence of old with new,
// and writes the result back — refusing (rather than silently no-op'ing or
// clobbering the wrong spot) when old appears zero or more-than-one times,
// so a fixture drift breaks this test loudly instead of scoring nonsense.
func replaceOnce(path, old, new string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("indexbench: %s: %w", path, err)
	}
	s := string(data)
	if n := strings.Count(s, old); n != 1 {
		return fmt.Errorf("indexbench: %s: expected exactly one occurrence of %q, found %d", path, old, n)
	}
	return os.WriteFile(path, []byte(strings.Replace(s, old, new, 1)), 0o644)
}

// askFindSymbolFile answers "what/where is name" through FindSymbol — the
// same store-backed call the find_symbol tool makes — as "<file>:<kind>",
// or "<missing>" when no symbol by that name resolves.
func askFindSymbolFile(name string) freshnessAsk {
	return func(ctx context.Context, c *client.Client, service string) (string, error) {
		nodes, err := c.FindSymbol(ctx, service, name)
		if err != nil {
			return "", err
		}
		if len(nodes) == 0 {
			return "<missing>", nil
		}
		return fmt.Sprintf("%s:%s", nodes[0].FilePath, nodes[0].Kind), nil
	}
}

// askCallerCount answers "how many callers does name have" through
// FindCallers — the same store-backed call the find_callers tool makes.
func askCallerCount(name string) freshnessAsk {
	return func(ctx context.Context, c *client.Client, service string) (string, error) {
		nodes, err := c.FindCallers(ctx, service, name)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%d", len(nodes)), nil
	}
}

// reBalanceLiteral matches Balance's non-empty-account return statement's
// literal — distinct from its other return (`return 0, errMissingAccount`)
// by the trailing ", nil".
var reBalanceLiteral = regexp.MustCompile(`return (\d+), nil`)

// askBalanceLiteral answers "what value does Balance's non-empty branch
// return" through Sources() — the same live-disk read the references
// tool's occurrence half and read_source use (see the package doc comment
// for why this is the one question in the set that a live read, not the
// store, must answer).
func askBalanceLiteral(ctx context.Context, c *client.Client, service string) (string, error) {
	srcs, err := c.Sources(ctx, service)
	if err != nil {
		return "", err
	}
	for _, sf := range srcs {
		if sf.Path != "query.go" {
			continue
		}
		m := reBalanceLiteral.FindStringSubmatch(string(sf.Content))
		if m == nil {
			return "<no literal>", nil
		}
		return m[1], nil
	}
	return "<missing file>", nil
}

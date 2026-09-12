package contextbench

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Remembered cells (ADR-032 §2; docs/design/bench-pyramid.md §3.4). A run
// used to be an atomic purchase: 117 cells, ~15M tokens, all or nothing,
// even though the grep and `none` arms do not depend on a single line of
// fathom's code. The cell is the unit that is actually bought, and what
// binds it is a short list of invariants — the corpus bytes, the task, the
// arm, the model, the harness, the scoring rule, the prompt subject, and
// (for the fathom arm only) which fathom the arm was talking to.
//
// Those invariants hash to a cell key. A cell is stored content-addressed
// under that key, and a later run borrows any cell whose key it recomputes
// to the same value. A changed invariant produces a different key, so a
// borrow can never be silently wrong: it either matches or it does not
// exist. The evaluator additionally refuses a borrowed cell whose recorded
// key is not the key this run's invariants produce (AcceptBorrowed), and a
// row composed entirely of borrowed cells measures nothing new and is
// indeterminate with that reason named.

// CellInputs are the invariants that bind one scored cell. Every field is
// part of the key; a cell whose inputs differ in any of them is a
// different cell, not the same cell measured again.
type CellInputs struct {
	// CorpusSHA256 is the hash of the corpus file's bytes (CorpusSHA256).
	CorpusSHA256 string
	TaskID       string
	Arm          string
	// ModelID is the model identity the run was pinned to: the dated id
	// when the operator pins one (`--model-id`), else the alias the runner
	// asked for. An alias is self-consistent but weaker — if the alias
	// re-resolves to a newer dated model, cells keyed on the alias keep
	// matching. The ledger row records the RESOLVED id beside the key so a
	// reader can see when that happened; pin --model-id to the previous
	// row's model_id to borrow across a model release safely.
	ModelID string
	// HarnessVersion is the `claude --version` string the runner captured.
	// O24's finding — that Claude Code 2.1.x defers MCP tool schemas and
	// costs the fathom arm one or two ToolSearch turns the grep arm never
	// pays — is why the harness is an invariant and not a footnote.
	HarnessVersion string
	// Scoring is the criterion's scoring mode (ScoringSubstring or
	// ScoringPrecision): the same transcript scored two ways is two cells.
	Scoring string
	// PromptSubject is the runner's --prompt-subject (the corpus is
	// parameterized by subject, PB-6).
	PromptSubject string
	// ArmSurface identifies the subject-under-test for arms that have one:
	// the fathom binary commit for the fathom arm, empty for grep and
	// none. This is the whole saving — a fathom change changes the fathom
	// arm's key and nothing else's, so control cells survive it.
	ArmSurface string
}

// canonical renders the inputs as labeled, newline-separated lines. Labels
// make the hash unambiguous: without them "ab"+"c" and "a"+"bc" would
// collide, and a key that can collide is worse than no key at all.
func (in CellInputs) canonical() string {
	var b strings.Builder
	for _, kv := range [][2]string{
		{"corpus_sha256", in.CorpusSHA256},
		{"task_id", in.TaskID},
		{"arm", in.Arm},
		{"model_id", in.ModelID},
		{"harness_version", in.HarnessVersion},
		{"scoring", in.Scoring},
		{"prompt_subject", in.PromptSubject},
		{"arm_surface", in.ArmSurface},
	} {
		b.WriteString(kv[0])
		b.WriteByte('=')
		b.WriteString(kv[1])
		b.WriteByte('\n')
	}
	return b.String()
}

// CellKey is the content address of one scored cell: sha256 over the
// labeled invariants, hex-encoded. It is a pure function of its input, so
// two runs on two machines that agree on the invariants agree on the key.
func CellKey(in CellInputs) string {
	sum := sha256.Sum256([]byte(in.canonical()))
	return hex.EncodeToString(sum[:])
}

// Validate reports the first invariant a key cannot be built without. An
// empty invariant is not a neutral value: it would make two genuinely
// different cells hash the same.
func (in CellInputs) Validate() error {
	for _, kv := range [][2]string{
		{"corpus_sha256", in.CorpusSHA256},
		{"task_id", in.TaskID},
		{"arm", in.Arm},
		{"model_id", in.ModelID},
		{"harness_version", in.HarnessVersion},
		{"scoring", in.Scoring},
		{"prompt_subject", in.PromptSubject},
	} {
		if kv[1] == "" {
			return fmt.Errorf("cell key: %s is empty — a key over an unknown invariant would let an unrelated cell be borrowed", kv[0])
		}
	}
	return nil
}

// cellTrialFile is the per-trial file name inside a cell directory.
func cellTrialFile(trial int) string {
	if trial < 1 {
		trial = 1
	}
	return fmt.Sprintf("t%d.json", trial)
}

// CellPath returns the content-addressed path of one trial of one cell:
// <cellsDir>/<key>/t<n>.json.
func CellPath(cellsDir, key string, trial int) string {
	return filepath.Join(cellsDir, key, cellTrialFile(trial))
}

// StoreCell writes a scored cell into the content-addressed store. The
// result must carry the key it was scored under. A cell already present is
// LEFT ALONE: the first writer owns the provenance, so re-running a cell
// whose invariants did not change cannot quietly rewrite which run the
// reading came from. A borrowed cell is never written back — it is already
// in the store, under this exact key.
func StoreCell(cellsDir string, r ArmResult) error {
	if r.CellKey == "" {
		return fmt.Errorf("storing cell %s/%s: no cell_key", r.TaskID, r.Arm)
	}
	if r.Borrowed {
		return nil
	}
	dir := filepath.Join(cellsDir, r.CellKey)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating cell dir %s: %w", dir, err)
	}
	path := filepath.Join(dir, cellTrialFile(r.Trial))
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	raw, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling cell %s: %w", r.CellKey, err)
	}
	if err := os.WriteFile(path, append(raw, '\n'), 0o644); err != nil {
		return fmt.Errorf("writing cell %s: %w", path, err)
	}
	return nil
}

// BorrowCell returns the stored cell for (key, trial) marked Borrowed,
// with the run that paid for it named in SourceRunID. The second return is
// false when no such cell exists — the caller then runs the cell for real.
// A read error is an error, never a miss: "I could not read it" and "it is
// not there" are different facts (ADR-027).
func BorrowCell(cellsDir, key string, trial int) (ArmResult, bool, error) {
	path := CellPath(cellsDir, key, trial)
	raw, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return ArmResult{}, false, nil
		}
		return ArmResult{}, false, fmt.Errorf("reading cell %s: %w", path, err)
	}
	var r ArmResult
	if err := json.Unmarshal(raw, &r); err != nil {
		return ArmResult{}, false, fmt.Errorf("parsing cell %s: %w", path, err)
	}
	if r.CellKey != key {
		return ArmResult{}, false, fmt.Errorf("cell %s records key %s — the store is inconsistent with its own addressing", path, r.CellKey)
	}
	r.Borrowed = true
	if r.SourceRunID == "" {
		r.SourceRunID = "unrecorded"
	}
	return r, true, nil
}

// AcceptBorrowed is the evaluator's admission rule for a borrowed cell:
// key equality, and nothing else. want is the key THIS run's invariants
// produce for the cell; a borrowed reading whose recorded key differs was
// bought under different invariants and may not stand in for it. The error
// names the cell, so a refusal is never anonymous.
func AcceptBorrowed(want string, r ArmResult) error {
	if !r.Borrowed {
		return nil
	}
	if r.CellKey == "" {
		return fmt.Errorf("borrowed cell %s/%s/t%d (from run %s) carries no cell_key — provenance unverifiable", r.TaskID, r.Arm, r.Trial, orUnrecorded(r.SourceRunID))
	}
	if r.CellKey != want {
		return fmt.Errorf("borrowed cell %s/%s/t%d (from run %s) has key %s, but this run's invariants produce %s — refused", r.TaskID, r.Arm, r.Trial, orUnrecorded(r.SourceRunID), short(r.CellKey), short(want))
	}
	return nil
}

func orUnrecorded(s string) string {
	if s == "" {
		return "unrecorded"
	}
	return s
}

// RefuseMismatchedBorrows splits results into the cells this run may score
// and the refusals, recomputing each borrowed cell's key from base (whose
// TaskID, Arm and ArmSurface are filled per result from surfaces). A
// refused cell is DROPPED, which makes its (task, arm) incomplete and the
// row indeterminate through the existing coverage rule — never silently
// scored. Both outputs are deterministic: kept keeps input order, refused
// is sorted.
func RefuseMismatchedBorrows(results []ArmResult, base CellInputs, surfaces map[string]string) (kept []ArmResult, refused []string) {
	for _, r := range results {
		if !r.Borrowed {
			kept = append(kept, r)
			continue
		}
		in := base
		in.TaskID, in.Arm = r.TaskID, r.Arm
		in.ArmSurface = surfaces[r.Arm]
		if err := AcceptBorrowed(CellKey(in), r); err != nil {
			refused = append(refused, err.Error())
			continue
		}
		kept = append(kept, r)
	}
	sort.Strings(refused)
	return kept, refused
}

// BorrowedCell is one borrowed reading's provenance, as the ledger row
// records it.
type BorrowedCell struct {
	TaskID      string `json:"task_id"`
	Arm         string `json:"arm"`
	Trial       int    `json:"trial"`
	CellKey     string `json:"cell_key"`
	SourceRunID string `json:"source_run_id"`
}

// BorrowedCellsOf lists every borrowed reading in results, sorted by
// (task, arm, trial) — the provenance the ledger row carries so a reader
// can tell which numbers this run paid for and which it inherited.
func BorrowedCellsOf(results []ArmResult) []BorrowedCell {
	var out []BorrowedCell
	for _, r := range results {
		if !r.Borrowed {
			continue
		}
		out = append(out, BorrowedCell{TaskID: r.TaskID, Arm: r.Arm, Trial: r.Trial, CellKey: r.CellKey, SourceRunID: orUnrecorded(r.SourceRunID)})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].TaskID != out[j].TaskID {
			return out[i].TaskID < out[j].TaskID
		}
		if out[i].Arm != out[j].Arm {
			return out[i].Arm < out[j].Arm
		}
		return out[i].Trial < out[j].Trial
	})
	return out
}

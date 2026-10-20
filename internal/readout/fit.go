package readout

// fit.go — the fitted table (design docs/design/readouts.md §13, ADR-073's
// first decision model, provider none): the policy table's byte floor and
// long-read cut are fitted on a seeded half of the outcome labels the
// readout report's replay computes and scored on the other half. The fit is
// pure and deterministic: the halves come from a hash of the seed and each
// call's id, never from the labels' order, and ties keep the larger cut
// point. The fitted cut points persist in the user's policy file
// (PolicyPath), written atomically, which rich mode reads in place of the
// built-in ones.

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	toml "github.com/pelletier/go-toml/v2"

	"github.com/geoffmilleraz/fathom/internal/ask"
)

// fitMaxCandidates bounds the cut points tried per fit: the sorted distinct
// values, thinned to evenly spaced ones past it.
const fitMaxCandidates = 512

// FitLabel is one replayed call's outcome: its features and which answer
// was right, where the replay could tell.
type FitLabel struct {
	// ID is the call's tool-use id, the split's key.
	ID       string
	Features Features
	// LeanRight: nothing the lean view dropped was referenced and nothing
	// was recovered within five calls. LeanKnown when the replay computed it.
	LeanKnown, LeanRight bool
	// EnrichRight: the next call touched a file or symbol a record named.
	// EnrichKnown when the call's row logged the table's features.
	EnrichKnown, EnrichRight bool
}

// FitAccuracy is the table's accuracy over a set of scored labels, with its
// 95% Wilson bounds.
type FitAccuracy struct {
	N        int     `json:"n"`
	Right    int     `json:"right"`
	Accuracy float64 `json:"accuracy"`
	Lower    float64 `json:"lower"`
	Upper    float64 `json:"upper"`
}

// FitHalf is one half's reading: every scored label, and those in the
// contested band.
type FitHalf struct {
	Labels    int         `json:"labels"`
	All       FitAccuracy `json:"all"`
	Contested FitAccuracy `json:"contested"`
}

// FitResult is a fit: the fitted cut points, the built-in ones, and both
// halves scored with the fitted ones.
type FitResult struct {
	Seed     int64   `json:"seed"`
	Labels   int     `json:"labels"`
	Unscored int     `json:"unscored"`
	Policy   Policy  `json:"policy"`
	Builtin  Policy  `json:"builtin"`
	Fit      FitHalf `json:"fit"`
	HeldOut  FitHalf `json:"held_out"`
}

// Fit is fitPolicyTable for callers outside the package.
func Fit(labels []FitLabel, seed int64) FitResult { return fitPolicyTable(labels, seed) }

// fitPolicyTable fits the byte floor, then the long-read cut, on the seeded
// half and scores both halves with them. Answers are scored as the table
// would serve a promoted rule, so the cut points are read although no rule
// is promoted: a lean label against lean or raw, an enrichment label (rules
// 1 and 2) against enriched or raw. A label neither outcome reached is
// unscored.
func fitPolicyTable(labels []FitLabel, seed int64) FitResult {
	train, test := splitHalves(labels, seed)
	p := BuiltinPolicy()
	p.LeanFloorBytes = bestCut(train, p, candidates(train, func(l FitLabel) (int, bool) {
		return l.Features.Bytes, l.LeanKnown
	}, 0), func(q *Policy, v int) { q.LeanFloorBytes = v })
	p.LongReadLines = bestCut(train, p, candidates(train, func(l FitLabel) (int, bool) {
		return l.Features.Lines, l.LeanKnown && l.Features.WholeRead
	}, BuiltinLongReadLines), func(q *Policy, v int) { q.LongReadLines = max(v, 1) })
	r := FitResult{Seed: seed, Labels: len(labels), Policy: p, Builtin: BuiltinPolicy()}
	r.Fit = scoreHalf(train, p)
	r.HeldOut = scoreHalf(test, p)
	r.Unscored = len(labels) - r.Fit.All.N - r.HeldOut.All.N
	return r
}

// splitHalves orders the labels by a hash of the seed and each id (the id
// breaks ties) and cuts them in two; the first half, the smaller when the
// count is odd, is the one fitted.
func splitHalves(labels []FitLabel, seed int64) (train, test []FitLabel) {
	type keyed struct {
		k uint64
		l FitLabel
	}
	ks := make([]keyed, len(labels))
	s := strconv.FormatInt(seed, 10) + "\x00"
	for i, l := range labels {
		sum := sha256.Sum256([]byte(s + l.ID))
		ks[i] = keyed{binary.BigEndian.Uint64(sum[:8]), l}
	}
	sort.SliceStable(ks, func(i, j int) bool {
		if ks[i].k != ks[j].k {
			return ks[i].k < ks[j].k
		}
		return ks[i].l.ID < ks[j].l.ID
	})
	half := len(ks) / 2
	for i, k := range ks {
		if i < half {
			train = append(train, k.l)
		} else {
			test = append(test, k.l)
		}
	}
	return train, test
}

// candidates is the cut points to try: the built-in one and every distinct
// value the labels that take part carry, sorted, thinned past
// fitMaxCandidates.
func candidates(ls []FitLabel, value func(FitLabel) (int, bool), builtin int) []int {
	seen := map[int]bool{builtin: true}
	vs := []int{builtin}
	for _, l := range ls {
		if v, ok := value(l); ok && !seen[v] {
			seen[v] = true
			vs = append(vs, v)
		}
	}
	sort.Ints(vs)
	if len(vs) <= fitMaxCandidates {
		return vs
	}
	out := make([]int, 0, fitMaxCandidates)
	for i := 0; i < fitMaxCandidates; i++ {
		out = append(out, vs[i*(len(vs)-1)/(fitMaxCandidates-1)])
	}
	return out
}

// bestCut is the candidate (cands is sorted and never empty: it holds the
// built-in) that scores the most labels right with set applied to p; a tie
// keeps the larger cut point, the one that serves less lean.
func bestCut(ls []FitLabel, p Policy, cands []int, set func(*Policy, int)) int {
	best, bestRight := cands[0], -1
	for _, v := range cands {
		q := p
		set(&q, v)
		right := 0
		for _, l := range ls {
			if ok, scored, _ := scoreLabel(l, q); scored && ok {
				right++
			}
		}
		if right >= bestRight {
			best, bestRight = v, right
		}
	}
	return best
}

// scoreLabel scores the table's answer for l under p, as it would serve a
// promoted rule: right, whether an outcome was known, and whether the call
// sits in the contested band.
func scoreLabel(l FitLabel, p Policy) (right, scored, contested bool) {
	f := l.Features
	f.Promoted = true
	c := Choose(f, p)
	switch {
	case l.EnrichKnown && (c.Rule == 1 || c.Rule == 2):
		return (c.Mode == ChoiceEnriched) == l.EnrichRight, true, c.Contested
	case l.LeanKnown:
		return (c.Mode == ChoiceLean) == l.LeanRight, true, c.Contested
	}
	return false, false, c.Contested
}

// scoreHalf reads ls under p.
func scoreHalf(ls []FitLabel, p Policy) FitHalf {
	h := FitHalf{Labels: len(ls)}
	for _, l := range ls {
		right, scored, contested := scoreLabel(l, p)
		if !scored {
			continue
		}
		h.All.N++
		if contested {
			h.Contested.N++
		}
		if right {
			h.All.Right++
			if contested {
				h.Contested.Right++
			}
		}
	}
	h.All.finish()
	h.Contested.finish()
	return h
}

func (a *FitAccuracy) finish() {
	if a.N == 0 {
		return
	}
	lo, hi := ask.Wilson(a.Right, a.N)
	a.Accuracy = fitRound(float64(a.Right) / float64(a.N))
	a.Lower, a.Upper = fitRound(lo), fitRound(hi)
}

func fitRound(f float64) float64 { return math.Round(f*1e4) / 1e4 }

// PolicyFile is the fitted policy's place under the fathom directory.
const PolicyFile = "readouts/policy.toml"

// policyVersion is the policy file's format version.
const policyVersion = 1

// policyFile is the file's form: the cut points and the fit's provenance.
type policyFile struct {
	Version         int     `toml:"version"`
	Seed            int64   `toml:"seed"`
	Labels          int     `toml:"labels"`
	HeldOutAccuracy float64 `toml:"held_out_accuracy"`
	LeanFloorBytes  int     `toml:"lean_floor_bytes"`
	LongReadLines   int     `toml:"long_read_lines"`
}

// PolicyPath is the policy file under fathomDir ("" when fathomDir is).
func PolicyPath(fathomDir string) string {
	if fathomDir == "" {
		return ""
	}
	return filepath.Join(fathomDir, filepath.FromSlash(PolicyFile))
}

// LoadPolicyFile reads the fitted cut points under fathomDir; found says
// the file answered (it exists and is valid). An absent file (or no fathom
// directory) is the built-ins and no error; an invalid one is the built-ins
// and the error saying why, which doctor names.
func LoadPolicyFile(fathomDir string) (p Policy, found bool, err error) {
	path := PolicyPath(fathomDir)
	if path == "" {
		return BuiltinPolicy(), false, nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return BuiltinPolicy(), false, nil
	}
	if err != nil {
		return BuiltinPolicy(), false, err
	}
	var pf policyFile
	dec := toml.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&pf); err != nil {
		return BuiltinPolicy(), false, fmt.Errorf("%s: %v", path, err)
	}
	switch {
	case pf.Version != policyVersion:
		return BuiltinPolicy(), false, fmt.Errorf("%s: version %d, want %d", path, pf.Version, policyVersion)
	case pf.LeanFloorBytes < 0:
		return BuiltinPolicy(), false, fmt.Errorf("%s: lean_floor_bytes %d is negative", path, pf.LeanFloorBytes)
	case pf.LongReadLines < 1:
		return BuiltinPolicy(), false, fmt.Errorf("%s: long_read_lines %d, want at least 1", path, pf.LongReadLines)
	}
	return Policy{LeanFloorBytes: pf.LeanFloorBytes, LongReadLines: pf.LongReadLines}, true, nil
}

// WritePolicy writes r's cut points under fathomDir atomically: a temporary
// file beside it, then a rename, so a reader sees the old file or the new.
func WritePolicy(fathomDir string, r FitResult) error {
	path := PolicyPath(fathomDir)
	if path == "" {
		return errors.New("no fathom directory to write the policy under")
	}
	body, err := toml.Marshal(policyFile{
		Version: policyVersion, Seed: r.Seed, Labels: r.Labels, HeldOutAccuracy: r.HeldOut.All.Accuracy,
		LeanFloorBytes: r.Policy.LeanFloorBytes, LongReadLines: r.Policy.LongReadLines,
	})
	if err != nil {
		return err
	}
	head := "# The readout policy table's fitted cut points, written by `fathom readout fit --write`.\n" +
		"# Rich mode reads them in place of the built-in ones; delete this file to restore those.\n"
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".policy-*.toml")
	if err != nil {
		return err
	}
	_, werr := tmp.WriteString(head + string(body))
	cerr := tmp.Close()
	if werr == nil {
		werr = cerr
	}
	if werr == nil {
		werr = os.Rename(tmp.Name(), path)
	}
	if werr != nil {
		_ = os.Remove(tmp.Name())
	}
	return werr
}

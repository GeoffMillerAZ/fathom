package view_test

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/pkg/lang"
	"github.com/geoffmilleraz/fathom/pkg/lang/view"
)

// The reference below follows the HLP-1 spec as amended by ESCALATE-27 R3 and R4, as plain
// loops over every symbol. It shares no code with the implementation.

func refKnown(s lang.Symbol) bool { return s.EndLine >= s.Line && s.Line > 0 }

func refEnclosing(r lang.Result, line int) (lang.Symbol, view.Status) {
	if line < 1 || r.Skipped != "" {
		return lang.Symbol{}, view.StatusUnknown
	}
	var cand lang.Symbol
	have := false
	for _, s := range r.Symbols {
		if !refKnown(s) || line < s.Line || line > s.EndLine {
			continue
		}
		better := !have ||
			s.Line > cand.Line ||
			(s.Line == cand.Line && s.EndLine < cand.EndLine) ||
			(s.Line == cand.Line && s.EndLine == cand.EndLine && s.Key > cand.Key)
		if better {
			cand, have = s, true
		}
	}
	candLine := 0
	if have {
		candLine = cand.Line
	}
	hidden := false
	for _, s := range r.Symbols {
		if refKnown(s) {
			continue
		}
		if have && (s.Line < 1 || s.Line >= candLine) && s.Line <= line {
			hidden = true
		}
		if !have && s.Line <= line {
			hidden = true
		}
	}
	if r.Incomplete || hidden {
		return cand, view.StatusUnknown
	}
	if !have {
		return lang.Symbol{}, view.StatusNone
	}
	return cand, view.StatusFound
}

func refTouched(r lang.Result, start, end int) ([]lang.Symbol, bool) {
	if start < 1 || end < start || r.Skipped != "" {
		return nil, false
	}
	var out []lang.Symbol
	complete := !r.Incomplete
	for _, s := range r.Symbols {
		if refKnown(s) {
			if s.Line <= end && s.EndLine >= start {
				out = append(out, s)
			}
			continue
		}
		if s.Line >= start && s.Line <= end {
			out = append(out, s)
		}
		if s.Line < start {
			complete = false
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		if a.EndLine != b.EndLine {
			return a.EndLine > b.EndLine
		}
		return a.Key < b.Key
	})
	return out, complete
}

// check compares the index with the reference over every line and a spread of ranges.
func check(t *testing.T, name string, r lang.Result) {
	t.Helper()
	ix := view.NewIndex(r)
	top := 3
	for _, s := range r.Symbols {
		top = max(top, s.Line+2, s.EndLine+2)
	}
	for line := -1; line <= top; line++ {
		wantS, wantSt := refEnclosing(r, line)
		gotS, gotSt := ix.Enclosing(line)
		if gotSt != wantSt || (line >= 1 && gotS != wantS) {
			t.Fatalf("%s: Enclosing(%d) = %q %q, reference %q %q", name, line, gotS.Key, gotSt, wantS.Key, wantSt)
		}
		for end := line - 1; end <= min(top, line+40); end++ {
			wantT, wantC := refTouched(r, line, end)
			gotT, gotC := ix.Touched(line, end)
			if gotC != wantC || len(gotT) != len(wantT) || (len(wantT) > 0 && !reflect.DeepEqual(gotT, wantT)) {
				t.Fatalf("%s: Touched(%d,%d) = %v %v, reference %v %v", name, line, end, keys(gotT), gotC, keys(wantT), wantC)
			}
		}
	}
}

// genResult builds a symbol set that stresses the tie-breaks: nested, overlapping, equal spans,
// unknown spans (EndLine 0 or below Line) and incomplete reads. Keys are unique.
func genResult(rng *rand.Rand) lang.Result {
	n := rng.Intn(14)
	r := lang.Result{Dialect: "gen"}
	perm := rng.Perm(n + 3)
	for i := 0; i < n; i++ {
		line := 1 + rng.Intn(24)
		s := lang.Symbol{Key: fmt.Sprintf("k%02d", perm[i]), Line: line}
		s.Name = s.Key
		switch rng.Intn(10) {
		case 0:
			s.EndLine = 0
		case 1:
			s.EndLine = line - 1
		case 2:
			s.EndLine = line // one line
		default:
			s.EndLine = line + rng.Intn(14)
		}
		r.Symbols = append(r.Symbols, s)
		if rng.Intn(5) == 0 && i+1 < n { // an equal span, different key
			i++
			t := s
			t.Key = fmt.Sprintf("k%02d", perm[i])
			t.Name = t.Key
			r.Symbols = append(r.Symbols, t)
		}
	}
	r.Incomplete = rng.Intn(8) == 0
	if rng.Intn(40) == 0 {
		r = lang.Result{Dialect: "", Skipped: lang.SkipBinary, Symbols: []lang.Symbol{}}
	}
	return r
}

func TestPropertyGeneratedMatchesReference(t *testing.T) {
	rng := rand.New(rand.NewSource(20261123))
	for i := 0; i < 400; i++ {
		r := genResult(rng)
		check(t, fmt.Sprintf("gen#%d", i), r)
		// input order must not matter
		rng.Shuffle(len(r.Symbols), func(a, b int) { r.Symbols[a], r.Symbols[b] = r.Symbols[b], r.Symbols[a] })
		check(t, fmt.Sprintf("gen#%d shuffled", i), r)
	}
}

// pinnedResults reads every lang.Result a golden in this repository pins: the extract golden of
// fathom-lang and the golden Results of the sample dialect pack.
func pinnedResults(t *testing.T) map[string]lang.Result {
	t.Helper()
	out := map[string]lang.Result{}
	extract := filepath.Join("..", "..", "..", "cmd", "fathom-lang", "testdata", "golden", "extract.json")
	raw, err := os.ReadFile(extract)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Files []struct {
			Path   string       `json:"path"`
			Result *lang.Result `json:"result"`
		} `json:"files"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	for _, f := range doc.Files {
		if f.Result != nil {
			out["extract.json:"+f.Path] = *f.Result
		}
	}
	packs, err := filepath.Glob(filepath.Join("..", "langtest", "testdata", "samplepack", "dialects", "*", "testdata", "*.golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range packs {
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		var r lang.Result
		if err := json.Unmarshal(raw, &r); err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		out[filepath.ToSlash(strings.TrimPrefix(p, filepath.Join("..", "langtest")+string(filepath.Separator)))] = r
	}
	dialects, err := filepath.Glob(filepath.Join("..", "..", "..", "internal", "extractor", "dialects", "*", "testdata", "*.golden.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(dialects) == 0 {
		t.Fatal("no dialect goldens found")
	}
	for _, p := range dialects {
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		var r lang.Result
		if strings.HasPrefix(strings.TrimSpace(string(raw)), "[") { // a golden of bare symbols
			err = json.Unmarshal(raw, &r.Symbols)
		} else {
			err = json.Unmarshal(raw, &r)
		}
		if err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		for i := range r.Symbols { // dialect goldens carry no Key: give each symbol a unique one
			r.Symbols[i].Key = fmt.Sprintf("%s#%04d", r.Symbols[i].Name, i)
		}
		out["dialect:"+filepath.ToSlash(strings.TrimPrefix(p, filepath.Join("..", "..", "..", "internal", "extractor", "dialects")+string(filepath.Separator)))] = r
	}
	return out
}

func TestPropertyPinnedGoldenResultsMatchReference(t *testing.T) {
	pinned := pinnedResults(t)
	if len(pinned) < 5 {
		t.Fatalf("found %d pinned golden Results, expected the fathom-lang extract golden and the sample pack goldens", len(pinned))
	}
	names := make([]string, 0, len(pinned))
	for n := range pinned {
		names = append(names, n)
	}
	sort.Strings(names)
	rng := rand.New(rand.NewSource(7))
	withSpan := 0
	for _, n := range names {
		r := pinned[n]
		for _, s := range r.Symbols {
			if refKnown(s) {
				withSpan++
				break
			}
		}
		check(t, n, r)
		// the same Result read incomplete, and with some end lines withheld, as a weaker dialect would give it
		inc := r
		inc.Incomplete = true
		check(t, n+" (incomplete)", inc)
		weak := r
		weak.Symbols = append([]lang.Symbol(nil), r.Symbols...)
		for i := range weak.Symbols {
			if rng.Intn(2) == 0 {
				weak.Symbols[i].EndLine = 0
			}
		}
		check(t, n+" (end lines withheld)", weak)
	}
	if withSpan == 0 {
		t.Fatal("no pinned golden Result has a symbol with a known span: the property test would prove nothing")
	}
}

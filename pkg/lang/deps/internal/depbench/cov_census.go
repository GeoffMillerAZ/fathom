package depbench

// COV helpers belong to the test instrument, never the product reader.
import (
	"encoding/json"
	"path"
	"slices"
	"strings"

	"github.com/geoffmilleraz/fathom/pkg/lang/deps"
)

type COVReading struct {
	Reading
	ExternalAsLocal                     int
	RejectedInputs, RejectedOracleFiles int
}

// COVScore adds COV-D6 independently of native coverage. The native missing
// count and inventory-refuted external claims are separate evidence counts.
func COVScore(o Observation, inventory map[string]bool) COVReading {
	out := COVReading{Reading: Score(o)}
	for _, f := range o.Files {
		if f.Complete {
			for _, e := range f.Edges {
				if e.Resolution == deps.ResolvedExternal && covLocal(e.To, inventory) {
					out.ExternalAsLocal++
				}
			}
		}
	}
	out.B1 = out.B1 && out.ExternalAsLocal == 0
	return out
}

func covLocal(id string, inventory map[string]bool) bool {
	if inventory[id] {
		return true
	}
	ecosystem, name, ok := strings.Cut(id, ":")
	if !ok {
		return false
	}
	switch ecosystem {
	case "npm":
		parts := strings.Split(name, "/")
		n := 1
		if strings.HasPrefix(name, "@") {
			n = 2
		}
		return len(parts) >= n && inventory["npm:"+strings.Join(parts[:n], "/")]
	case "python":
		// A local top-level package rules out claiming any of its members external.
		head, _, _ := strings.Cut(name, ".")
		return inventory["python:"+head]
	}
	return false
}

// COVInventory is built from tracked source and manifests, with no reader or
// native output as input. Python names are relative to the oracle's fixed roots;
// namespace prefixes count as local candidates too.
func COVInventory(tree map[string][]byte, roots []string) map[string]bool {
	out := map[string]bool{}
	for file, data := range tree {
		if path.Base(file) == "package.json" {
			var m struct{ Name string }
			if json.Unmarshal(data, &m) == nil && m.Name != "" {
				out["npm:"+m.Name] = true
			}
		}
		if !strings.HasSuffix(file, ".py") {
			continue
		}
		for _, root := range roots {
			rel := file
			if root != "." && root != "" {
				var ok bool
				rel, ok = strings.CutPrefix(file, strings.TrimSuffix(root, "/")+"/")
				if !ok {
					continue
				}
			}
			rel = strings.TrimSuffix(rel, ".py")
			rel = strings.TrimSuffix(rel, "/__init__")
			parts := strings.Split(rel, "/")
			for n := 1; n <= len(parts); n++ {
				out["python:"+strings.Join(parts[:n], ".")] = true
			}
		}
	}
	return out
}

func COVIdentityOnly(f deps.FileDeps) bool {
	if f.Complete || len(f.Unknowns) == 0 {
		return false
	}
	for _, u := range f.Unknowns {
		if u.Code != deps.UnknownExternalUnpinned || u.Raw == "" || u.Raw == "***" {
			return false
		}
		if !slices.ContainsFunc(f.Edges, func(e deps.Edge) bool {
			return e.Raw == u.Raw && e.Line == u.Line && e.Resolution == deps.ResolvedExternal && e.To != "" && e.Version == ""
		}) {
			return false
		}
	}
	return true
}

type COVStep struct {
	Bucket       string
	Unlock       int
	Cumulative   int
	Prerequisite bool
	Enables      []string
	Marginals    map[string]int
}

// COVGreedy uses sorted ties, a fixed 2% bar and at most limit buckets. A
// prerequisite is justified by a remaining blocking set within the same cap.
func COVGreedy(sets map[string][]string, denominator, limit int) []COVStep {
	all := map[string]bool{}
	for _, set := range sets {
		for _, b := range set {
			all[b] = true
		}
	}
	keys := make([]string, 0, len(all))
	for k := range all {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	closed := map[string]bool{}
	unlocked := func() int {
		n := 0
		for _, set := range sets {
			if len(set) > 0 && !slices.ContainsFunc(set, func(b string) bool { return !closed[b] }) {
				n++
			}
		}
		return n
	}
	var out []COVStep
	for len(out) < limit {
		before := unlocked()
		best := ""
		gain := -1
		marginals := map[string]int{}
		var enables []string
		for _, key := range keys {
			if closed[key] {
				continue
			}
			closed[key] = true
			n := unlocked() - before
			marginals[key] = n
			delete(closed, key)
			if n > gain {
				best, gain = key, n
			}
		}
		if best == "" {
			break
		}
		prerequisite := false
		if gain*100 < 2*denominator {
			best = ""

			bundleGain := -1
			candidates := map[string][]string{}
			for _, set := range sets {
				var remaining []string
				for _, b := range set {
					if !closed[b] {
						remaining = append(remaining, b)
					}
				}
				slices.Sort(remaining)
				remaining = slices.Compact(remaining)
				if len(remaining) > 1 && len(remaining) <= limit-len(out) {
					candidates[strings.Join(remaining, "\x00")] = remaining
				}
			}
			order := make([]string, 0, len(candidates))
			for k := range candidates {
				order = append(order, k)
			}
			slices.Sort(order)
			for _, k := range order {
				remaining := candidates[k]
				for _, b := range remaining {
					closed[b] = true
				}
				n := unlocked() - before
				for _, b := range remaining {
					delete(closed, b)
				}
				if n*100 >= 2*denominator && (n > bundleGain || n == bundleGain && len(remaining) < len(enables)) {
					best, bundleGain, enables = remaining[0], n, remaining
				}
			}
			if best == "" {
				break
			}
			closed[best] = true
			gain = unlocked() - before
			delete(closed, best)
			prerequisite = true
		}
		closed[best] = true
		out = append(out, COVStep{Bucket: best, Unlock: gain, Cumulative: unlocked(), Prerequisite: prerequisite, Enables: enables, Marginals: marginals})
	}
	return out
}

type COVClosureCounts struct {
	Selected, Complete, OnlyUnmeasured, OnlyMeasured, MeasuredAndUnmeasured int
	RejectedBlocked                                                         int
}

// COVClosure grades selected roots only. All local edge kinds participate,
// including config-ref. Missing/unread targets are unmeasured blockers.
func COVClosure(selected []string, files map[string]deps.FileDeps) COVClosureCounts {
	return COVClosureRejected(selected, files, nil)
}

func COVClosureRejected(selected []string, files map[string]deps.FileDeps, rejected map[string]bool) COVClosureCounts {
	out := COVClosureCounts{Selected: len(selected)}
	direct := map[string][]string{}
	for file := range files {
		direct[path.Dir(file)] = append(direct[path.Dir(file)], file)
	}
	for _, start := range selected {
		todo := []string{start}
		seen := map[string]bool{}
		measured, unmeasured := false, false
		rejectedBlocked := false
		for len(todo) > 0 {
			file := todo[len(todo)-1]
			todo = todo[:len(todo)-1]
			if seen[file] {
				continue
			}
			seen[file] = true
			f, ok := files[file]
			if rejected[file] {
				rejectedBlocked = true
				switch path.Ext(file) {
				case ".py", ".ts", ".tsx", ".cts", ".mts", ".js", ".jsx", ".cjs", ".mjs":
					measured = true
				default:
					unmeasured = true
				}
			} else {
				if !ok || !f.Measured {
					unmeasured = true
				}
				if ok && f.Measured && !f.Complete {
					measured = true
				}
			}
			for _, e := range f.Edges {
				switch e.Resolution {
				case deps.ResolvedFile:
					todo = append(todo, e.To)
				case deps.ResolvedDirectory:
					todo = append(todo, direct[e.To]...)
				}
			}
		}
		if rejectedBlocked {
			out.RejectedBlocked++
		}
		switch {
		case measured && unmeasured:
			out.MeasuredAndUnmeasured++
		case measured:
			out.OnlyMeasured++
		case unmeasured:
			out.OnlyUnmeasured++
		default:
			out.Complete++
		}
	}
	return out
}

type COVBucket struct {
	Key                                               string
	Code                                              deps.UnknownCode
	Shape                                             string
	Occurrences, Files, OracleFiles, SoleBlockerFiles int
	OracleOccurrences                                 int
	OracleGaps                                        map[string]int
	DecidableBy                                       []deps.DecidableBy
}

type COVDevelopment struct {
	Score        COVReading
	Buckets      []COVBucket
	BlockingSets map[string][]string
	OracleGaps   map[string]int
	Greedy       []COVStep
	IdentityOnly int
}

func COVAnalyze(o Observation, shapes map[string][]string, gaps map[string]string, inventory map[string]bool) COVDevelopment {
	return COVAnalyzeRejected(o, shapes, gaps, inventory, nil)
}

func COVAnalyzeRejected(o Observation, shapes map[string][]string, gaps map[string]string, inventory map[string]bool, rejected map[string]bool) COVDevelopment {
	out := COVDevelopment{Score: COVScore(o, inventory), BlockingSets: map[string][]string{}, OracleGaps: map[string]int{}}
	truth := map[string]int{}
	for _, f := range o.Truth {
		truth[f.Path]++
	}
	buckets := map[string]*COVBucket{}
	for _, f := range o.Files {
		if gap := gaps[f.Path]; gap != "" {
			out.OracleGaps[gap]++
		}
		if rejected[f.Path] {
			out.Score.RejectedInputs++
			if truth[f.Path] == 1 {
				out.Score.RejectedOracleFiles++
			}
			continue
		}
		if COVIdentityOnly(f) {
			out.IdentityOnly++
		}
		set := []string{}
		for i, u := range f.Unknowns {
			shape := "unknown-shape"
			if i < len(shapes[f.Path]) {
				shape = shapes[f.Path][i]
			}
			key := string(u.Code) + "/" + shape
			b := buckets[key]
			if b == nil {
				b = &COVBucket{Key: key, Code: u.Code, Shape: shape, DecidableBy: []deps.DecidableBy{}, OracleGaps: map[string]int{}}
				buckets[key] = b
			}
			b.Occurrences++
			if truth[f.Path] == 1 {
				b.OracleOccurrences++
			}
			b.DecidableBy = append(b.DecidableBy, u.DecidableBy...)
			if !slices.Contains(set, key) {
				set = append(set, key)
				b.Files++
				if gap := gaps[f.Path]; gap != "" {
					b.OracleGaps[gap]++
				}
				if truth[f.Path] == 1 {
					b.OracleFiles++
				}
			}
		}
		slices.Sort(set)
		out.BlockingSets[f.Path] = set
		if len(set) == 1 {
			buckets[set[0]].SoleBlockerFiles++
		}
	}
	keys := make([]string, 0, len(buckets))
	for k := range buckets {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	for _, k := range keys {
		b := buckets[k]
		slices.Sort(b.DecidableBy)
		b.DecidableBy = slices.Compact(b.DecidableBy)
		out.Buckets = append(out.Buckets, *b)
	}
	out.Greedy = COVGreedy(out.BlockingSets, len(o.Files), 5)
	return out
}

// COVSeal constructs an allowlisted wire value. Never serialize the development
// analysis, paths, operands, shapes, closure, subject breakdown or native stats.
func COVSeal(s COVReading) []byte {
	type seal struct {
		Files                 int                      `json:"files"`
		OracleCovered         int                      `json:"oracle_covered"`
		Complete              int                      `json:"complete"`
		B1                    bool                     `json:"b1"`
		B2                    bool                     `json:"b2"`
		B3                    bool                     `json:"b3"`
		Missing               int                      `json:"missing"`
		ExternalAsLocal       int                      `json:"external_as_local"`
		Wrong                 int                      `json:"wrong"`
		B2Inconclusive        int                      `json:"b2_inconclusive"`
		CompleteWithoutOracle int                      `json:"complete_without_oracle"`
		CompleteWithOracle    int                      `json:"complete_with_oracle"`
		UnknownByCode         map[deps.UnknownCode]int `json:"unknown_by_code"`
	}
	b, _ := json.Marshal(seal{s.Files, s.OracleFiles, s.Complete, s.B1, s.B2, s.B3, s.Missing, s.ExternalAsLocal, s.Wrong, s.B2Inconclusive, s.CompleteWithoutOracle, s.CompleteWithOracle, s.UnknownByCode})
	return b
}

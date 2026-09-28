package verify

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/sdd"
)

// taskLineRe matches a Markdown checklist item: "- [ ] title", "* [x] title",
// "- [~] title", "- [-] title". Group 1 is the checkbox char; group 2 is the
// title text.
var taskLineRe = regexp.MustCompile(`^\s*[-*]\s+\[([ xX~\-])\]\s+(.*\S)\s*$`)

// baselineLineRe matches the plan-level "Baseline: <sha>" header line
// (ADR-044 §4) ParseMarkdownPlan reads into Plan.Baseline. Not a checkbox
// row — this line stands on its own, anywhere in the file (conventionally
// near the top), case-insensitive on the label the way a header reads
// naturally either way. Group 1 is the ref/sha itself.
var baselineLineRe = regexp.MustCompile(`(?i)^\s*Baseline:\s*(\S+)\s*$`)

// proofLineRe matches the plan-level "Proof: <value>" header line
// (ADR-049, docs/design/spec-interop.md §4), the same shape as
// baselineLineRe: its own line, case-insensitive on the "Proof:" label,
// one whitespace-free value. Group 1 is the value as written (e.g.
// "certified"); parseProofHeader/ParseMarkdownPlan compare it
// case-insensitively, so casing in the value itself is not normalized
// here.
var proofLineRe = regexp.MustCompile(`(?i)^\s*Proof:\s*(\S+)\s*$`)

// parseProofHeader reports the plan-level "Proof: <value>" header's value
// when line matches that shape exactly (its own line, not inside a
// checkbox row's annotation block). ok is false otherwise, including for
// a checkbox row whose title happens to contain "Proof: certified" —
// taskLineRe's own leading "- [ ]"/"* [x]" shape never matches this
// regexp, so the two never collide in practice, but callers are expected
// to try this only on non-checkbox lines, mirroring how ParseMarkdownPlan
// tries baselineLineRe first.
func parseProofHeader(line string) (string, bool) {
	if m := proofLineRe.FindStringSubmatch(line); m != nil {
		return m[1], true
	}
	return "", false
}

// ParseMarkdownPlan turns a Markdown checklist (the kind people actually write —
// e.g. tmp/TRACKING.md) into a verify.Plan, so verify_plan / gen-status-cue can
// run against a real plan file rather than only structured tasks.
//
// Each checklist line becomes a Task. The title may carry:
//   - bracketed spec IDs ([US-042], [EP-013], …), extracted via sdd; and
//   - an optional annotation block "{symbol: Foo, package: dir/of/foo,
//     reachable_from: Bar, min_tier: verified, skip_test: true,
//     allow_contradiction: false}" that supplies the acceptance contract. A
//     line with no annotation and no spec IDs verifies as "no-contract"
//     (honestly Unknown) rather than a false pass.
//
// The annotation must sit on the checkbox line itself: this parser is
// line-based, so an annotation on a continuation line is silently ignored.
//
// Task IDs are assigned deterministically as "md-001", "md-002", … in document
// order (zero-padded so they sort correctly).
func ParseMarkdownPlan(service string, md []byte) Plan {
	plan := Plan{Service: service}
	n := 0
	for i, line := range strings.Split(string(md), "\n") {
		if plan.Baseline == "" {
			if bm := baselineLineRe.FindStringSubmatch(line); bm != nil {
				plan.Baseline = bm[1]
				continue
			}
		}
		if plan.Proof == "" {
			if pv, ok := parseProofHeader(line); ok {
				plan.Proof = pv
				continue
			}
		}
		m := taskLineRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		check := m[1] // ' ', 'x', 'X', '~', or '-'
		raw := m[2]
		ann, title, suspect := parseAnnotation(raw)
		n++
		t := Task{
			ID:         fmt.Sprintf("md-%03d", n),
			Title:      title,
			SpecIDs:    sdd.ExtractRefs(raw),
			Claimed:    check == "x" || check == "X",
			Skipped:    check == "-",
			InProgress: check == "~",
			Line:       i + 1,
			Weight:     1,

			AnnotationSuspect: suspect,
		}
		if ann != nil {
			if id := ann["id"]; id != "" {
				t.ID = id // author-owned stable ID overrides positional md-NNN
			}
			t.Disposition = ann["disposition"]
			t.Reason = ann["reason"]
			t.Trigger = ann["trigger"]
			t.Expect.Symbol = ann["symbol"]
			t.Expect.Package = ann["package"]
			t.Expect.ReachableFrom = ann["reachable_from"]
			if mt := core.EvidenceTier(ann["min_tier"]); mt.Valid() {
				t.Expect.MinTier = mt
			}
			t.Expect.SkipTest = truthy(ann["skip_test"])
			t.Expect.SkipReason = ann["skip"]
			t.Expect.AllowContradiction = truthy(ann["allow_contradiction"])
			// certified_by (vision 33 / ADR-043 stage 1, TC-9, design doc
			// §4): repeatable — same comma-separated-VALUE convention
			// `needs` already uses, so more than one test is written
			// quoted ("certified_by: "TestA,TestB""; a comma anywhere in
			// an unquoted value would otherwise fragment the annotation
			// block itself). strength is a single optional floor applied
			// to every named test.
			if v, ok := ann["certified_by"]; ok {
				for _, name := range strings.Split(v, ",") {
					if name = strings.TrimSpace(name); name != "" {
						t.Expect.CertifiedBy = append(t.Expect.CertifiedBy, name)
					}
				}
			}
			t.Expect.CertifyStrength = ann["strength"]
			// ADR-030: the lexical oracle keys. `path`/`pattern` mean nothing
			// without `oracle`, so they are read only alongside it — a stray
			// pattern on a symbol row stays inert instead of half-contracting.
			if o := ann["oracle"]; o != "" {
				t.Oracle = o
				t.OraclePath = ann["path"]
				t.OraclePattern = ann["pattern"]
				// ADR-034 decision 2: the ledger oracle's own keys. Read
				// only alongside `oracle` for the same reason path/pattern
				// are — a stray `run_id`/`task`/`arm` on a symbol row
				// stays inert instead of half-contracting.
				t.OracleRunID = ann["run_id"]
				t.OracleTaskID = ann["task"]
				t.OracleArm = ann["arm"]
				t.OracleMinTrials = ann["min_trials"]
				t.OracleCriterion = ann["criterion"]
				t.OracleAcceptVerdict = ann["accept_verdict"]
			}
			if v, ok := ann["ctx"]; ok {
				if v == "" {
					t.CtxMalformed = true
				} else {
					t.Ctx = strings.Fields(v)
				}
			}
			if v, ok := ann["needs"]; ok {
				for _, id := range strings.Split(v, ",") {
					if id = strings.TrimSpace(id); id != "" {
						t.Needs = append(t.Needs, id)
					}
				}
			}
			// ADR-044 (vision 34): the refactor-conservation grammar.
			// gone/tests are single free-text values; retires/
			// surface_change are repeatable via the SAME quote-when-
			// commas convention certified_by and needs already use.
			t.Gone = ann["gone"]
			t.Tests = ann["tests"]
			if v, ok := ann["retires"]; ok {
				for _, key := range strings.Split(v, ",") {
					if key = strings.TrimSpace(key); key != "" {
						t.Retires = append(t.Retires, key)
					}
				}
			}
			if v, ok := ann["surface_change"]; ok {
				for _, key := range strings.Split(v, ",") {
					if key = strings.TrimSpace(key); key != "" {
						t.SurfaceChange = append(t.SurfaceChange, key)
					}
				}
			}
			// CV-27: `gone: pattern` row's optional `paths:` scope. Read
			// unconditionally (harmless on any other row) — the same
			// "meaningless without its partner key stays inert" shape
			// oracle's own path/pattern keys already follow; EvaluateGone
			// refuses it on a non-pattern gone: form.
			if v, ok := ann["paths"]; ok {
				for _, p := range strings.Split(v, ",") {
					if p = strings.TrimSpace(p); p != "" {
						t.GonePaths = append(t.GonePaths, p)
					}
				}
			}
			t.Max, t.BoundsInvalid = parseIntAnnotation(ann, "max", t.BoundsInvalid)
			t.Min, t.BoundsInvalid = parseIntAnnotation(ann, "min", t.BoundsInvalid)
			t.Eq, t.BoundsInvalid = parseIntAnnotation(ann, "eq", t.BoundsInvalid)
			// Spec interop (ADR-049, design §4): weight: and accept:,
			// both non-gating.
			t.Weight, t.WeightInvalid = parseWeight(ann)
			t.Accept = parseAccept(ann)
		}
		plan.Tasks = append(plan.Tasks, t)
	}
	return plan
}

// trailingBraceBlock locates s's trailing "{...}" block — the annotation
// convention every reader/writer in this file shares (parseAnnotation,
// UpsertAnnotationField): earlier braces (code literals like Beat{...},
// prose examples of annotations) stay title text (GI-7), and brace pairing
// is QUOTE-AWARE — a brace inside a double-quoted span is value content,
// never a delimiter. ok is false when s carries no balanced brace pair at
// all.
func trailingBraceBlock(s string) (open, close int, ok bool) {
	type pair struct{ open, close int }
	var (
		pairs   []pair
		stack   []int
		inQuote bool
	)
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"':
			inQuote = !inQuote
		case '{':
			if !inQuote {
				stack = append(stack, i)
			}
		case '}':
			if !inQuote && len(stack) > 0 {
				pairs = append(pairs, pair{open: stack[len(stack)-1], close: i})
				stack = stack[:len(stack)-1]
			}
		}
	}
	if len(pairs) == 0 {
		return 0, 0, false
	}
	// The block whose close brace is last is the trailing annotation.
	best := pairs[0]
	for _, p := range pairs[1:] {
		if p.close > best.close {
			best = p
		}
	}
	return best.open, best.close, true
}

// parseAnnotation splits a "{key: value, …}" block out of s, returning the
// parsed key→value map (nil when there is no block), s with the block
// removed and trimmed, and a non-empty suspect string when a brace block
// LOOKED like an annotation but yielded no keys — the O4 silent-void class,
// surfaced so LintPlan can be loud instead of the row quietly degrading to
// a positional md-NNN id with no contract.
//
// Values may be double-quoted; a quoted value may contain commas AND braces
// — the notation's own vocabulary ("never write {symbol, package} in a
// value") is exactly what authors quote, and it must never terminate the
// block (O4). Unquoted values must not contain commas or braces.
func parseAnnotation(s string) (map[string]string, string, string) {
	i, j, ok := trailingBraceBlock(s)
	if !ok {
		return nil, strings.TrimSpace(s), ""
	}
	block := s[i+1 : j]
	title := strings.TrimSpace(strings.TrimSpace(s[:i]) + " " + strings.TrimSpace(s[j+1:]))
	m := map[string]string{}
	for _, kvPair := range splitPairs(block) {
		kv := strings.SplitN(kvPair, ":", 2)
		if len(kv) != 2 {
			continue
		}
		k := strings.TrimSpace(kv[0])
		v := unquote(strings.TrimSpace(kv[1]))
		if k != "" {
			m[k] = v
		}
	}
	if len(m) == 0 {
		if suspectAnnotation(block) {
			return nil, title, block
		}
		return nil, title, ""
	}
	return m, title, ""
}

// suspectAnnotation reports whether a zero-key brace block was PROBABLY a
// mangled annotation rather than innocent title text (Beat{stuck}): it
// contains a known contract key as a whole word. Loudness over silence for
// exactly these — a lost contract is the gate's biggest honesty risk (O4).
func suspectAnnotation(block string) bool {
	for _, f := range strings.FieldsFunc(block, func(r rune) bool {
		return r == ' ' || r == ',' || r == ':' || r == '\t'
	}) {
		for _, k := range annotationKeys {
			if f == k {
				return true
			}
		}
	}
	return false
}

// annotationKeys is every key the tokenizer recognises as evidence that a
// brace block was MEANT to be an annotation. Plan-grammar keys came first;
// the oracle keys (ADR-026 §3) joined them when intent conditions started
// carrying `{oracle: …}` blocks parsed by this same tokenizer — without
// them a mangled oracle block would yield zero keys, look like prose, and
// vanish silently: the exact O4 class this list exists to catch.
var annotationKeys = []string{
	// plan grammar v2
	"id", "symbol", "package", "skip_test", "skip", "reachable_from",
	"min_tier", "disposition", "reason", "trigger", "allow_contradiction",
	"ctx", "needs",
	// certified_by (vision 33 / ADR-043 stage 1, TC-9)
	"certified_by", "strength",
	// intent-condition oracles (ADR-026)
	"oracle", "paths", "pattern", "min", "max", "eq", "run", "path",
	"service", "present", "after", "plan", "rows", "row", "partial",
	// ledger oracle (ADR-034 decision 2)
	"run_id", "task", "arm", "min_trials",
	// refactor-conservation grammar (ADR-044, vision 34)
	"gone", "tests", "retires", "surface_change",
	// spec interop (ADR-049, docs/design/spec-interop.md §4)
	"weight", "accept",
}

// ParseAnnotation is parseAnnotation's exported face for callers outside
// this package — internal/intent parses `{oracle: …}` blocks on intent
// condition lines with the SAME tokenizer the plan grammar uses, so the two
// grammars can never drift into two quoting rules (O4's lesson).
//
// It returns the key→value map (nil when the line carries no block), the
// line with the block removed, and a non-empty suspect string when a block
// looked like an annotation but yielded no keys.
func ParseAnnotation(s string) (map[string]string, string, string) {
	return parseAnnotation(s)
}

// UpsertAnnotationField sets key to value within line's trailing "{...}"
// annotation block (RC-30, cmd/fathom/threadpromote.go's `thread promote
// --route` — writing a route's pointers onto a plan row's `ctx:` field is
// the first caller, but the operation is grammar-level, not ctx-specific)
// — the SAME quote/brace-aware block trailingBraceBlock/ParseAnnotation
// locate. An existing key's value is replaced IN PLACE; every other
// key/value pair's original text is preserved byte-for-byte (order,
// spacing, quoting) — only the named key's segment changes. A key absent
// from an existing block is appended before the closing brace; a line
// with NO block at all gets one appended: "<line> {key: value}".
//
// value is wrapped in double quotes in the output only when it contains a
// comma or a brace (the grammar's own "must quote" rule) — never
// otherwise, so an ordinary space-separated ctx: pointer list stays
// unquoted like every other row's.
//
// ok is false when line's trailing block cannot be safely edited: a
// brace block that looks like a MANGLED annotation (suspectAnnotation,
// O4) rather than a fresh one, or one that already repeats key more than
// once (ambiguous which occurrence is authoritative) — either way,
// writing into a block the tokenizer itself doesn't trust would stack a
// second corruption on the first, so this refuses rather than guessing.
// Callers must treat ok:false as "do not write this line back".
func UpsertAnnotationField(line, key, value string) (result string, ok bool) {
	encoded := value
	if strings.ContainsAny(value, ",{}") {
		encoded = `"` + value + `"`
	}
	newPair := key + ": " + encoded

	i, j, hasBlock := trailingBraceBlock(line)
	if !hasBlock {
		return strings.TrimRight(line, " \t") + " {" + newPair + "}", true
	}

	block := line[i+1 : j]
	pairs := splitPairs(block)
	matches := 0
	for idx, p := range pairs {
		kv := strings.SplitN(p, ":", 2)
		if len(kv) == 2 && strings.TrimSpace(kv[0]) == key {
			pairs[idx] = " " + newPair
			matches++
		}
	}
	switch {
	case matches > 1:
		return line, false
	case matches == 0:
		m := map[string]string{}
		for _, p := range pairs {
			kv := strings.SplitN(p, ":", 2)
			if len(kv) == 2 && strings.TrimSpace(kv[0]) != "" {
				m[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
			}
		}
		if len(m) == 0 && suspectAnnotation(block) {
			return line, false
		}
		if strings.TrimSpace(block) == "" {
			pairs = []string{" " + newPair}
		} else {
			pairs = append(pairs, " "+newPair)
		}
	}
	return line[:i+1] + strings.Join(pairs, ",") + line[j:], true
}

// splitPairs splits an annotation block on commas, treating commas inside
// double-quoted spans as literal value content.
func splitPairs(block string) []string {
	var (
		out     []string
		start   int
		inQuote bool
	)
	for i := 0; i < len(block); i++ {
		switch block[i] {
		case '"':
			inQuote = !inQuote
		case ',':
			if !inQuote {
				out = append(out, block[start:i])
				start = i + 1
			}
		}
	}
	out = append(out, block[start:])
	return out
}

// unquote strips one pair of surrounding double quotes, if present.
func unquote(v string) string {
	if len(v) >= 2 && v[0] == '"' && v[len(v)-1] == '"' {
		return v[1 : len(v)-1]
	}
	return v
}

// parseIntAnnotation reads key from ann as an optional integer bound
// (ADR-044's max/min/eq): absent stays nil, unchanged; present and
// well-formed returns a fresh *int; present and malformed (not a base-10
// integer) returns nil and appends key to invalid — loud, never a silently
// ignored bound (O4's lesson applied to a numeric key).
func parseIntAnnotation(ann map[string]string, key string, invalid []string) (*int, []string) {
	v, ok := ann[key]
	if !ok {
		return nil, invalid
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil {
		return nil, append(invalid, key)
	}
	return &n, invalid
}

// parseWeight reads the optional `weight:` key (ADR-049, spec-interop design
// §4): a positive base-10 integer, the row's share of a weighted rollup.
// Absent returns (1, ""). Present and not a positive integer returns 1 and
// the quoted raw value, which LintPlan refuses — loud, never a silently
// reweighted row (O4's lesson applied to a numeric key).
func parseWeight(ann map[string]string) (int, string) {
	v, ok := ann["weight"]
	if !ok {
		return 1, ""
	}
	n, err := strconv.Atoi(strings.TrimSpace(v))
	if err != nil || n < 1 {
		return 1, strconv.Quote(v)
	}
	return n, ""
}

// parseAccept reads the optional `accept:` key (ADR-049, spec-interop design
// §4): acceptance text, conventionally quoted because it carries commas.
// Non-gating by construction — nothing in verifyTask reads it.
func parseAccept(ann map[string]string) string {
	return strings.TrimSpace(ann["accept"])
}

func truthy(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "yes", "1":
		return true
	}
	return false
}

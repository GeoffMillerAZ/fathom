package semantic

// rulessource.go — GL-1 (docs/plans/governance-learning.md, design doc §3
// "Rules readers"): fathom reads rules only from CLAUDE.md, AGENTS.md and
// its own corpus today; teams also write rules in .cursor/rules, Copilot
// instructions, Kiro steering, BUGBOT.md and GEMINI.md. RulesSource is the
// port every one of those client readers (GL-2..GL-6) implements to feed
// the SAME compile path CLAUDE.md/AGENTS.md already go through — so every
// compiled rule, whatever client wrote it, still passes I27 (its citation
// sits inside its own section, its scope matches the files it actually
// names, never over-broad): RulesSourceCorpus hands back plain
// core.SourceFile rows, and the caller runs them through the same
// SelectCorpus/SplitMarkdownSections/compile machinery unchanged. This
// file adds no second compile path and no second citation rule.
//
// A source carries a LEVEL (RulesSourceLevel), configured per source id in
// .fathom/config.toml's [rules.sources] table: "off" (never read), "warn"
// (read and compiled, served with a note naming the source — the served
// note itself is the obligations layer's job, not this port's) or "error"
// (read and compiled, additionally eligible for the PreToolUse deny,
// ADR-038 — a compiled claim decides whether an edit VIOLATES it; the
// level only decides whether a violation MAY block, never this port).
// Unset is "warn" (design doc §3's own default); CLAUDE.md/AGENTS.md are
// UNCHANGED by any of this — they are not routed through this port at all
// (design doc §3's table lists them "exists", not a new reader).
//
// Registration mirrors internal/harness's own Codec registry
// (internal/harness/table.go, codec.go): each reader's own file calls
// RegisterRulesSource in an init(), and the registry is read in sorted-ID
// order everywhere, so behavior never depends on init() ordering.

import (
	"sort"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/filter"
)

// RulesSourceLevel is one of "off" / "warn" / "error" (design doc §3).
type RulesSourceLevel string

const (
	RulesLevelOff   RulesSourceLevel = "off"
	RulesLevelWarn  RulesSourceLevel = "warn"
	RulesLevelError RulesSourceLevel = "error"
)

// ParseRulesSourceLevel validates a [rules.sources] config value: "off",
// "warn" or "error", case-sensitive (the CUE schema enum, when this lands
// there, uses the same three literals). An unset value ("") is the
// documented default, "warn" — this is the one place that default is
// applied, so a caller reading raw config never re-decides it. Any other
// string is rejected (ok=false) rather than silently defaulting, so a
// typo'd level in .fathom/config.toml is a load-time error, not a rule
// that quietly stops being read.
func ParseRulesSourceLevel(s string) (RulesSourceLevel, bool) {
	switch RulesSourceLevel(s) {
	case RulesLevelOff, RulesLevelWarn, RulesLevelError:
		return RulesSourceLevel(s), true
	case "":
		return RulesLevelWarn, true
	default:
		return "", false
	}
}

// RulesSource is one client's rules-file reader: Cursor's .cursor/rules,
// Copilot's instructions, Kiro's steering, BUGBOT.md, GEMINI.md (GL-2..
// GL-6), each a small adapter over its own file shape onto the plain
// core.SourceFile the compile path already consumes.
type RulesSource interface {
	// ID names the source for [rules.sources] and for the served note a
	// caller attaches downstream (design doc §3: "served with a note").
	ID() string
	// Detect reports whether this source has anything to read under root
	// — a fast presence check (the client's own rules dir/file exists) so
	// an absent client's reader never turns into a corpus walk.
	Detect(root string) bool
	// Read returns this source's rules files as core.SourceFile. The
	// caller (RulesSourceCorpus) feeds them into the SAME
	// SelectCorpus/SplitMarkdownSections/compile path CLAUDE.md/AGENTS.md
	// already go through — I27 is enforced exactly once, in one place,
	// for every source, never re-implemented per client.
	Read(root string) ([]core.SourceFile, error)
}

// rulesSourceRegistry is package-level, filled by RegisterRulesSource —
// each reader's own file registers itself in an init(). Never mutated
// after startup in production; tests register/unregister a stub directly.
var rulesSourceRegistry = map[string]RulesSource{}

// RegisterRulesSource adds src to the registry under its own ID. A second
// registration of the same ID replaces the first (defensive; every real
// reader registers exactly once) rather than panicking, so a test can
// register a stub under a fixed ID without a fresh process.
func RegisterRulesSource(src RulesSource) {
	rulesSourceRegistry[src.ID()] = src
}

// RegisteredRulesSources returns every registered source's ID, sorted — a
// caller that needs a stable listing (a future `fathom harness rules`, a
// doctor row) never depends on init() order.
func RegisteredRulesSources() []string {
	out := make([]string, 0, len(rulesSourceRegistry))
	for id := range rulesSourceRegistry {
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// RulesSourceCorpus walks the registry in sorted-ID order and returns the
// combined core.SourceFile corpus (sorted by path) from every source whose
// configured level (levels[id]; unset defaults to RulesLevelWarn, same as
// ParseRulesSourceLevel's own default) is not RulesLevelOff and that
// Detects something under root — plus the level actually applied, keyed by
// source ID, so a caller can serve or log it (design doc §3's "served
// with a note", ADR-038's deny eligibility). A source that is off, absent
// (Detect false) or errors on Read contributes nothing and is absent from
// both return values — never a zero-value placeholder standing in for "we
// tried."
func RulesSourceCorpus(root string, levels map[string]RulesSourceLevel) ([]core.SourceFile, map[string]RulesSourceLevel) {
	var out []core.SourceFile
	applied := map[string]RulesSourceLevel{}
	for _, id := range RegisteredRulesSources() {
		lvl, ok := levels[id]
		if !ok {
			lvl = RulesLevelWarn
		}
		if lvl == RulesLevelOff {
			continue
		}
		src := rulesSourceRegistry[id]
		if !src.Detect(root) {
			continue
		}
		files, err := src.Read(root)
		if err != nil || len(files) == 0 {
			continue
		}
		applied[id] = lvl
		out = append(out, files...)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, applied
}

// WithRulesSources is the compile path's entry for the client readers
// (GL-2..GL-6): corpus (the layer's own selection from the index) plus
// every enabled source's files under root, minus any the layer's exclude
// globs drop, deduplicated by path (a file both routes select is compiled
// once) and sorted. root "" (a service with no known directory) returns
// corpus unchanged. levels is the [rules.sources] map (nil: every source at
// its default, warn).
func WithRulesSources(corpus []core.SourceFile, root string, exclude []string, levels map[string]RulesSourceLevel) []core.SourceFile {
	if root == "" {
		return corpus
	}
	extra, _ := RulesSourceCorpus(root, levels)
	if len(extra) == 0 {
		return corpus
	}
	var excl *filter.Matcher
	if len(exclude) > 0 {
		excl = filter.New(exclude)
	}
	seen := make(map[string]bool, len(corpus))
	out := append([]core.SourceFile(nil), corpus...)
	for _, f := range corpus {
		seen[f.Path] = true
	}
	for _, f := range extra {
		if seen[f.Path] || (excl != nil && excl.Match(f.Path)) {
			continue
		}
		seen[f.Path] = true
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

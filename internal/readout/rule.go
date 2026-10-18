package readout

import (
	"fmt"
	"regexp"
	"strings"
)

// Rule is one readout rule, in the user file format (design §7).
type Rule struct {
	ID       string    `toml:"id"`
	Version  int       `toml:"version"`
	About    string    `toml:"about"`
	Match    Match     `toml:"match"`
	When     When      `toml:"when"`
	Mode     string    `toml:"mode"`
	Recovery string    `toml:"recovery"`
	Stages   []Stage   `toml:"stage"`
	Tests    []Fixture `toml:"test"`

	// Source names the file the rule came from ("builtin:<name>" or a path).
	Source string `toml:"-"`
	// loss is Lossiness(Stages), computed at load.
	loss LossClass
}

// Ref is the rule's `id@version`, as the receipt and the hook row carry it.
func (r *Rule) Ref() string { return fmt.Sprintf("%s@%d", r.ID, r.Version) }

// Match is a token matcher over the lexed command, never a regular
// expression over the raw string.
type Match struct {
	Argv0     []string `toml:"argv0"`
	Sub       []string `toml:"sub"`
	FlagsAny  []string `toml:"flags_any"`
	FlagsNone []string `toml:"flags_none"`
	// Compound is "simple" (the default), "display_tail" or "after_quiet".
	Compound string `toml:"compound"`
	// DeclaredTest also accepts any command the caller's test-run matcher
	// (TR-1) says runs a declared test command (Input.DeclaredTest), in
	// place of argv0, sub and flags_any; flags_none and compound still
	// apply. The engine keeps no list of test commands of its own.
	DeclaredTest bool `toml:"declared_test"`
}

// When is the output floor below which the output stays raw.
type When struct {
	BytesGT int `toml:"bytes_gt"`
	LinesGT int `toml:"lines_gt"`
}

// Fixture is one rule test: an input and the readout it must give.
type Fixture struct {
	// Input is the stdout; InputFile names a file beside the rule instead.
	Input     string `toml:"input"`
	InputFile string `toml:"input_file"`
	// Expect is the readout, receipt included; unset for a Trip fixture.
	Expect string `toml:"expect"`
	// Stream is the stream class the fixture's output is read as; "" is the
	// one the rule's match reaches, or diagnostic when it reaches both.
	Stream Stream `toml:"stream"`
	// Trip is true when a protected line would drop, so raw is served.
	Trip bool `toml:"trip"`
}

// Stage is one step of a rule; Kind is from the closed set.
type Stage struct {
	Kind     string   `toml:"kind"`
	Pattern  string   `toml:"pattern"`
	Captures []string `toml:"captures"`
	Key      string   `toml:"key"`
	Keep     []string `toml:"keep"`
	Head     int      `toml:"head"`
	Tail     int      `toml:"tail"`
	PerGroup int      `toml:"per_group"`
	Above    int      `toml:"above"`
	Template string   `toml:"template"`
	Record   string   `toml:"record"`
	Join     string   `toml:"join"`

	re   *regexp.Regexp
	key  *regexp.Regexp
	tmpl []tmplPart
}

// The stage kinds (design §6, the closed set).
const (
	StageANSI      = "ansi"
	StageCollapse  = "collapse"
	StageGroup     = "group"
	StageExtract   = "extract"
	StageJoin      = "join"
	StageFields    = "fields"
	StageKeep      = "keep"
	StageDrop      = "drop"
	StageCap       = "cap"
	StageHunks     = "hunks"
	StageSummarize = "summarize"
	StageOutline   = "outline"
)

// LossClass is how much a stage or rule can lose.
type LossClass int

// The loss classes, weakest first.
const (
	LossNone    LossClass = iota // display-equivalent, invertible or additive
	LossColumns                  // keeps every line, drops columns
	LossLines                    // drops lines
	LossWhole                    // replaces lines with a summary
)

func (c LossClass) String() string {
	switch c {
	case LossNone:
		return "none"
	case LossColumns:
		return "columns"
	case LossLines:
		return "lines"
	case LossWhole:
		return "whole"
	}
	return "unknown"
}

// stageLoss is each stage kind's fixed class. A kind outside it is refused.
var stageLoss = map[string]LossClass{
	StageANSI:      LossNone,
	StageCollapse:  LossNone,
	StageGroup:     LossNone,
	StageExtract:   LossNone,
	StageJoin:      LossNone,
	StageFields:    LossColumns,
	StageKeep:      LossLines,
	StageDrop:      LossLines,
	StageCap:       LossLines,
	StageHunks:     LossLines,
	StageSummarize: LossWhole,
	StageOutline:   LossWhole,
}

// Lossiness is a rule's lossiness computed from its stages: the strongest
// class among them, never declared by an author (RO-6). A stage kind outside
// the closed set reads as LossWhole, the most conservative class.
func Lossiness(stages []Stage) LossClass {
	worst := LossNone
	for _, s := range stages {
		c, ok := stageLoss[s.Kind]
		if !ok {
			c = LossWhole
		}
		if c > worst {
			worst = c
		}
	}
	return worst
}

// The recoveries a lossy rule names (design §6.4).
const (
	RecoveryRepeat    = "repeat"
	RecoverySpan      = "span"
	RecoveryPersisted = "persisted"
	RecoverySaved     = "saved"
)

// joinHeadStamp is the HEAD stamp join: its first line (`HEAD <sha>`)
// stamps a repeat recovery in the receipt, never the body.
const joinHeadStamp = "head_stamp"

// Joins are names from this closed built-in table; the caller computes
// their lines (Input.Joins), so the engine does no I/O.
var builtinJoins = map[string]bool{
	"outline":           true,
	"head_stamp":        true,
	"last_failures":     true,
	"changed_files":     true,
	"edited_in_session": true,
	"guard_for_command": true,
}

// The modes (ADR-071 decision 5).
const (
	ModeOff     = "off"
	ModeObserve = "observe"
	ModeLean    = "lean"
	ModeRich    = "rich"
)

// tmplPart is one piece of a template: literal text, or a field reference
// with an optional value map (`{type:d=/,l=@}`).
type tmplPart struct {
	lit    string
	field  string
	mapped map[string]string // nil: the field's value as is
}

// compileTemplate parses a template's `{field}` and `{field:a=x,b=y}`
// references, each of which must name one of fields.
func compileTemplate(t string, fields map[string]bool) ([]tmplPart, error) {
	var parts []tmplPart
	for t != "" {
		i := strings.IndexByte(t, '{')
		if i < 0 {
			parts = append(parts, tmplPart{lit: t})
			break
		}
		if i > 0 {
			parts = append(parts, tmplPart{lit: t[:i]})
		}
		j := strings.IndexByte(t[i:], '}')
		if j < 0 {
			return nil, fmt.Errorf("template: unclosed `{`")
		}
		ref := t[i+1 : i+j]
		t = t[i+j+1:]
		p := tmplPart{field: ref}
		if k := strings.IndexByte(ref, ':'); k >= 0 {
			p.field = ref[:k]
			p.mapped = map[string]string{}
			for _, kv := range strings.Split(ref[k+1:], ",") {
				eq := strings.IndexByte(kv, '=')
				if eq <= 0 {
					return nil, fmt.Errorf("template: `{%s}` maps need value=text pairs", ref)
				}
				p.mapped[kv[:eq]] = kv[eq+1:]
			}
		}
		if !fields[p.field] {
			return nil, fmt.Errorf("template: `{%s}` names no capture", p.field)
		}
		parts = append(parts, p)
	}
	return parts, nil
}

func renderTemplate(parts []tmplPart, vals map[string]string) string {
	var b strings.Builder
	for _, p := range parts {
		if p.field == "" {
			b.WriteString(p.lit)
			continue
		}
		v := vals[p.field]
		if p.mapped != nil {
			v = p.mapped[v]
		}
		b.WriteString(v)
	}
	return b.String()
}

// captureIndex maps each capture name to its group: a named group of that
// name, else the positional group.
func captureIndex(re *regexp.Regexp, names []string) (map[string]int, error) {
	idx := map[string]int{}
	for i, n := range names {
		g := re.SubexpIndex(n)
		if g < 0 {
			g = i + 1
		}
		if g > re.NumSubexp() {
			return nil, fmt.Errorf("capture %q has no group in the pattern", n)
		}
		idx[n] = g
	}
	return idx, nil
}

// compileStage checks a stage's per-kind parameters and compiles its
// patterns and template.
func compileStage(s *Stage) error {
	if _, ok := stageLoss[s.Kind]; !ok {
		return fmt.Errorf("unknown stage kind %q", s.Kind)
	}
	need := func(ok bool, what string) error {
		if !ok {
			return fmt.Errorf("%s stage needs %s", s.Kind, what)
		}
		return nil
	}
	if s.Pattern != "" {
		re, err := regexp.Compile(s.Pattern)
		if err != nil {
			return fmt.Errorf("%s stage: pattern: %w", s.Kind, err)
		}
		s.re = re
	}
	if s.Key != "" {
		re, err := regexp.Compile(s.Key)
		if err != nil {
			return fmt.Errorf("%s stage: key: %w", s.Kind, err)
		}
		s.key = re
	}
	fields := map[string]bool{}
	if s.re != nil {
		if _, err := captureIndex(s.re, s.Captures); err != nil {
			return fmt.Errorf("%s stage: %w", s.Kind, err)
		}
		for _, n := range s.Captures {
			fields[n] = true
		}
	}
	switch s.Kind {
	case StageANSI, StageCollapse:
		return nil
	case StageGroup:
		return need(s.key != nil, "a key")
	case StageExtract:
		if err := need(s.re != nil, "a pattern"); err != nil {
			return err
		}
		if err := need(s.Record != "", "a record kind"); err != nil {
			return err
		}
		// A template is the receipt's count clause: {count} the records,
		// {groups} the distinct keys among them.
		fields["count"] = true
		if s.key != nil {
			fields["groups"] = true
		}
	case StageJoin:
		if !builtinJoins[s.Join] {
			return fmt.Errorf("join %q is not in the built-in join table", s.Join)
		}
		return nil
	case StageFields:
		if err := need(s.re != nil && len(s.Captures) > 0, "a pattern and captures"); err != nil {
			return err
		}
		if (s.Template == "") == (len(s.Keep) == 0) {
			return fmt.Errorf("fields stage needs exactly one of template or keep")
		}
		for _, k := range s.Keep {
			if !fields[k] {
				return fmt.Errorf("fields stage: keep %q names no capture", k)
			}
		}
	case StageKeep, StageDrop:
		return need(s.re != nil, "a pattern")
	case StageCap:
		if s.PerGroup > 0 {
			return need(s.key != nil, "a key for per_group")
		}
		return need(s.Head > 0 || s.Tail > 0, "head, tail or per_group")
	case StageHunks:
		if err := need(s.Above > 0 && (s.Head > 0 || s.Tail > 0), "above and a head or tail"); err != nil {
			return err
		}
		if err := need(s.Template != "", "a template (the dropped span's note)"); err != nil {
			return err
		}
		for _, f := range []string{"n", "file", "from", "to", "commit"} {
			fields[f] = true
		}
	case StageSummarize:
		if err := need(s.re != nil, "a pattern"); err != nil {
			return err
		}
		if err := need(s.Template != "", "a template"); err != nil {
			return err
		}
		fields["count"] = true
	case StageOutline:
		if s.Head == 0 {
			s.Head = 40
		}
	}
	if s.Template != "" {
		parts, err := compileTemplate(s.Template, fields)
		if err != nil {
			return fmt.Errorf("%s stage: %w", s.Kind, err)
		}
		s.tmpl = parts
	}
	return nil
}

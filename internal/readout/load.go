package readout

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	cueerrors "cuelang.org/go/cue/errors"
	toml "github.com/pelletier/go-toml/v2"
)

// readoutsSchema is byte-identical to schema/readouts/schema.cue, held to it
// by TestReadoutsSchemaMatchesSource.
//
//go:embed readouts_schema.cue
var readoutsSchema []byte

// builtinFS holds the built-in rules: rules/*.toml, in the user format
// (RO-8 to RO-13 add them; rules/README.md is not a rule).
//
//go:embed rules
var builtinFS embed.FS

// The user rule files, under the per-user fathom directory.
const (
	UserFile = "readouts.toml"
	UserDir  = "readouts.d"
)

// RuleSet is the loaded rules: the served ones in load order (a later rule
// replaces an earlier one with the same id, and the last matching rule
// wins at Render), and the refused ones, named.
type RuleSet struct {
	Rules   []*Rule
	Refused []Refusal
	// Repo is the repository's .fathom/readouts.toml, when LoadRepoRules
	// found one (RO-33); nil otherwise.
	Repo *RepoFile
	// Packs is each tech pack that carries readout rules, when
	// LoadPackRules found any (RO-34), in discovery order.
	Packs []PackReadouts
}

// Refusal is a rule, or a whole file, that is not served, and why.
type Refusal struct {
	Source string
	ID     string // "" when the whole file was refused
	Reason string
}

// LoadRules loads the embedded built-ins, then fathomDir/readouts.toml, then
// fathomDir/readouts.d/*.toml in name order (RO-7). Production passes
// fathomhome.FathomHome(); "" loads the built-ins alone. Each file is
// validated against the embedded CUE schema (unknown keys refused) and
// each rule is compiled and held to the contract before it is served: a
// computed lossiness with a recovery when lossy, a protected-line fixture
// for each stream a lossy rule reaches, every fixture passing twice with
// the same bytes, and every lossless stage round-tripping through its
// inverse. A rule that fails is not served and is named in Refused. The
// error is operational only (the embedded rules unreadable).
func LoadRules(fathomDir string) (*RuleSet, error) {
	sub, err := fs.Sub(builtinFS, "rules")
	if err != nil {
		return nil, err
	}
	return loadRules(sub, fathomDir)
}

func loadRules(builtins fs.FS, fathomDir string) (*RuleSet, error) {
	rs := &RuleSet{}
	names, err := fs.Glob(builtins, "*.toml")
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	for _, n := range names {
		data, err := fs.ReadFile(builtins, n)
		if err != nil {
			return nil, err
		}
		rs.addFile("builtin:"+n, data, builtins)
	}
	if fathomDir == "" {
		return rs, nil
	}
	if data, err := os.ReadFile(filepath.Join(fathomDir, UserFile)); err == nil {
		rs.addFile(filepath.Join(fathomDir, UserFile), data, os.DirFS(fathomDir))
	} else if !errors.Is(err, fs.ErrNotExist) {
		rs.Refused = append(rs.Refused, Refusal{Source: filepath.Join(fathomDir, UserFile), Reason: err.Error()})
	}
	dir := filepath.Join(fathomDir, UserDir)
	entries, err := os.ReadDir(dir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		rs.Refused = append(rs.Refused, Refusal{Source: dir, Reason: err.Error()})
	}
	dirFS := os.DirFS(dir)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".toml") {
			continue
		}
		p := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(p)
		if err != nil {
			rs.Refused = append(rs.Refused, Refusal{Source: p, Reason: err.Error()})
			continue
		}
		rs.addFile(p, data, dirFS)
	}
	return rs, nil
}

// ruleFile is a rule file's decoded shape.
type ruleFile struct {
	Schema int    `toml:"schema"`
	Rule   []Rule `toml:"rule"`
}

func (rs *RuleSet) addFile(source string, data []byte, dir fs.FS) {
	rules, err := parseRuleFile(data)
	if err != nil {
		rs.Refused = append(rs.Refused, Refusal{Source: source, Reason: err.Error()})
		return
	}
	seen := map[string]bool{}
	for i := range rules {
		r := &rules[i]
		r.Source = source
		if seen[r.ID] {
			rs.Refused = append(rs.Refused, Refusal{Source: source, ID: r.ID, Reason: "duplicate id in the file"})
			continue
		}
		seen[r.ID] = true
		if err := prepare(r, dir); err != nil {
			rs.Refused = append(rs.Refused, Refusal{Source: source, ID: r.ID, Reason: err.Error()})
			continue
		}
		kept := rs.Rules[:0]
		for _, old := range rs.Rules {
			if old.ID != r.ID {
				kept = append(kept, old)
			}
		}
		rs.Rules = append(kept, r)
	}
}

// parseRuleFile decodes TOML, validates it against the embedded schema and
// decodes it again strictly into rules.
func parseRuleFile(data []byte) ([]Rule, error) {
	var raw map[string]any
	if err := toml.Unmarshal(data, &raw); err != nil {
		var derr *toml.DecodeError
		if errors.As(err, &derr) {
			row, col := derr.Position()
			return nil, fmt.Errorf("parse TOML: line %d, column %d: %w", row, col, err)
		}
		return nil, fmt.Errorf("parse TOML: %w", err)
	}
	if err := validateRuleFile(raw); err != nil {
		return nil, err
	}
	var f ruleFile
	dec := toml.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("decode: %w", err)
	}
	return f.Rule, nil
}

func validateRuleFile(raw map[string]any) error {
	cctx := cuecontext.New()
	schema := cctx.CompileBytes(readoutsSchema)
	if err := schema.Err(); err != nil {
		return fmt.Errorf("compile embedded schema: %w", err)
	}
	def := schema.LookupPath(cue.ParsePath("#File"))
	doc := cctx.Encode(raw)
	if err := doc.Err(); err != nil {
		return fmt.Errorf("encode: %w", err)
	}
	if err := def.Unify(doc).Validate(cue.Concrete(true)); err != nil {
		return schemaError(err)
	}
	return nil
}

// schemaError renders CUE errors once per path, sorted.
func schemaError(err error) error {
	byPath := map[string][]string{}
	for _, e := range cueerrors.Errors(err) {
		k := strings.TrimPrefix(strings.Join(e.Path(), "."), "#File.")
		format, args := e.Msg()
		byPath[k] = append(byPath[k], fmt.Sprintf(format, args...))
	}
	keys := make([]string, 0, len(byPath))
	for k := range byPath {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	lines := make([]string, 0, len(keys))
	for _, k := range keys {
		msgs := byPath[k]
		sort.Strings(msgs)
		lines = append(lines, fmt.Sprintf("schema: %s: %s", k, strings.Join(msgs, "; ")))
	}
	return errors.New(strings.Join(lines, "\n"))
}

// prepare compiles a rule and holds it to the contract (design §6, §8).
func prepare(r *Rule, dir fs.FS) error {
	for i := range r.Stages {
		if err := compileStage(&r.Stages[i]); err != nil {
			return fmt.Errorf("stage %d: %w", i+1, err)
		}
	}
	r.loss = Lossiness(r.Stages)
	if r.loss != LossNone && r.Recovery == "" {
		return fmt.Errorf("lossy (%s) rule names no recovery (repeat, span, persisted or saved)", r.loss)
	}
	if r.Recovery == RecoverySpan {
		for _, a := range r.Match.Argv0 {
			switch a {
			case "cat", "head", "tail", "sed":
			default:
				return fmt.Errorf("recovery span needs a file read, not %q", a)
			}
		}
	}
	streams := ruleStreams(r.Match)
	need := map[Stream]bool{}
	if r.loss != LossNone {
		for _, s := range streams {
			need[s] = true
		}
	}
	for i := range r.Tests {
		f := &r.Tests[i]
		if (f.Input == "") == (f.InputFile == "") {
			return fmt.Errorf("fixture %d: needs exactly one of input and input_file", i+1)
		}
		if f.InputFile != "" {
			if path.Base(f.InputFile) != f.InputFile || f.InputFile == ".." {
				return fmt.Errorf("fixture %d: input_file must be a file beside the rule", i+1)
			}
			b, err := fs.ReadFile(dir, f.InputFile)
			if err != nil {
				return fmt.Errorf("fixture %d: %w", i+1, err)
			}
			f.Input, f.InputFile = string(b), ""
		}
		if f.Stream == "" {
			f.Stream = StreamDiagnostic
			if len(streams) == 1 {
				f.Stream = streams[0]
			}
		}
		reach := false
		for _, s := range streams {
			reach = reach || s == f.Stream
		}
		if !reach {
			return fmt.Errorf("fixture %d: stream %s is not one this rule's match reaches (the content table is fixed in code)", i+1, f.Stream)
		}
		if !f.Trip && f.Expect == "" {
			return fmt.Errorf("fixture %d: needs expect (or trip = true)", i+1)
		}
		got, res, err := runFixture(r, f)
		if err != nil {
			return fmt.Errorf("fixture %d: %w", i+1, err)
		}
		again, _, _ := runFixture(r, f)
		if again != got {
			return fmt.Errorf("fixture %d: two runs gave different bytes", i+1)
		}
		switch {
		case f.Trip && len(res.trips) == 0:
			return fmt.Errorf("fixture %d: expected a protected-line trip, none tripped", i+1)
		case !f.Trip && len(res.trips) > 0:
			return fmt.Errorf("fixture %d: protected line %d (%s) would drop", i+1, res.trips[0].Line, res.trips[0].Class)
		case !f.Trip && got != f.Expect:
			return fmt.Errorf("fixture %d: %s", i+1, firstDifference(got, f.Expect))
		}
		if f.Stream == StreamDiagnostic && f.Trip {
			delete(need, StreamDiagnostic)
		}
		if f.Stream == StreamContent && !f.Trip && res.contentWordLost() {
			delete(need, StreamContent)
		}
	}
	for _, s := range []Stream{StreamContent, StreamDiagnostic} {
		if !need[s] {
			continue
		}
		if s == StreamDiagnostic {
			return fmt.Errorf("lossy rule needs a protected-line fixture for its diagnostic stream: one that trips (trip = true)")
		}
		return fmt.Errorf("lossy rule needs a protected-line fixture for its content stream: an error or path:line line dropped with no trip")
	}
	return nil
}

// runFixture renders one fixture: the readout and its receipt, or on a
// trip the raw input and the trip's receipt. A lossless stage that does not
// round-trip through its inverse fails it.
func runFixture(r *Rule, f *Fixture) (string, result, error) {
	mode := r.Mode
	if mode == "" {
		mode = ModeObserve
	}
	env := runEnv{capChars: DefaultCap, offset: 0}
	res := run(r, f.Input, f.Stream, env)
	if res.invErr != nil {
		return "", res, res.invErr
	}
	if len(res.trips) > 0 {
		return f.Input + newlineIfNeeded(f.Input) + tripReceipt(r, mode, res.trips) + "\n", res, nil
	}
	return res.body() + receipt(r, mode, res, env) + "\n", res, nil
}

// contentWordLost reports whether an error-word or path:line line was not
// kept exact: the content-stream half of the protected-line fixture.
func (r *result) contentWordLost() bool {
	return r.contentLost
}

// firstDifference names the first line where got and want differ.
func firstDifference(got, want string) string {
	g, _ := splitLines(got)
	w, _ := splitLines(want)
	for i := 0; i < len(g) || i < len(w); i++ {
		var gl, wl string
		if i < len(g) {
			gl = g[i]
		}
		if i < len(w) {
			wl = w[i]
		}
		if gl != wl || i >= len(g) || i >= len(w) {
			return fmt.Sprintf("line %d: got %q, want %q", i+1, gl, wl)
		}
	}
	return fmt.Sprintf("got %q, want %q", got, want)
}

package custodian

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
)

// LoadError collects every problem one file had, so its author goes round
// the loop once (the capability loader's discipline, reused verbatim in
// spirit).
type LoadError struct {
	Path  string
	Scope Scope
	Errs  []error
}

func (e *LoadError) Error() string {
	msgs := make([]string, 0, len(e.Errs))
	for _, err := range e.Errs {
		msgs = append(msgs, err.Error())
	}
	return fmt.Sprintf("concerns: %s: %s", e.Path, strings.Join(msgs, "; "))
}

func (e *LoadError) Unwrap() []error { return e.Errs }

// ScopeFile is one scope's parsed concerns.cue: the concerns it writes and
// the shipped packs it enables (CC-12).
type ScopeFile struct {
	Concerns []Concern
	Packs    []string
}

// LoadFile reads one scope's concerns file and returns the concerns it
// writes itself (the packs it enables are LoadScopeFile's; Resolve expands
// them).
func LoadFile(path string, scope Scope) ([]Concern, error) {
	sf, err := LoadScopeFile(path, scope)
	return sf.Concerns, err
}

// LoadScopeFile reads one scope's concerns file: its concerns and the packs
// it enables.
//
// Three states, not two (the O22 lesson): an ABSENT file is zero concerns
// with no error; a present file with neither field (an all-commented
// scaffold) is the same; a present, malformed file is a loud *LoadError
// naming every problem. Nothing is dropped quietly — which is also why an
// unknown TOP-LEVEL field is refused: `pack: ["lefthook"]` for `packs:`
// would otherwise enable nothing and say nothing.
func LoadScopeFile(path string, scope Scope) (ScopeFile, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return ScopeFile{}, nil
	}
	if err != nil {
		return ScopeFile{}, fmt.Errorf("concerns: read %s: %w", path, err)
	}
	doc, errs := parseDocument(data, path, scope, false)
	if len(errs) > 0 {
		return ScopeFile{}, &LoadError{Path: path, Scope: scope, Errs: errs}
	}
	return ScopeFile{Concerns: doc.concerns, Packs: doc.packs}, nil
}

// parsedDocument is one CUE file's content: a scope file (concerns +
// packs) or a pack file (pack metadata + concerns).
type parsedDocument struct {
	concerns []Concern
	packs    []string
	meta     packMeta
	sawMeta  bool
}

// parseDocument decodes a concerns-shaped CUE file. isPack selects the
// pack-file grammar (`pack` + `concerns`) over the scope-file grammar
// (`concerns` + `packs`); every other top-level field is refused by name.
func parseDocument(data []byte, filename string, scope Scope, isPack bool) (parsedDocument, []error) {
	var doc parsedDocument
	cctx := cuecontext.New()
	v := cctx.CompileBytes(data, cue.Filename(filename))
	if v.Err() != nil {
		return doc, []error{v.Err()}
	}
	top, err := v.Fields(cue.Optional(true))
	if err != nil {
		return doc, []error{fmt.Errorf("concerns: %w", err)}
	}
	known := "concerns, packs"
	if isPack {
		known = "pack, concerns"
	}
	var errs []error
	for top.Next() {
		name := top.Selector().Unquoted()
		if top.IsOptional() {
			errs = append(errs, fmt.Errorf("top-level %q is declared OPTIONAL (%s?:) — in a configuration file that declares the field may exist rather than setting it, so it has no effect; write %q: without the question mark", name, name, name))
			continue
		}
		switch {
		case name == "concerns":
			cs, cerrs := parseConcerns(top.Value(), scope)
			errs = append(errs, cerrs...)
			doc.concerns = cs
		case name == "packs" && !isPack:
			ps, perrs := parsePackList(top.Value())
			errs = append(errs, perrs...)
			doc.packs = ps
		case name == "pack" && isPack:
			m, merrs := parsePackMeta(top.Value())
			errs = append(errs, merrs...)
			doc.meta, doc.sawMeta = m, true
		default:
			errs = append(errs, fmt.Errorf("unknown top-level field %q (known: %s)", name, known))
		}
	}
	if len(errs) > 0 {
		sort.SliceStable(errs, func(i, j int) bool { return errs[i].Error() < errs[j].Error() })
		return parsedDocument{}, errs
	}
	return doc, nil
}

// parseConcerns decodes the `concerns` struct, sorted by ID.
func parseConcerns(root cue.Value, scope Scope) ([]Concern, []error) {
	fields, ferr := root.Fields(cue.Optional(true))
	if ferr != nil {
		return nil, []error{fmt.Errorf("concerns: %w", ferr)}
	}
	var (
		out  []Concern
		errs []error
	)
	for fields.Next() {
		id := fields.Selector().Unquoted()
		if fields.IsOptional() {
			errs = append(errs, fmt.Errorf("concern %q is declared OPTIONAL (%s?:) — write it without the question mark", id, id))
			continue
		}
		c, centErrs := parseConcern(id, fields.Value(), scope)
		if len(centErrs) > 0 {
			errs = append(errs, centErrs...)
			continue
		}
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, errs
}

// parsePackList decodes `packs: ["lefthook", …]`: every name must be a
// shipped pack (refused by name with the list — an unknown pack would
// otherwise enable nothing, silently), duplicates collapse, output sorted.
func parsePackList(v cue.Value) ([]string, []error) {
	list, err := v.List()
	if err != nil {
		return nil, []error{fmt.Errorf("packs must be a list of pack names (known packs: %s): %w", strings.Join(KnownPacks(), ", "), err)}
	}
	seen := map[string]bool{}
	var (
		out  []string
		errs []error
	)
	for list.Next() {
		name, err := list.Value().String()
		if err != nil {
			errs = append(errs, fmt.Errorf("packs: every entry must be a pack name string: %w", err))
			continue
		}
		if !isKnownPack(name) {
			errs = append(errs, fmt.Errorf("packs: unknown pack %q (known packs: %s)", name, strings.Join(KnownPacks(), ", ")))
			continue
		}
		if !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out, errs
}

// parseConcern walks one entry's fields WITH optionals visible, so a
// `want?:` — which in a configuration instance declares the field may
// exist rather than setting it — is refused instead of read past (the
// phase-O trust-boundary inversion, same refusal as the capability
// loader).
func parseConcern(id string, entry cue.Value, scope Scope) (Concern, []error) {
	c := Concern{ID: id, Scope: scope}
	var errs []error

	sub, serr := entry.Fields(cue.Optional(true))
	if serr != nil {
		return c, []error{fmt.Errorf("concern %q: %w", id, serr)}
	}
	sawCheck := false
	for sub.Next() {
		name := sub.Selector().Unquoted()
		if sub.IsOptional() {
			errs = append(errs, fmt.Errorf("concern %q declares %q as OPTIONAL (%s?:) — in a configuration file that declares the field may exist rather than setting it, so this line has no effect; write %q: without the question mark", id, name, name, name))
			continue
		}
		switch name {
		case "want":
			s, err := sub.Value().String()
			if err != nil {
				errs = append(errs, fmt.Errorf("concern %q: want must be a string: %w", id, err))
				continue
			}
			c.Want = s
		case "why":
			s, err := sub.Value().String()
			if err != nil {
				errs = append(errs, fmt.Errorf("concern %q: why must be a string: %w", id, err))
				continue
			}
			c.Why = s
		case "off":
			b, err := sub.Value().Bool()
			if err != nil {
				errs = append(errs, fmt.Errorf("concern %q: off must be true or false: %w", id, err))
				continue
			}
			c.Off = b
		case "check":
			sawCheck = true
			ck, ckErrs := parseCheck(id, sub.Value())
			if len(ckErrs) > 0 {
				errs = append(errs, ckErrs...)
				continue
			}
			c.Check = ck
		default:
			errs = append(errs, fmt.Errorf("concern %q: unknown field %q (known: want, why, off, check)", id, name))
		}
	}

	// An `off: true` entry is a disable switch for an inherited concern —
	// want/check are optional there. Everything else must be a complete,
	// falsifiable want.
	if !c.Off {
		if c.Want == "" {
			errs = append(errs, fmt.Errorf("concern %q: want is required — a concern without a stated want is unreviewable", id))
		}
		if !sawCheck {
			errs = append(errs, fmt.Errorf("concern %q: check is required — a concern without a machine-evaluable check is a wish, and the custodian refuses wishes", id))
		}
	}
	if len(errs) > 0 {
		return c, errs
	}
	return c, nil
}

func parseCheck(id string, v cue.Value) (Check, []error) {
	var ck Check
	var errs []error

	sub, serr := v.Fields(cue.Optional(true))
	if serr != nil {
		return ck, []error{fmt.Errorf("concern %q: check: %w", id, serr)}
	}
	var seen []string // the fields this check sets, for the kind-applicability refusal
	for sub.Next() {
		name := sub.Selector().Unquoted()
		if sub.IsOptional() {
			errs = append(errs, fmt.Errorf("concern %q declares check field %q as OPTIONAL (%s?:) — write it without the question mark", id, name, name))
			continue
		}
		if _, ok := checkFieldKinds[name]; ok {
			seen = append(seen, name)
		}
		switch name {
		case "kind":
			s, err := sub.Value().String()
			if err != nil {
				errs = append(errs, fmt.Errorf("concern %q: check.kind must be a string: %w", id, err))
				continue
			}
			ck.Kind = CheckKind(s)
		case "pattern":
			s, err := sub.Value().String()
			if err != nil {
				errs = append(errs, fmt.Errorf("concern %q: check.pattern must be a string: %w", id, err))
				continue
			}
			ck.Pattern = s
		case "where":
			s, err := sub.Value().String()
			if err != nil {
				errs = append(errs, fmt.Errorf("concern %q: check.where must be a string: %w", id, err))
				continue
			}
			ck.Where = s
		case "command":
			s, err := sub.Value().String()
			if err != nil {
				errs = append(errs, fmt.Errorf("concern %q: check.command must be a string: %w", id, err))
				continue
			}
			ck.Command = s
		case "file":
			s, err := sub.Value().String()
			if err != nil {
				errs = append(errs, fmt.Errorf("concern %q: check.file must be a string: %w", id, err))
				continue
			}
			ck.File = s
		case "lines":
			n, err := sub.Value().Int64()
			if err != nil {
				errs = append(errs, fmt.Errorf("concern %q: check.lines must be an integer: %w", id, err))
				continue
			}
			ck.Lines = int(n)
		case "bytes":
			n, err := sub.Value().Int64()
			if err != nil {
				errs = append(errs, fmt.Errorf("concern %q: check.bytes must be an integer: %w", id, err))
				continue
			}
			ck.Bytes = int(n)
		case "days":
			n, err := sub.Value().Int64()
			if err != nil {
				errs = append(errs, fmt.Errorf("concern %q: check.days must be an integer: %w", id, err))
				continue
			}
			ck.Days = int(n)
		default:
			errs = append(errs, fmt.Errorf("concern %q: check has unknown field %q", id, name))
		}
	}
	if len(errs) > 0 {
		return ck, errs
	}
	// A field the kind does not read is a wish about behaviour that does
	// not exist — `file:` scoping a required_pattern would load, scan the
	// whole corpus, and say nothing — so it is refused like any other
	// unknown field (the schema stays closed). An unknown kind is
	// validateCheck's to name.
	if isKnownKind(ck.Kind) {
		for _, name := range seen {
			if kinds := checkFieldKinds[name]; !containsKind(kinds, ck.Kind) {
				errs = append(errs, fmt.Errorf("concern %q: check.%s applies to %s only, not %q", id, name, kindNames(kinds), ck.Kind))
			}
		}
	}
	if ck.Kind == KindHandoffCitesInstruments && contains(seen, "days") && ck.Days <= 0 {
		errs = append(errs, fmt.Errorf("concern %q: check.days must be > 0 (omit it for the default of %d)", id, DefaultHandoffDays))
	}
	if len(errs) > 0 {
		return ck, errs
	}
	errs = append(errs, validateCheck(id, &ck)...)
	return ck, errs
}

// validateCheck enforces kind-specific requirements at load, so the audit
// never discovers an unevaluable check at run time. It also defaults and
// bounds `where` against the corpus registry (corpus.go, CC-11): a kind
// that reads a corpus defaults to agent-config and accepts only the
// corpora it can read; a kind that reads none refuses `where` outright —
// naming a corpus that does not exist, or one the kind would ignore, is
// refused rather than silently scanning something else.
func validateCheck(id string, ck *Check) []error {
	var errs []error
	switch ck.Kind {
	case KindForbiddenPattern, KindRequiredPattern:
		if ck.Pattern == "" {
			errs = append(errs, fmt.Errorf("concern %q: %s requires a pattern", id, ck.Kind))
		} else if _, err := regexp.Compile(ck.Pattern); err != nil {
			errs = append(errs, fmt.Errorf("concern %q: pattern does not compile: %v", id, err))
		}
	case KindCommandDocumentedAndExists:
		if strings.TrimSpace(ck.Command) == "" {
			errs = append(errs, fmt.Errorf("concern %q: command_documented_and_exists requires a command", id))
		}
	case KindMaxLines:
		if ck.File == "" {
			errs = append(errs, fmt.Errorf("concern %q: max_lines requires a file", id))
		}
		if ck.Lines <= 0 {
			errs = append(errs, fmt.Errorf("concern %q: max_lines requires lines > 0", id))
		}
	case KindMaxBytes:
		if ck.File == "" {
			errs = append(errs, fmt.Errorf("concern %q: max_bytes requires a file", id))
		}
		if ck.Bytes <= 0 {
			errs = append(errs, fmt.Errorf("concern %q: max_bytes requires bytes > 0", id))
		}
	case KindStanzaPresent:
		if ck.File == "" {
			errs = append(errs, fmt.Errorf("concern %q: stanza_present requires a file", id))
		}
	case KindPointersResolve, KindStanzaToolsServed, KindCommandsResolve:
		// no fields beyond where
	case KindADRReferencesResolve:
		// no fields: the corpus is every doc that cites decisions, and the
		// record set is whichever home the repo actually has
	case KindWiringCurrent:
		// no fields: the evaluator is injected by cmd/fathom (WithWiringDrift)
	case KindHandoffCitesInstruments:
		if ck.Days == 0 {
			ck.Days = DefaultHandoffDays
		}
	case "":
		errs = append(errs, fmt.Errorf("concern %q: check.kind is required (known kinds: %s)", id, kindList()))
		return errs
	default:
		errs = append(errs, fmt.Errorf("concern %q: unknown check kind %q — a concern whose check cannot be stated mechanically is refused (known kinds: %s)", id, ck.Kind, kindList()))
		return errs
	}

	accepted := corporaFor(ck.Kind)
	switch {
	case accepted == nil && ck.Where != "":
		errs = append(errs, fmt.Errorf("concern %q: %s reads no corpus, so check.where %q would be ignored — remove it", id, ck.Kind, ck.Where))
	case accepted == nil:
	case ck.Where == "":
		ck.Where = CorpusAgentConfig
	case !contains(accepted, ck.Where):
		if _, registered := registry[ck.Where]; !registered {
			errs = append(errs, fmt.Errorf("concern %q: unknown corpus %q in check.where (registered corpora: %s)", id, ck.Where, strings.Join(KnownCorpora(), ", ")))
		} else {
			errs = append(errs, fmt.Errorf("concern %q: %s reads the %s corpus only, not %q", id, ck.Kind, strings.Join(accepted, ", "), ck.Where))
		}
	}
	return errs
}

// checkFieldKinds names, per check field, the kinds that read it; a field
// set on any other kind is refused at load. `where` is bounded separately
// (validateCheck, against the corpus registry).
var checkFieldKinds = map[string][]CheckKind{
	"pattern": {KindForbiddenPattern, KindRequiredPattern},
	"command": {KindCommandDocumentedAndExists},
	"file":    {KindMaxLines, KindMaxBytes, KindStanzaPresent},
	"lines":   {KindMaxLines},
	"bytes":   {KindMaxBytes},
	"days":    {KindHandoffCitesInstruments},
}

func isKnownKind(k CheckKind) bool { return containsKind(KnownKinds, k) }

func containsKind(ks []CheckKind, k CheckKind) bool {
	for _, v := range ks {
		if v == k {
			return true
		}
	}
	return false
}

func kindNames(ks []CheckKind) string {
	parts := make([]string, len(ks))
	for i, k := range ks {
		parts[i] = string(k)
	}
	return strings.Join(parts, ", ")
}

func contains(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func kindList() string {
	parts := make([]string, len(KnownKinds))
	for i, k := range KnownKinds {
		parts[i] = string(k)
	}
	return strings.Join(parts, ", ")
}

// Merge resolves the two scopes: the union by ID, with a project entry
// replacing a same-ID user entry WHOLESALE (law 13 — the repo governs; no
// field-level blending, which would manufacture a concern neither scope
// wrote). Output sorted by ID.
func Merge(user, project []Concern) []Concern {
	byID := make(map[string]Concern, len(user)+len(project))
	for _, c := range user {
		byID[c.ID] = c
	}
	for _, c := range project {
		byID[c.ID] = c
	}
	out := make([]Concern, 0, len(byID))
	for _, c := range byID {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// ExampleTemplate is what `fathom init` scaffolds (CC-10): entirely
// commented, so the file exists, teaches the check kinds, and loads as
// zero concerns (the three-state loader's scaffold path).
func ExampleTemplate() []byte {
	return []byte(`// .fathom/concerns.cue — the config custodian's contracts for this repo
// (vision 22, "instructed means true"). This file GOVERNS: an entry here
// beats ~/.fathom/concerns.cue (law 13).
//
// A concern is a FALSIFIABLE WANT about this repo's configuration — agent
// config (CLAUDE.md / AGENTS.md), lefthook, Taskfile, CI. Run
// ` + "`fathom config audit`" + ` for evidence;
// fathom reports and never edits your prose — you decide.
//
// Uncomment and adapt. Check kinds: forbidden_pattern, required_pattern,
// command_documented_and_exists, commands_resolve (every task target and
// script a corpus invokes exists), max_lines, max_bytes, pointers_resolve,
// stanza_present, stanza_tools_served (the fathom stanza names only tools
// the daemon serves), handoff_cites_instruments (a handoff note claiming
// shipped work cites a commit / plan row / verify run / retro; days: 7).
// Corpora for where: agent-config (default), lefthook, taskfile, ci.
//
// Packs: curated concern sets fathom ships — claude-code, codex, lefthook,
// taskfile. Enable them, then override any entry by writing its id here
// (replaced wholesale) or disable it with <id>: {off: true}:
//
// packs: ["claude-code", "lefthook", "taskfile"]
//
// concerns: {
// 	gate_documented: {
// 		want:  "the deterministic gate is named in agent config and its target exists"
// 		check: {kind: "command_documented_and_exists", command: "task validate"}
// 	}
// 	router_thin: {
// 		want:  "the root CLAUDE.md stays a router"
// 		check: {kind: "max_lines", file: "CLAUDE.md", lines: 120}
// 	}
// 	pointers_hold: {
// 		want:  "every pointer in agent config resolves"
// 		check: {kind: "pointers_resolve"}
// 	}
// }
`)
}

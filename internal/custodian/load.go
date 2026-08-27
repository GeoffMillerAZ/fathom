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

// LoadFile reads one scope's concerns file.
//
// Three states, not two (the O22 lesson): an ABSENT file is zero concerns
// with no error; a present file whose `concerns` field is absent (an
// all-commented scaffold) is the same; a present, malformed file is a loud
// *LoadError naming every problem. Nothing is dropped quietly.
func LoadFile(path string, scope Scope) ([]Concern, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("concerns: read %s: %w", path, err)
	}

	cctx := cuecontext.New()
	v := cctx.CompileBytes(data, cue.Filename(path))
	if v.Err() != nil {
		return nil, &LoadError{Path: path, Scope: scope, Errs: []error{v.Err()}}
	}

	root := v.LookupPath(cue.ParsePath("concerns"))
	if !root.Exists() {
		return nil, nil // an all-commented scaffold, or a blank file
	}

	fields, ferr := root.Fields(cue.Optional(true))
	if ferr != nil {
		return nil, &LoadError{Path: path, Scope: scope, Errs: []error{fmt.Errorf("concerns: %w", ferr)}}
	}

	var (
		out  []Concern
		errs []error
	)
	for fields.Next() {
		id := fields.Selector().Unquoted()
		c, centErrs := parseConcern(id, fields.Value(), scope)
		if len(centErrs) > 0 {
			errs = append(errs, centErrs...)
			continue
		}
		out = append(out, c)
	}
	if len(errs) > 0 {
		sort.SliceStable(errs, func(i, j int) bool { return errs[i].Error() < errs[j].Error() })
		return nil, &LoadError{Path: path, Scope: scope, Errs: errs}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
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
	sawDays := false
	for sub.Next() {
		name := sub.Selector().Unquoted()
		if sub.IsOptional() {
			errs = append(errs, fmt.Errorf("concern %q declares check field %q as OPTIONAL (%s?:) — write it without the question mark", id, name, name))
			continue
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
		case "days":
			n, err := sub.Value().Int64()
			if err != nil {
				errs = append(errs, fmt.Errorf("concern %q: check.days must be an integer: %w", id, err))
				continue
			}
			sawDays = true
			ck.Days = int(n)
		default:
			errs = append(errs, fmt.Errorf("concern %q: check has unknown field %q", id, name))
		}
	}
	if len(errs) > 0 {
		return ck, errs
	}
	// `days` is the handoff window and nothing else's: a key that no kind
	// reads is a wish about behaviour that does not exist, refused like
	// any other unknown field (the schema stays closed).
	if sawDays {
		if ck.Kind != KindHandoffCitesInstruments {
			errs = append(errs, fmt.Errorf("concern %q: check.days applies to %s only, not %q", id, KindHandoffCitesInstruments, ck.Kind))
		} else if ck.Days <= 0 {
			errs = append(errs, fmt.Errorf("concern %q: check.days must be > 0 (omit it for the default of %d)", id, DefaultHandoffDays))
		}
	}
	if len(errs) > 0 {
		return ck, errs
	}
	errs = append(errs, validateCheck(id, &ck)...)
	return ck, errs
}

// validateCheck enforces kind-specific requirements at load, so the audit
// never discovers an unevaluable check at run time. It also defaults and
// bounds `where`: v1's only corpus is agent-config, and naming a corpus
// that does not exist yet is refused rather than silently scanning
// nothing.
func validateCheck(id string, ck *Check) []error {
	var errs []error
	switch ck.Kind {
	case KindForbiddenPattern:
		if ck.Pattern == "" {
			errs = append(errs, fmt.Errorf("concern %q: forbidden_pattern requires a pattern", id))
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
	case KindPointersResolve:
		// no fields beyond where
	case KindStanzaToolsServed:
		// no fields beyond where; the registry is the daemon's, at audit time
	case KindADRReferencesResolve:
		// no fields: the corpus is every doc that cites decisions, and the
		// record set is whichever home the repo actually has
	case KindHandoffCitesInstruments:
		if ck.Days == 0 {
			ck.Days = DefaultHandoffDays
		}
	case "":
		errs = append(errs, fmt.Errorf("concern %q: check.kind is required (known kinds: %s)", id, kindList()))
	default:
		errs = append(errs, fmt.Errorf("concern %q: unknown check kind %q — a concern whose check cannot be stated mechanically is refused (known kinds: %s)", id, ck.Kind, kindList()))
	}
	switch ck.Kind {
	case KindForbiddenPattern, KindCommandDocumentedAndExists, KindPointersResolve, KindStanzaToolsServed:
		if ck.Where == "" {
			ck.Where = CorpusAgentConfig
		} else if ck.Where != CorpusAgentConfig {
			errs = append(errs, fmt.Errorf("concern %q: unknown corpus %q in check.where — v1 audits %q only (lefthook/taskfile corpora arrive as packs, CC-12)", id, ck.Where, CorpusAgentConfig))
		}
	}
	return errs
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
// A concern is a FALSIFIABLE WANT about this repo's agent config
// (CLAUDE.md / AGENTS.md). Run ` + "`fathom config audit`" + ` for evidence;
// fathom reports and never edits your prose — you decide.
//
// Uncomment and adapt. Check kinds: forbidden_pattern,
// command_documented_and_exists, max_lines, pointers_resolve,
// stanza_tools_served (the fathom stanza names only tools the daemon
// serves), handoff_cites_instruments (a handoff note claiming shipped
// work cites a commit / plan row / verify run / retro; days: 7).
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

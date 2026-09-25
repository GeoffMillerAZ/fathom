// Package styleguide is an experimental app-owned tool, not an application
// runtime dependency. Source CUE and approved font stacks are trusted app inputs;
// HTTP callers may supply only bounded edits, never replacement CUE or CSS.
package styleguide

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/geoffmilleraz/app-kit/appkit/ui/tokens"
)

const (
	maxSource = 64 << 10
	maxColors = 128
	maxEdits  = 2*maxColors + 8
	maxValue  = 1024
	maxBundle = 1 << 20
)

var (
	colorEdit    = regexp.MustCompile(`^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)
	radiusEdit   = regexp.MustCompile(`^([0-9]+(?:\.[0-9]+)?|\.[0-9]+)(px|rem)$`)
	fontID       = regexp.MustCompile(`^[a-z][a-z0-9-]{0,63}$`)
	checkedHex   = regexp.MustCompile(`^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$`)
	checkedOKLCH = regexp.MustCompile(`(?i)^oklch\(\s*(` + colorNumber + `%?)\s+(` + colorNumber + `%?)\s+(` +
		colorNumber + `(?:deg)?)\s*(?:/\s*(` + colorNumber + `%?)\s*)?\)$`)
)

const colorNumber = `[+-]?(?:[0-9]+(?:\.[0-9]*)?|\.[0-9]+)(?:[eE][+-]?[0-9]+)?`

type EngineOptions struct {
	Source                   []byte
	SourceLabel, KitRevision string
	Fonts                    map[string]string
	Checks                   []ContrastCheck
	DarkVariant              tokens.DarkVariant
}

type ContrastCheck struct {
	ID         string  `json:"id"`
	Foreground string  `json:"foreground"`
	Background string  `json:"background"`
	Scheme     string  `json:"scheme"`
	Minimum    float64 `json:"minimum"`
}

type Field struct{ Path, Value, Kind string }

// fieldError associates a rejected override with its control without parsing
// prose or duplicating the engine's validation rules in the HTTP renderer.
type fieldError struct {
	path  string
	cause error
}

func (e *fieldError) Error() string { return fmt.Sprintf("styleguide: %s: %v", e.path, e.cause) }
func (e *fieldError) Unwrap() error { return e.cause }

type Change struct {
	Path   string `json:"path"`
	Before string `json:"before"`
	After  string `json:"after"`
}

type CheckResult struct {
	ID         string  `json:"id"`
	Foreground string  `json:"foreground"`
	Background string  `json:"background"`
	Scheme     string  `json:"scheme"`
	Minimum    float64 `json:"minimum"`
	Ratio      float64 `json:"ratio"`
	Passed     bool    `json:"passed"`
	Error      string  `json:"error,omitempty"`
}

type Candidate struct {
	// CUE is a concrete JSON-as-CUE snapshot, not a patch of the original.
	CUE, CSS     []byte
	SHA256       string
	ReviewSHA256 string
	Changes      []Change
	Checks       []CheckResult
	Exportable   bool
}

type Engine struct {
	source                   []byte
	sourceSHA                string
	sourceLabel, kitRevision string
	baseline                 tokens.Tokens
	fields                   []Field
	fonts                    map[string]string
	checks                   []ContrastCheck
	darkVariant              tokens.DarkVariant
}

func NewEngine(options EngineOptions) (*Engine, error) {
	if len(options.Source) == 0 || len(options.Source) > maxSource {
		return nil, errors.New("styleguide: source must contain 1..65536 bytes")
	}
	if !metadataText(options.SourceLabel, 256) || !metadataText(options.KitRevision, 256) {
		return nil, errors.New("styleguide: bounded source label and kit revision are required")
	}
	variant := options.DarkVariant
	if variant == "" {
		variant = tokens.DarkVariantNone
	}
	if variant != tokens.DarkVariantNone && variant != tokens.DarkVariantAttribute {
		return nil, errors.New("styleguide: unsupported dark variant")
	}
	source := bytes.Clone(options.Source)
	baseline, err := tokens.Load(source)
	if err != nil {
		return nil, errors.New("styleguide: source is not a valid concrete token set")
	}
	if len(baseline.Colors) > maxColors || len(baseline.Name) > 64 || len(baseline.Description) > 4096 {
		return nil, errors.New("styleguide: source token set exceeds the experiment bounds")
	}
	for role := range baseline.Colors {
		if len(role) > 64 {
			return nil, errors.New("styleguide: token role exceeds 64 bytes")
		}
	}
	fields := tokenFields(baseline)
	for _, field := range fields {
		if len(field.Value) > maxValue || !utf8.ValidString(field.Value) {
			return nil, errors.New("styleguide: token value exceeds the experiment bounds")
		}
	}
	if len(options.Fonts) > 32 || len(options.Checks) == 0 || len(options.Checks) > 64 {
		return nil, errors.New("styleguide: at most 32 approved fonts and 1..64 contrast checks required")
	}
	fonts := make(map[string]string, len(options.Fonts))
	for id, stack := range options.Fonts {
		if !fontID.MatchString(id) || !metadataText(stack, maxValue) {
			return nil, errors.New("styleguide: invalid approved font ID or literal stack")
		}
		fonts[id] = stack
	}
	checks := append([]ContrastCheck(nil), options.Checks...)
	seen := make(map[string]bool, len(checks))
	for _, check := range checks {
		_, foreground := baseline.Colors[check.Foreground]
		_, background := baseline.Colors[check.Background]
		if !metadataText(check.ID, 64) || seen[check.ID] || !foreground || !background ||
			(check.Scheme != "light" && check.Scheme != "dark") ||
			!finite(check.Minimum) || check.Minimum < 1 || check.Minimum > 21 {
			return nil, errors.New("styleguide: contrast checks require unique IDs, existing roles, schemes and finite floors in 1..21")
		}
		seen[check.ID] = true
	}
	sort.Slice(checks, func(i, j int) bool { return checks[i].ID < checks[j].ID })
	return &Engine{source: source, sourceSHA: digest(source), sourceLabel: options.SourceLabel,
		kitRevision: options.KitRevision, baseline: baseline, fields: fields, fonts: fonts,
		checks: checks, darkVariant: variant}, nil
}

func (e *Engine) SourceSHA256() string { return e.sourceSHA }
func (e *Engine) SourceLabel() string  { return e.sourceLabel }
func (e *Engine) KitRevision() string  { return e.kitRevision }
func (e *Engine) Fields() []Field      { return append([]Field(nil), e.fields...) }
func (e *Engine) Fonts() map[string]string {
	out := make(map[string]string, len(e.fonts))
	for id, stack := range e.fonts {
		out[id] = stack
	}
	return out
}

// Prepare never mutates the baseline. Omit unchanged edits; unchanged literal
// values are also accepted. Changed fonts must be approved IDs, except an empty
// display value explicitly restores inheritance from sans.
func (e *Engine) Prepare(edits map[string]string) (Candidate, error) {
	if len(edits) > maxEdits {
		return Candidate{}, errors.New("styleguide: too many edits")
	}
	known := make(map[string]Field, len(e.fields))
	for _, field := range e.fields {
		known[field.Path] = field
	}
	for path, value := range edits {
		if _, ok := known[path]; !ok || len(value) > maxValue || !utf8.ValidString(value) {
			return Candidate{}, errors.New("styleguide: unknown field or oversized token edit")
		}
	}
	draft := e.baseline
	draft.Colors = make(map[string]tokens.ColorPair, len(e.baseline.Colors))
	for role, pair := range e.baseline.Colors {
		draft.Colors[role] = pair
	}
	for _, field := range e.fields {
		value, exists := edits[field.Path]
		if !exists || value == field.Value {
			continue
		}
		var err error
		switch field.Kind {
		case "color":
			if !colorEdit.MatchString(value) {
				err = errors.New("use an opaque #rgb or #rrggbb color")
			} else {
				value = strings.ToLower(value)
			}
		case "radius":
			value, err = canonicalRadius(value)
		case "font":
			if value != "" || field.Path != "fonts.display" {
				var ok bool
				value, ok = e.fonts[value]
				if !ok {
					err = errors.New("select an approved font ID")
				}
			}
		}
		if err != nil {
			return Candidate{}, &fieldError{path: field.Path, cause: err}
		}
		setTokenField(&draft, field.Path, value)
	}
	concrete, err := snapshot(draft)
	if err != nil || len(concrete) > 128<<10 {
		return Candidate{}, errors.New("styleguide: candidate snapshot exceeds bounds")
	}
	validated, err := tokens.Load(concrete)
	if err != nil {
		return Candidate{}, errors.New("styleguide: candidate does not satisfy the token schema")
	}
	css := []byte(tokens.EmitThemeCSSOpts(validated, tokens.EmitOptions{DarkVariant: e.darkVariant}))
	if len(css) > 256<<10 {
		return Candidate{}, errors.New("styleguide: candidate CSS exceeds bounds")
	}
	candidate := Candidate{CUE: concrete, CSS: css, SHA256: digest(concrete),
		Changes: []Change{}, Checks: []CheckResult{}, Exportable: true}
	for _, after := range tokenFields(validated) {
		before := known[after.Path]
		if before.Value != after.Value {
			// Empty display is preserved here, not flattened into the sans stack.
			candidate.Changes = append(candidate.Changes, Change{after.Path, before.Value, after.Value})
		}
	}
	for _, check := range e.checks {
		result := CheckResult{ID: check.ID, Foreground: check.Foreground, Background: check.Background,
			Scheme: check.Scheme, Minimum: check.Minimum}
		// The existing contrast parser is permissive. Establish strict selected
		// measurement eligibility first; this is not a whole-source CSS sanitizer.
		measurable := true
		for _, role := range []string{check.Foreground, check.Background} {
			pair := validated.Colors[role]
			value := pair.Light
			if check.Scheme == "dark" {
				value = pair.Dark
			}
			measurable = measurable && checkedColor(value)
		}
		var ratio float64
		var contrastErr error
		if measurable {
			ratio, contrastErr = tokens.ContrastRoles(validated, check.Foreground, check.Background, check.Scheme)
		}
		switch {
		case contrastErr != nil || !measurable:
			result.Error = "Contrast is unknown for these token values."
		case !finite(ratio) || ratio < 1 || ratio > 21:
			result.Error = "Contrast returned a non-finite or out-of-range result."
		default:
			result.Ratio, result.Passed = ratio, ratio >= check.Minimum
		}
		candidate.Checks = append(candidate.Checks, result)
		candidate.Exportable = candidate.Exportable && result.Passed
	}
	binding, err := e.receipt(candidate)
	if err != nil {
		return Candidate{}, err
	}
	candidate.ReviewSHA256 = digest(binding)
	return candidate, nil
}

// Export recomputes the candidate and its complete review binding. The digest
// acknowledges these exact bytes/checks; it does not authenticate human approval.
// The original remains exact, including comments and CUE expressions.
func (e *Engine) Export(edits map[string]string, reviewedSHA256 string) ([]byte, error) {
	candidate, err := e.Prepare(edits)
	if err != nil {
		return nil, err
	}
	if reviewedSHA256 != candidate.ReviewSHA256 || !candidate.Exportable {
		return nil, errors.New("styleguide: export requires the current review digest and all configured checks to pass")
	}
	diff, err := jsonBytes(candidate.Changes)
	if err != nil {
		return nil, err
	}
	receipt, err := e.receipt(candidate)
	if err != nil {
		return nil, err
	}
	files := []struct {
		name string
		body []byte
	}{{"original.cue", e.source}, {"candidate.cue", candidate.CUE}, {"candidate.css", candidate.CSS},
		{"diff.json", diff}, {"receipt.json", receipt}}
	total := 0
	for _, file := range files {
		total += len(file.body)
	}
	if total > maxBundle-4096 {
		return nil, errors.New("styleguide: export bundle exceeds 1 MiB")
	}
	var out bytes.Buffer
	archive := zip.NewWriter(&out)
	for _, file := range files {
		header := &zip.FileHeader{Name: file.name, Method: zip.Store}
		header.SetModTime(time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC))
		header.SetMode(0600)
		writer, err := archive.CreateHeader(header)
		if err != nil {
			return nil, errors.New("styleguide: could not build export archive")
		}
		if _, err := writer.Write(file.body); err != nil {
			return nil, errors.New("styleguide: could not build export archive")
		}
	}
	if err := archive.Close(); err != nil || out.Len() > maxBundle {
		return nil, errors.New("styleguide: could not complete bounded export archive")
	}
	return out.Bytes(), nil
}

type fileIdentity struct {
	SHA256 string `json:"sha256"`
	Size   int    `json:"size"`
}

func (e *Engine) receipt(candidate Candidate) ([]byte, error) {
	diff, err := jsonBytes(candidate.Changes)
	if err != nil {
		return nil, err
	}
	return jsonBytes(struct {
		SchemaVersion int                     `json:"schema_version"`
		Scope         string                  `json:"scope"`
		SourceLabel   string                  `json:"source_label"`
		KitRevision   string                  `json:"kit_revision"`
		DarkVariant   tokens.DarkVariant      `json:"dark_variant"`
		CandidateForm string                  `json:"candidate_format"`
		Files         map[string]fileIdentity `json:"files"`
		Definitions   []ContrastCheck         `json:"check_definitions"`
		Checks        []CheckResult           `json:"checks"`
		Exportable    bool                    `json:"exportable"`
	}{1, "experimental-app-owned-styleguide", e.sourceLabel, e.kitRevision, e.darkVariant,
		"concrete JSON-as-CUE snapshot; not a source patch", map[string]fileIdentity{
			"original.cue": {e.sourceSHA, len(e.source)}, "candidate.cue": {candidate.SHA256, len(candidate.CUE)},
			"candidate.css": {digest(candidate.CSS), len(candidate.CSS)}, "diff.json": {digest(diff), len(diff)},
		}, e.checks, candidate.Checks, candidate.Exportable})
}

func tokenFields(t tokens.Tokens) []Field {
	fields := []Field{{"fonts.sans", t.Fonts.Sans, "font"}, {"fonts.mono", t.Fonts.Mono, "font"},
		{"fonts.display", t.Fonts.Display, "font"}, {"radius.sm", t.Radius.SM, "radius"},
		{"radius.md", t.Radius.MD, "radius"}, {"radius.lg", t.Radius.LG, "radius"},
		{"radius.xl", t.Radius.XL, "radius"}, {"radius.full", t.Radius.Full, "radius"}}
	for role, pair := range t.Colors {
		fields = append(fields, Field{"colors." + role + ".light", pair.Light, "color"},
			Field{"colors." + role + ".dark", pair.Dark, "color"})
	}
	sort.Slice(fields, func(i, j int) bool { return fields[i].Path < fields[j].Path })
	return fields
}

func setTokenField(t *tokens.Tokens, path, value string) {
	parts := strings.Split(path, ".")
	if parts[0] == "colors" {
		pair := t.Colors[parts[1]]
		if parts[2] == "light" {
			pair.Light = value
		} else {
			pair.Dark = value
		}
		t.Colors[parts[1]] = pair
		return
	}
	field := map[string]*string{"fonts.sans": &t.Fonts.Sans, "fonts.mono": &t.Fonts.Mono,
		"fonts.display": &t.Fonts.Display, "radius.sm": &t.Radius.SM, "radius.md": &t.Radius.MD,
		"radius.lg": &t.Radius.LG, "radius.xl": &t.Radius.XL, "radius.full": &t.Radius.Full}[path]
	*field = value
}

func canonicalRadius(value string) (string, error) {
	match := radiusEdit.FindStringSubmatch(value)
	if match != nil {
		number, err := strconv.ParseFloat(match[1], 64)
		limit := 9999.0
		if match[2] == "rem" {
			limit = 100
		}
		if err == nil && finite(number) && number >= 0 && number <= limit {
			return strconv.FormatFloat(number, 'f', -1, 64) + match[2], nil
		}
	}
	return "", errors.New("use a finite nonnegative radius up to 9999px or 100rem")
}

// checkedColor admits only the finite, opaque-capable syntax measured by the
// existing parser. Translucent alpha still fails ContrastRoles because no
// compositing context is declared by this experiment.
func checkedColor(value string) bool {
	if checkedHex.MatchString(value) {
		return true
	}
	parts := checkedOKLCH.FindStringSubmatch(value)
	if parts == nil {
		return false
	}
	for i, part := range parts[1:] {
		if i == 3 && part == "" {
			continue
		}
		part = strings.TrimSuffix(strings.ToLower(part), "deg")
		percentage := strings.HasSuffix(part, "%")
		number, err := strconv.ParseFloat(strings.TrimSuffix(part, "%"), 64)
		if err != nil || !finite(number) {
			return false
		}
		if percentage {
			number /= 100
			if i == 1 {
				number *= 0.4
			}
		}
		if (i == 0 || i == 3) && (number < 0 || number > 1) || i == 1 && number < 0 {
			return false
		}
	}
	return true
}

func snapshot(t tokens.Tokens) ([]byte, error) {
	return jsonBytes(struct {
		Tokens tokens.Tokens `json:"tokens"`
	}{t})
}

func jsonBytes(value any) ([]byte, error) {
	body, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, errors.New("styleguide: could not encode deterministic metadata")
	}
	return append(body, '\n'), nil
}

func metadataText(value string, maximum int) bool {
	return value != "" && len(value) <= maximum && utf8.ValidString(value) &&
		strings.IndexFunc(value, func(r rune) bool { return r < 32 || r == 127 }) < 0
}

func finite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }
func digest(body []byte) string {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

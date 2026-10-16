// Package designadapt builds deliberately source-specific private adaptations.
// It does not broaden the syntax translator or confer target certification.
package designadapt

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designconvert"
	"golang.org/x/net/html"
)

const PricingArchive = "marketing@8863da80a161b3dd0f3ee2991657edbc29855ff8dc23915dda1f520fd6859675"
const PricingMember = "html/sections/pricing/05-single-price-with-details.html"
const PricingSourceSHA = "f545f273689b9a4312e5d0eeb66c15dc21d7d318e102e37c39b40f494e017e0f"

//go:embed pricing.go pricing_model.go.txt pricing.cue pricing_styles.css
var pricingImplementation embed.FS

// PricingArtifact contains licensed derivative material. Keep it private, like
// the source. Defaults deliberately has no usable URL: the app must supply one.
// No field can claim visual, behavioral or target compatibility certification.
type PricingArtifact struct {
	SchemaVersion  int                       `json:"schema_version"`
	Adapter        string                    `json:"adapter"`
	Source         designconvert.Source      `json:"source"`
	Syntax         designconvert.Transformer `json:"syntax"`
	RuleSHA256     string                    `json:"rule_sha256"`
	SyntaxSHA256   string                    `json:"syntax_sha256"`
	TemplSHA256    string                    `json:"templ_sha256"`
	ModelSHA256    string                    `json:"model_sha256"`
	DefaultsSHA256 string                    `json:"defaults_sha256"`
	CSSSHA256      string                    `json:"css_sha256"`
	Templ          string                    `json:"templ"`
	Model          string                    `json:"model"`
	CSS            string                    `json:"css"`
	Defaults       json.RawMessage           `json:"defaults"`
	Verification   string                    `json:"verification"`
}

// AdaptPricing accepts only frozen P02. The caller must first verify the archive
// with designassets.ReadFile. It performs no I/O, compilation or source execution.
func AdaptPricing(source designconvert.Source, body []byte) (PricingArtifact, error) {
	if source.ArchiveID != PricingArchive || source.Path != PricingMember || source.SHA256 != PricingSourceSHA {
		return PricingArtifact{}, fmt.Errorf("pricing adapter: unsupported source identity")
	}
	return adaptPricing(source, body)
}

// VerifyPricingReplay is identity/reproducibility evidence only. The caller must
// independently verify original archive access/membership, just as for generation.
func VerifyPricingReplay(artifact PricingArtifact, body []byte) error {
	if len(artifact.Templ) > 1<<20 || len(artifact.Model) > 32768 || len(artifact.Defaults) > 32768 || len(artifact.CSS) > 16384 {
		return fmt.Errorf("pricing adapter: artifact exceeds replay bounds")
	}
	want, err := AdaptPricing(artifact.Source, body)
	if err != nil {
		return err
	}
	// Compare fields first so attacker-supplied provenance strings cannot force
	// unbounded JSON allocation. Only Defaults needs canonical byte comparison.
	gotDefaults, wantDefaults := artifact.Defaults, want.Defaults
	artifact.Defaults, want.Defaults = nil, nil
	if artifact.SchemaVersion != want.SchemaVersion || artifact.Adapter != want.Adapter || artifact.Source != want.Source || artifact.Syntax != want.Syntax || artifact.RuleSHA256 != want.RuleSHA256 || artifact.SyntaxSHA256 != want.SyntaxSHA256 || artifact.TemplSHA256 != want.TemplSHA256 || artifact.ModelSHA256 != want.ModelSHA256 || artifact.DefaultsSHA256 != want.DefaultsSHA256 || artifact.CSSSHA256 != want.CSSSHA256 || artifact.Templ != want.Templ || artifact.Model != want.Model || artifact.CSS != want.CSS || artifact.Verification != want.Verification || !bytes.Equal(gotDefaults, wantDefaults) {
		return fmt.Errorf("pricing adapter: replay mismatch")
	}
	return nil
}

type pricingToken struct {
	token   html.Token
	mapping designconvert.Mapping
}

func adaptPricing(source designconvert.Source, body []byte) (PricingArtifact, error) {
	base, err := designconvert.Translate(source, body)
	if err != nil || base.Status != "translated" || base.Artifact == nil {
		return PricingArtifact{}, fmt.Errorf("pricing adapter: source did not pass syntax translation")
	}
	// Token offsets and generated ranges come from the unchanged literal rule.
	// Never locate fields by their vendor copy or by unrestricted string replace.
	z := html.NewTokenizer(bytes.NewReader(body))
	var tokens []pricingToken
	for i := 0; ; i++ {
		t := z.Next()
		if t == html.ErrorToken {
			if z.Err() != io.EOF {
				return PricingArtifact{}, fmt.Errorf("pricing adapter: tokenization failed")
			}
			break
		}
		if i >= len(base.Artifact.Mappings) {
			return PricingArtifact{}, fmt.Errorf("pricing adapter: source map mismatch")
		}
		tokens = append(tokens, pricingToken{z.Token(), base.Artifact.Mappings[i]})
	}
	if len(tokens) != len(base.Artifact.Mappings) {
		return PricingArtifact{}, fmt.Errorf("pricing adapter: incomplete source map")
	}

	fields := []string{"Heading", "Introduction", "Plan", "Description", "Included", "Features", "Features", "Features", "Features", "Payment", "Price", "Currency", "Action", "Note"}
	parents := []string{"h2", "p", "h3", "p", "h4", "li", "li", "li", "li", "p", "span", "span", "a", "p"}
	defaults := map[string]any{"URL": ""}
	features := []string{}
	chunks := make([]string, len(tokens))
	var stack []string
	fieldIndex, links := 0, 0
	var starts, ends []int
	for i, item := range tokens {
		t, m := item.token, item.mapping
		chunk := base.Artifact.Content[m.OutputStart:m.OutputEnd]
		switch t.Type {
		case html.StartTagToken:
			stack = append(stack, t.Data)
			if t.Data == "li" {
				starts = append(starts, i)
			}
			if t.Data == "a" {
				links++
				needle := `href={ "#" }`
				if strings.Count(chunk, needle) != 1 {
					return PricingArtifact{}, fmt.Errorf("pricing adapter: expected one placeholder destination")
				}
				chunk = strings.Replace(chunk, needle, `href={ data.URL }`, 1)
			}
		case html.EndTagToken:
			if t.Data == "li" {
				ends = append(ends, i)
			}
			if len(stack) == 0 || stack[len(stack)-1] != t.Data {
				return PricingArtifact{}, fmt.Errorf("pricing adapter: unexpected nesting")
			}
			stack = stack[:len(stack)-1]
		case html.TextToken:
			value := strings.TrimSpace(t.Data)
			if value != "" {
				if fieldIndex >= len(fields) || len(stack) == 0 || stack[len(stack)-1] != parents[fieldIndex] {
					return PricingArtifact{}, fmt.Errorf("pricing adapter: unexpected field structure")
				}
				field := fields[fieldIndex]
				expression := "data." + field
				if field == "Features" {
					expression = "feature"
					features = append(features, value)
				} else {
					defaults[field] = value
				}
				at := strings.Index(t.Data, value)
				// Preserve every original surrounding whitespace byte as text.
				expression = strconv.Quote(t.Data[:at]) + " + " + expression + " + " + strconv.Quote(t.Data[at+len(value):])
				needle := "{ " + strconv.Quote(t.Data) + " }"
				if strings.Count(chunk, needle) != 1 {
					return PricingArtifact{}, fmt.Errorf("pricing adapter: text mapping mismatch")
				}
				chunk = strings.Replace(chunk, needle, "{ "+expression+" }", 1)
				fieldIndex++
			}
		}
		chunks[i] = chunk
	}
	if fieldIndex != len(fields) || links != 1 || len(starts) != 4 || len(ends) != 4 || len(stack) != 0 {
		return PricingArtifact{}, fmt.Errorf("pricing adapter: incomplete pricing structure")
	}
	// Reuse the first source list item's exact markup. Require each other item
	// to have the same token structure, attributes and whitespace around its text.
	for n := 1; n < 4; n++ {
		if !samePricingItem(tokens[starts[0]:ends[0]+1], tokens[starts[n]:ends[n]+1]) {
			return PricingArtifact{}, fmt.Errorf("pricing adapter: feature item variants require review")
		}
		if starts[n] != ends[n-1]+2 || tokens[ends[n-1]+1].token.Type != html.TextToken || strings.TrimSpace(tokens[ends[n-1]+1].token.Data) != "" {
			return PricingArtifact{}, fmt.Errorf("pricing adapter: unexpected feature separator")
		}
		if tokens[ends[n-1]+1].token.Data != tokens[ends[0]+1].token.Data {
			return PricingArtifact{}, fmt.Errorf("pricing adapter: inconsistent feature separator")
		}
	}
	var output strings.Builder
	output.WriteString("package converted\n\ntempl pricing(data Pricing) {\n")
	for i := 0; i < len(chunks); i++ {
		if i == starts[0] {
			output.WriteString("// A typed Go loop preserves the exact source separators.\n@pricingFeatures(data.Features)\n")
			i = ends[3]
		} else {
			output.WriteString(chunks[i])
		}
	}
	output.WriteString("}\n")
	output.WriteString("\ntempl pricingFeature(feature string) {\n")
	for j := starts[0]; j <= ends[0]; j++ {
		output.WriteString(chunks[j])
	}
	output.WriteString("}\n")
	output.WriteString("\ntempl pricingEnhanced(content templ.Component) {\n<div data-appkit-pricing>\n@content\n</div>\n}\n")
	defaults["Features"] = features
	defaultsJSON, err := json.Marshal(defaults)
	if err != nil {
		return PricingArtifact{}, err
	}
	model, err := pricingImplementation.ReadFile("pricing_model.go.txt")
	if err != nil {
		return PricingArtifact{}, err
	}
	rule, err := pricingImplementation.ReadFile("pricing.go")
	if err != nil {
		return PricingArtifact{}, err
	}
	h := sha256.New()
	schema, err := pricingImplementation.ReadFile("pricing.cue")
	if err != nil {
		return PricingArtifact{}, err
	}
	css, err := pricingImplementation.ReadFile("pricing_styles.css")
	if err != nil {
		return PricingArtifact{}, err
	}
	for _, input := range [][]byte{rule, model, schema, css} {
		fmt.Fprintf(h, "%d:", len(input))
		_, _ = h.Write(input)
	}
	// Only source-verified whitespace is written by the Go loop. All configurable
	// feature text still goes through the compiled templ component's escaping.
	model = append(model, []byte("\nconst pricingFeatureSeparator = "+strconv.Quote(tokens[ends[0]+1].token.Data)+"\n")...)
	digest := func(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }
	result := PricingArtifact{SchemaVersion: 1, Adapter: "p02-pricing-v1", Source: source, Syntax: base.Transformer,
		RuleSHA256: hex.EncodeToString(h.Sum(nil)), SyntaxSHA256: base.Transformer.SHA256,
		TemplSHA256: digest([]byte(output.String())), ModelSHA256: digest(model), DefaultsSHA256: digest(defaultsJSON),
		CSSSHA256: digest(css), CSS: string(css), Templ: output.String(), Model: string(model), Defaults: defaultsJSON, Verification: "not-verified"}
	encoded, err := json.Marshal(result)
	if err != nil {
		return PricingArtifact{}, err
	}
	ctx := cuecontext.New()
	if err := ctx.CompileBytes(schema).LookupPath(cue.ParsePath("#PricingArtifact")).Unify(ctx.CompileBytes(encoded)).Validate(cue.Concrete(true)); err != nil {
		return PricingArtifact{}, fmt.Errorf("pricing adapter: generated envelope failed schema")
	}
	return result, nil
}

func samePricingItem(a, b []pricingToken) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		x, y := a[i].token, b[i].token
		if x.Type != y.Type || len(x.Attr) != len(y.Attr) {
			return false
		}
		for j := range x.Attr {
			if x.Attr[j] != y.Attr[j] {
				return false
			}
		}
		if x.Type == html.TextToken && strings.TrimSpace(x.Data) != "" && strings.TrimSpace(y.Data) != "" {
			x.Data = strings.Replace(x.Data, strings.TrimSpace(x.Data), "FIELD", 1)
			y.Data = strings.Replace(y.Data, strings.TrimSpace(y.Data), "FIELD", 1)
		}
		if x.Data != y.Data {
			return false
		}
	}
	return true
}

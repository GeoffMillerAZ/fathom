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
	"github.com/geoffmilleraz/app-kit/internal/designconvert"
	"golang.org/x/net/html"
)

const HeroArchive = "marketing@8863da80a161b3dd0f3ee2991657edbc29855ff8dc23915dda1f520fd6859675"
const HeroMember = "html/sections/heroes/09-split-with-image.html"
const HeroSourceSHA = "356c19f1095d82529cb54014e3b94a67b7531489bb26b96b5fba8686d94beba9"
const heroSourceSize = 2463

//go:embed hero.go hero_model.go.txt hero.cue hero_styles.css
var heroImplementation embed.FS

// HeroSourceAssets preserves original references, not bundled or approved assets.
// Generation and replay never fetch these URLs. The consumer supplies its own
// explicit URLs and remains responsible for image rights and delivery.
type HeroSourceAssets struct {
	LogoLightURL string `json:"logo_light_url"`
	LogoDarkURL  string `json:"logo_dark_url"`
	ImageURL     string `json:"image_url"`
}

// HeroArtifact is a private licensed derivative, not target certification.
// All six configurable URLs in Defaults are empty until the app supplies them.
type HeroArtifact struct {
	SchemaVersion  int                       `json:"schema_version"`
	Adapter        string                    `json:"adapter"`
	Source         designconvert.Source      `json:"source"`
	Syntax         designconvert.Transformer `json:"syntax"`
	SourceAssets   HeroSourceAssets          `json:"source_assets"`
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

// AdaptHero accepts only frozen P01. The caller must independently verify archive
// access and membership using designassets.ReadFile. No I/O or execution occurs.
func AdaptHero(source designconvert.Source, body []byte) (HeroArtifact, error) {
	if source.ArchiveID != HeroArchive || source.Path != HeroMember || source.SHA256 != HeroSourceSHA || source.Size != heroSourceSize {
		return HeroArtifact{}, fmt.Errorf("hero adapter: unsupported source identity")
	}
	return adaptHero(source, body)
}

// VerifyHeroReplay proves only source/rule/toolchain identity and reproducibility.
func VerifyHeroReplay(artifact HeroArtifact, body []byte) error {
	return verifyHeroReplayWith(artifact, body, AdaptHero)
}

func verifyHeroReplayWith(artifact HeroArtifact, body []byte, generate func(designconvert.Source, []byte) (HeroArtifact, error)) error {
	if len(artifact.Templ) > 1<<20 || len(artifact.Model) > 32768 || len(artifact.CSS) > 16384 || len(artifact.Defaults) > 32768 {
		return fmt.Errorf("hero adapter: artifact exceeds replay bounds")
	}
	want, err := generate(artifact.Source, body)
	if err != nil {
		return err
	}
	// Field comparison avoids serializing untrusted provenance before rejection.
	if artifact.SchemaVersion != want.SchemaVersion || artifact.Adapter != want.Adapter || artifact.Source != want.Source || artifact.Syntax != want.Syntax || artifact.SourceAssets != want.SourceAssets || artifact.RuleSHA256 != want.RuleSHA256 || artifact.SyntaxSHA256 != want.SyntaxSHA256 || artifact.TemplSHA256 != want.TemplSHA256 || artifact.ModelSHA256 != want.ModelSHA256 || artifact.DefaultsSHA256 != want.DefaultsSHA256 || artifact.CSSSHA256 != want.CSSSHA256 || artifact.Templ != want.Templ || artifact.Model != want.Model || artifact.CSS != want.CSS || artifact.Verification != want.Verification || !bytes.Equal(artifact.Defaults, want.Defaults) {
		return fmt.Errorf("hero adapter: replay mismatch")
	}
	return nil
}

// This structural signature contains no purchased copy or classes. It deliberately
// refuses other hero arrangements rather than guessing which content means what.
const heroShape = "div div div div img img div div a span /span span /span /a /div /div h1 /h1 p /p div a /a a span /span /a /div /div /div div img /div /div /div"

func adaptHero(source designconvert.Source, body []byte) (HeroArtifact, error) {
	fail := func(reason string) (HeroArtifact, error) {
		return HeroArtifact{}, fmt.Errorf("hero adapter: %s", reason)
	}
	base, err := designconvert.Translate(source, body)
	if err != nil || base.Status != "translated" || base.Artifact == nil {
		return fail("source did not pass syntax translation")
	}
	defaults := map[string]any{
		"LogoLightURL": "", "LogoDarkURL": "", "ImageURL": "", "ImageAlt": "", "ImageDecorative": true,
		"AnnouncementURL": "", "PrimaryURL": "", "SecondaryURL": "",
	}
	var assets HeroSourceAssets
	fields := []string{"Announcement", "AnnouncementLabel", "", "Heading", "Introduction", "PrimaryLabel", "SecondaryLabel", ""}
	parents := []string{"div", "a", "span", "h1", "p", "a", "a", "span"}
	linkFields := []string{"AnnouncementURL", "PrimaryURL", "SecondaryURL"}
	imageFields := []string{"LogoLightURL", "LogoDarkURL", "ImageURL"}
	var stack, shape []string
	var output strings.Builder
	output.WriteString("package converted\n\ntempl hero(data Hero) {\n")
	fieldIndex, links, images, count := 0, 0, 0, 0
	z := html.NewTokenizer(bytes.NewReader(body))
	for ; ; count++ {
		kind := z.Next()
		if kind == html.ErrorToken {
			if z.Err() != io.EOF {
				return fail("tokenization failed")
			}
			break
		}
		if count >= len(base.Artifact.Mappings) {
			return fail("source map mismatch")
		}
		t := z.Token()
		m := base.Artifact.Mappings[count]
		chunk := base.Artifact.Content[m.OutputStart:m.OutputEnd]
		switch kind {
		case html.StartTagToken, html.SelfClosingTagToken:
			shape = append(shape, t.Data)
			if t.Data != "img" {
				stack = append(stack, t.Data)
			}
			if t.Data == "span" {
				if value, ok := heroAttribute(t, "aria-hidden"); !ok || value != "true" {
					return fail("source decorative span contract changed")
				}
			}
			if t.Data == "a" {
				value, ok := heroAttribute(t, "href")
				if !ok || value != "#" || links >= len(linkFields) {
					return fail("expected three placeholder destinations")
				}
				chunk, err = heroBindAttribute(chunk, "href", value, linkFields[links])
				if err != nil {
					return fail("destination mapping mismatch")
				}
				links++
			}
			if t.Data == "img" {
				src, srcOK := heroAttribute(t, "src")
				alt, altOK := heroAttribute(t, "alt")
				if !srcOK || src == "" || !altOK || images >= len(imageFields) {
					return fail("expected three explicit source images")
				}
				altField := "LogoAlt"
				switch images {
				case 0:
					if strings.TrimSpace(alt) == "" {
						return fail("source logo needs an accessible name")
					}
					assets.LogoLightURL, defaults["LogoAlt"] = src, alt
				case 1:
					if alt != defaults["LogoAlt"] {
						return fail("scheme-swapped logo names differ")
					}
					assets.LogoDarkURL = src
				case 2:
					if alt != "" {
						return fail("source image decoration contract changed")
					}
					assets.ImageURL, altField = src, "ImageAlt"
				}
				chunk, err = heroBindAttribute(chunk, "src", src, imageFields[images])
				if err != nil {
					return fail("image source mapping mismatch")
				}
				chunk, err = heroBindAttribute(chunk, "alt", alt, altField)
				if err != nil {
					return fail("image description mapping mismatch")
				}
				images++
			}
		case html.EndTagToken:
			shape = append(shape, "/"+t.Data)
			if len(stack) == 0 || stack[len(stack)-1] != t.Data {
				return fail("unexpected nesting")
			}
			stack = stack[:len(stack)-1]
		case html.TextToken:
			value := strings.TrimSpace(t.Data)
			if value != "" {
				if fieldIndex >= len(fields) || len(stack) == 0 || stack[len(stack)-1] != parents[fieldIndex] {
					return fail("unexpected copy structure")
				}
				field := fields[fieldIndex]
				if field == "" {
					if value != "→" {
						return fail("decorative arrow changed")
					}
				} else {
					defaults[field] = value
					at := strings.Index(t.Data, value)
					expression := strconv.Quote(t.Data[:at]) + " + data." + field + " + " + strconv.Quote(t.Data[at+len(value):])
					needle := "{ " + strconv.Quote(t.Data) + " }"
					if strings.Count(chunk, needle) != 1 {
						return fail("text mapping mismatch")
					}
					chunk = strings.Replace(chunk, needle, "{ "+expression+" }", 1)
				}
				fieldIndex++
			}
		default:
			return fail("unexpected token kind")
		}
		output.WriteString(chunk)
	}
	if count != len(base.Artifact.Mappings) || len(stack) != 0 || strings.Join(shape, " ") != heroShape || fieldIndex != len(fields) || links != 3 || images != 3 {
		return fail("incomplete hero structure")
	}
	output.WriteString("}\n\ntempl heroEnhanced(content templ.Component) {\n<div data-appkit-hero>\n@content\n</div>\n}\n")
	defaultsJSON, err := json.Marshal(defaults)
	if err != nil {
		return fail("defaults encoding failed")
	}
	inputs := make([][]byte, 4)
	h := sha256.New()
	for i, name := range []string{"hero.go", "hero_model.go.txt", "hero.cue", "hero_styles.css"} {
		inputs[i], err = heroImplementation.ReadFile(name)
		if err != nil {
			return fail("implementation unavailable")
		}
		fmt.Fprintf(h, "%d:", len(inputs[i]))
		_, _ = h.Write(inputs[i])
	}
	digest := func(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }
	result := HeroArtifact{SchemaVersion: 1, Adapter: "p01-hero-v1", Source: source, Syntax: base.Transformer, SourceAssets: assets,
		RuleSHA256: hex.EncodeToString(h.Sum(nil)), SyntaxSHA256: base.Transformer.SHA256,
		TemplSHA256: digest([]byte(output.String())), ModelSHA256: digest(inputs[1]), DefaultsSHA256: digest(defaultsJSON), CSSSHA256: digest(inputs[3]),
		Templ: output.String(), Model: string(inputs[1]), CSS: string(inputs[3]), Defaults: defaultsJSON, Verification: "not-verified"}
	encoded, err := json.Marshal(result)
	if err != nil {
		return fail("envelope encoding failed")
	}
	ctx := cuecontext.New()
	if err := ctx.CompileBytes(inputs[2]).LookupPath(cue.ParsePath("#HeroArtifact")).Unify(ctx.CompileBytes(encoded)).Validate(cue.Concrete(true)); err != nil {
		return fail("generated envelope failed schema")
	}
	return result, nil
}

func heroAttribute(t html.Token, key string) (string, bool) {
	for _, attr := range t.Attr {
		if attr.Key == key {
			return attr.Val, true
		}
	}
	return "", false
}

func heroBindAttribute(chunk, key, sourceValue, field string) (string, error) {
	needle := key + "={ " + strconv.Quote(sourceValue) + " }"
	if strings.Count(chunk, needle) != 1 {
		return "", fmt.Errorf("hero adapter: attribute mapping mismatch")
	}
	return strings.Replace(chunk, needle, key+"={ data."+field+" }", 1), nil
}

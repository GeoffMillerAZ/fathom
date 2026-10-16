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

const FormArchive = "application-ui@b26cbf6e9529c2219452de32b1a9c49ca12d7bc63bca5e48cd286088521ba0e8"
const FormMember = "html/forms/input-groups/03-input-with-validation-error.html"
const FormSourceSHA = "c35ba209333b7a4897c93a8c696b7fe306f4797625bc42906090e6edf864cc19"

//go:embed form.go form_model.go.txt form.cue
var formImplementation embed.FS

// FormArtifact is private generation evidence, not validation or certification.
type FormArtifact struct {
	SchemaVersion  int                       `json:"schema_version"`
	Adapter        string                    `json:"adapter"`
	Source         designconvert.Source      `json:"source"`
	Syntax         designconvert.Transformer `json:"syntax"`
	RuleSHA256     string                    `json:"rule_sha256"`
	SyntaxSHA256   string                    `json:"syntax_sha256"`
	TemplSHA256    string                    `json:"templ_sha256"`
	ModelSHA256    string                    `json:"model_sha256"`
	DefaultsSHA256 string                    `json:"defaults_sha256"`
	Templ          string                    `json:"templ"`
	Model          string                    `json:"model"`
	Defaults       json.RawMessage           `json:"defaults"`
	Verification   string                    `json:"verification"`
}

// AdaptForm accepts frozen P05 only. Independently verify archive membership/access
// with designassets.ReadFile first. No I/O or compiler execution occurs here.
func AdaptForm(source designconvert.Source, body []byte) (FormArtifact, error) {
	if source.ArchiveID != FormArchive || source.Path != FormMember || source.SHA256 != FormSourceSHA {
		return FormArtifact{}, fmt.Errorf("form adapter: unsupported source identity")
	}
	return adaptForm(source, body)
}

func VerifyFormReplay(artifact FormArtifact, body []byte) error {
	if len(artifact.Templ) > 1<<20 || len(artifact.Model) > 32768 || len(artifact.Defaults) > 32768 {
		return fmt.Errorf("form adapter: artifact exceeds replay bounds")
	}
	want, err := AdaptForm(artifact.Source, body)
	if err != nil {
		return err
	}
	gotJSON, err := json.Marshal(artifact)
	if err != nil {
		return fmt.Errorf("form adapter: invalid envelope")
	}
	wantJSON, err := json.Marshal(want)
	if err != nil || !bytes.Equal(gotJSON, wantJSON) {
		return fmt.Errorf("form adapter: replay mismatch")
	}
	return nil
}

// Named, reviewed palette changes only; retain other source class bytes.
var formNeutralPalette = [][2]string{
	{"text-red-900", "text-gray-900"}, {"outline-red-300", "outline-gray-300"},
	{"placeholder:text-red-300", "placeholder:text-gray-400"}, {"focus:outline-red-600", "focus:outline-indigo-600"},
	{"dark:text-red-400", "dark:text-white"}, {"dark:outline-red-500/50", "dark:outline-white/10"},
	{"dark:placeholder:text-red-400/70", "dark:placeholder:text-gray-500"}, {"dark:focus:outline-red-400", "dark:focus:outline-indigo-500"},
}

func formNeutralClass(value string) (string, error) {
	out := " " + value + " "
	for _, pair := range formNeutralPalette {
		old := " " + pair[0] + " "
		if strings.Count(out, old) != 1 {
			return "", fmt.Errorf("form adapter: unreviewed input palette")
		}
		out = strings.Replace(out, old, " "+pair[1]+" ", 1)
	}
	if strings.Contains(out, "red-") {
		return "", fmt.Errorf("form adapter: unmapped error palette")
	}
	return out[1 : len(out)-1], nil
}

func adaptForm(source designconvert.Source, body []byte) (FormArtifact, error) {
	fail := func(stage string) (FormArtifact, error) { return FormArtifact{}, fmt.Errorf("form adapter: %s", stage) }
	base, err := designconvert.Translate(source, body)
	if err != nil || base.Status != "translated" || base.Artifact == nil {
		return fail("source did not pass syntax translation")
	}
	z := html.NewTokenizer(bytes.NewReader(body))
	defaults := map[string]any{"Required": false, "Autofocus": false}
	var chunks, shape []string
	starts, ends := map[string]int{}, map[string]int{}
	var current, labelFor, inputID, describedBy, errorID, errorClass, neutralClass string
	for i := 0; ; i++ {
		kind := z.Next()
		if kind == html.ErrorToken {
			if z.Err() != io.EOF {
				return fail("tokenization failed")
			}
			break
		}
		if i >= len(base.Artifact.Mappings) {
			return fail("source map mismatch")
		}
		t := z.Token()
		m := base.Artifact.Mappings[i]
		chunk := base.Artifact.Content[m.OutputStart:m.OutputEnd]
		attrs := map[string]string{}
		for _, attr := range t.Attr {
			attrs[attr.Key] = attr.Val
		}
		bind := func(key, expression string) bool {
			value, ok := attrs[key]
			needle := key + "={ " + strconv.Quote(value) + " }"
			if !ok || strings.Count(chunk, needle) != 1 {
				return false
			}
			chunk = strings.Replace(chunk, needle, key+"={ "+expression+" }", 1)
			return true
		}
		switch kind {
		case html.StartTagToken, html.SelfClosingTagToken:
			shape = append(shape, t.Data)
			current = t.Data
			switch t.Data {
			case "label":
				labelFor = attrs["for"]
				if !bind("for", "data.ID") {
					return fail("label binding")
				}
			case "input":
				if attrs["type"] != "email" || attrs["aria-invalid"] != "true" || len(attrs) != 8 {
					return fail("unexpected input contract")
				}
				inputID, describedBy, errorClass = attrs["id"], attrs["aria-describedby"], attrs["class"]
				neutralClass, err = formNeutralClass(errorClass)
				if err != nil {
					return fail("unreviewed input palette")
				}
				for _, pair := range [][2]string{{"id", "ID"}, {"name", "Name"}, {"value", "Value"}, {"placeholder", "Placeholder"}} {
					defaults[pair[1]] = attrs[pair[0]]
					if !bind(pair[0], "data."+pair[1]) {
						return fail("input binding")
					}
				}
				if !bind("class", `emailFieldClass(data.Error != "")`) {
					return fail("input class binding")
				}
				invalid := `aria-invalid={ "true" }`
				description := "aria-describedby={ " + strconv.Quote(describedBy) + " }"
				if strings.Count(chunk, invalid) != 1 || strings.Count(chunk, description) != 1 {
					return fail("error association")
				}
				chunk = strings.Replace(chunk, invalid, `if data.Error != "" { aria-invalid={ "true" } aria-describedby={ data.ID + "-error" } }`, 1)
				chunk = strings.Replace(chunk, description, `required?={ data.Required } autofocus?={ data.Autofocus }`, 1)
			case "svg":
				if attrs["aria-hidden"] != "true" {
					return fail("icon accessibility")
				}
				starts["svg"] = i
			case "p":
				errorID = attrs["id"]
				if !bind("id", `data.ID + "-error"`) {
					return fail("error ID binding")
				}
				starts["p"] = i
			}
		case html.EndTagToken:
			shape = append(shape, "/"+t.Data)
			current = ""
			if t.Data == "svg" || t.Data == "p" {
				ends[t.Data] = i
			}
		case html.TextToken:
			value := strings.TrimSpace(t.Data)
			if value != "" {
				field := map[string]string{"label": "Label", "p": "Error"}[current]
				if field == "" || defaults[field] != nil {
					return fail("unexpected text binding")
				}
				defaults[field] = value
				at := strings.Index(t.Data, value)
				needle := "{ " + strconv.Quote(t.Data) + " }"
				if strings.Count(chunk, needle) != 1 {
					return fail("text mapping")
				}
				chunk = strings.Replace(chunk, needle, "{ "+strconv.Quote(t.Data[:at])+" + data."+field+" + "+strconv.Quote(t.Data[at+len(value):])+" }", 1)
			}
		}
		chunks = append(chunks, chunk)
	}
	if len(chunks) != len(base.Artifact.Mappings) || strings.Join(shape, " ") != "div label /label div input svg path /svg /div p /p /div" || labelFor == "" || labelFor != inputID || describedBy != inputID+"-error" || errorID != describedBy || defaults["Label"] == nil || defaults["Error"] == nil {
		return fail("unreviewed field structure")
	}
	var outputBuilder strings.Builder
	outputBuilder.WriteString("package converted\n\ntempl emailField(data EmailField) {\n")
	for i := 0; i < len(chunks); i++ {
		switch i {
		case starts["svg"]:
			outputBuilder.WriteString("@emailFieldWhen(data.Error != \"\", emailFieldIcon(data))\n")
			i = ends["svg"]
		case starts["p"]:
			outputBuilder.WriteString("@emailFieldWhen(data.Error != \"\", emailFieldError(data))\n")
			i = ends["p"]
		default:
			outputBuilder.WriteString(chunks[i])
		}
	}
	outputBuilder.WriteString("}\n")
	for _, part := range [][2]string{{"svg", "emailFieldIcon"}, {"p", "emailFieldError"}} {
		outputBuilder.WriteString("\ntempl " + part[1] + "(data EmailField) {\n")
		outputBuilder.WriteString(strings.Join(chunks[starts[part[0]]:ends[part[0]]+1], ""))
		outputBuilder.WriteString("}\n")
	}
	output := outputBuilder.String()
	model, err := formImplementation.ReadFile("form_model.go.txt")
	if err != nil {
		return fail("model input")
	}
	rule, err := formImplementation.ReadFile("form.go")
	if err != nil {
		return fail("rule input")
	}
	schema, err := formImplementation.ReadFile("form.cue")
	if err != nil {
		return fail("schema input")
	}
	h := sha256.New()
	for _, input := range [][]byte{rule, model, schema} {
		fmt.Fprintf(h, "%d:", len(input))
		_, _ = h.Write(input)
	}
	model = append(model, []byte("\nconst emailFieldErrorClass = "+strconv.Quote(errorClass)+"\nconst emailFieldNeutralClass = "+strconv.Quote(neutralClass)+"\n")...)
	defaultsJSON, err := json.Marshal(defaults)
	if err != nil {
		return fail("defaults encoding")
	}
	digest := func(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }
	result := FormArtifact{SchemaVersion: 1, Adapter: "p05-email-field-v1", Source: source, Syntax: base.Transformer,
		RuleSHA256: hex.EncodeToString(h.Sum(nil)), SyntaxSHA256: base.Transformer.SHA256,
		TemplSHA256: digest([]byte(output)), ModelSHA256: digest(model), DefaultsSHA256: digest(defaultsJSON),
		Templ: output, Model: string(model), Defaults: defaultsJSON, Verification: "not-verified"}
	encoded, err := json.Marshal(result)
	if err != nil {
		return fail("artifact encoding")
	}
	ctx := cuecontext.New()
	if err := ctx.CompileBytes(schema).LookupPath(cue.ParsePath("#FormArtifact")).Unify(ctx.CompileBytes(encoded)).Validate(cue.Concrete(true)); err != nil {
		return fail("generated envelope failed schema")
	}
	return result, nil
}

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

const DirectoryArchive = "application-ui@b26cbf6e9529c2219452de32b1a9c49ca12d7bc63bca5e48cd286088521ba0e8"
const DirectoryMember = "html/lists/tables/07-with-stacked-columns-on-mobile.html"
const DirectorySourceSHA = "5d00e2755dcf060cceafa4c2dba4f15452719e54f5cd40646ab3155c89dc8288"

//go:embed directory.go directory_model.go.txt directory.cue
var directoryImplementation embed.FS

// DirectoryArtifact is a private licensed derivative, never a target certificate.
// Defaults contain no usable destinations. The owner must supply app routes.
type DirectoryArtifact struct {
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

// AdaptDirectory accepts only frozen P04. Independently verify archive access
// and membership with designassets.ReadFile first. No I/O or compiler execution.
func AdaptDirectory(source designconvert.Source, body []byte) (DirectoryArtifact, error) {
	if source.ArchiveID != DirectoryArchive || source.Path != DirectoryMember || source.SHA256 != DirectorySourceSHA {
		return DirectoryArtifact{}, fmt.Errorf("directory adapter: unsupported source identity")
	}
	return adaptDirectory(source, body)
}

func VerifyDirectoryReplay(artifact DirectoryArtifact, body []byte) error {
	if len(artifact.Templ) > 1<<20 || len(artifact.Model) > 32768 || len(artifact.Defaults) > 32768 {
		return fmt.Errorf("directory adapter: artifact exceeds replay bounds")
	}
	want, err := AdaptDirectory(artifact.Source, body)
	if err != nil {
		return err
	}
	if artifact.SchemaVersion != want.SchemaVersion || artifact.Adapter != want.Adapter || artifact.Source != want.Source || artifact.Syntax != want.Syntax || artifact.RuleSHA256 != want.RuleSHA256 || artifact.SyntaxSHA256 != want.SyntaxSHA256 || artifact.TemplSHA256 != want.TemplSHA256 || artifact.ModelSHA256 != want.ModelSHA256 || artifact.DefaultsSHA256 != want.DefaultsSHA256 || artifact.Templ != want.Templ || artifact.Model != want.Model || !bytes.Equal(artifact.Defaults, want.Defaults) || artifact.Verification != want.Verification {
		return fmt.Errorf("directory adapter: replay mismatch")
	}
	return nil
}

type directoryToken struct {
	token   html.Token
	mapping designconvert.Mapping
}

func adaptDirectory(source designconvert.Source, body []byte) (DirectoryArtifact, error) {
	fail := func(stage string) (DirectoryArtifact, error) {
		return DirectoryArtifact{}, fmt.Errorf("directory adapter: %s", stage)
	}
	base, err := designconvert.Translate(source, body)
	if err != nil || base.Status != "translated" || base.Artifact == nil {
		return fail("source did not pass syntax translation")
	}
	z := html.NewTokenizer(bytes.NewReader(body))
	var tokens []directoryToken
	for i := 0; ; i++ {
		if z.Next() == html.ErrorToken {
			if z.Err() != io.EOF {
				return fail("tokenization failed")
			}
			break
		}
		if i >= len(base.Artifact.Mappings) {
			return fail("source map mismatch")
		}
		tokens = append(tokens, directoryToken{z.Token(), base.Artifact.Mappings[i]})
	}
	if len(tokens) != len(base.Artifact.Mappings) {
		return fail("incomplete source map")
	}

	// Structure, not vendor wording, locates fields. Repeated representations must
	// agree before they may share a field; a lost mobile value is never normalized.
	outerFields := []string{"Heading", "Introduction", "AddLabel", "NameLabel", "TitleLabel", "EmailLabel", "RoleLabel", "EditLabel"}
	outerParents := []string{"h1", "p", "button", "th", "th", "th", "th", "span"}
	rowFields := []string{"Name", "TitleLabel", "Title", "EmailLabel", "Email", "Title", "Email", "Role", "EditLabel", "Name"}
	rowParents := []string{"td", "dt", "dd", "dt", "dd", "td", "td", "td", "a", "span"}
	defaults := map[string]any{"AddURL": ""}
	var rows []map[string]string
	chunks := make([]string, len(tokens))
	var stack []string
	var starts, ends []int
	outerField, rowField, buttons, links, bodies := 0, 0, 0, 0, 0
	inBody, inRow := false, false
	for i, item := range tokens {
		t, m := item.token, item.mapping
		chunk := base.Artifact.Content[m.OutputStart:m.OutputEnd]
		switch t.Type {
		case html.StartTagToken:
			stack = append(stack, t.Data)
			if t.Data == "tbody" {
				bodies++
				if inBody || bodies != 1 || outerField != len(outerFields) {
					return fail("unexpected body")
				}
				inBody = true
			}
			if t.Data == "tr" && inBody {
				if inRow || len(rows) >= 6 {
					return fail("unexpected row")
				}
				inRow, rowField = true, 0
				starts = append(starts, i)
				rows = append(rows, map[string]string{"EditURL": ""})
			}
			if t.Data == "button" {
				buttons++
				if inBody || buttons != 1 || strings.Count(chunk, `type={ "button" }`) != 1 || strings.Count(chunk, "<button") != 1 {
					return fail("unexpected Add control")
				}
				// Declared semantic mapping: native navigation replaces the inert demo
				// button; classes, label and attribute order otherwise stay unchanged.
				chunk = strings.Replace(chunk, "<button", "<a", 1)
				chunk = strings.Replace(chunk, `type={ "button" }`, `href={ data.AddURL }`, 1)
			}
			if t.Data == "a" {
				links++
				if !inRow || strings.Count(chunk, `href={ "#" }`) != 1 {
					return fail("unexpected Edit destination")
				}
				chunk = strings.Replace(chunk, `href={ "#" }`, `href={ row.EditURL }`, 1)
			}
		case html.EndTagToken:
			if len(stack) == 0 || stack[len(stack)-1] != t.Data {
				return fail("unexpected nesting")
			}
			if t.Data == "button" {
				chunk = strings.Replace(chunk, "</button>", "</a>", 1)
			}
			if t.Data == "tr" && inBody {
				if !inRow || rowField != len(rowFields) {
					return fail("incomplete row fields")
				}
				ends = append(ends, i)
				inRow = false
			}
			if t.Data == "tbody" {
				if inRow {
					return fail("unclosed row")
				}
				inBody = false
			}
			stack = stack[:len(stack)-1]
		case html.TextToken:
			value := strings.TrimSpace(t.Data)
			if value != "" {
				if len(stack) == 0 {
					return fail("unparented field")
				}
				var expression string
				if inRow {
					if rowField >= len(rowFields) || stack[len(stack)-1] != rowParents[rowField] {
						return fail("unexpected row field")
					}
					field := rowFields[rowField]
					expression = "row." + field
					if strings.HasSuffix(field, "Label") {
						if defaults[field] != value {
							return fail("repeated label differs")
						}
						expression = "data." + field
					} else {
						row := rows[len(rows)-1]
						if rowField == 9 {
							if value != ", "+row["Name"] {
								return fail("accessible row name differs")
							}
							expression = `", " + row.Name`
						} else if old, exists := row[field]; exists && old != value {
							return fail("repeated row value differs")
						} else {
							row[field] = value
						}
					}
					rowField++
				} else {
					if inBody || outerField >= len(outerFields) || stack[len(stack)-1] != outerParents[outerField] {
						return fail("unexpected outer field")
					}
					field := outerFields[outerField]
					defaults[field] = value
					expression = "data." + field
					outerField++
				}
				at := strings.Index(t.Data, value)
				expression = strconv.Quote(t.Data[:at]) + " + " + expression + " + " + strconv.Quote(t.Data[at+len(value):])
				needle := "{ " + strconv.Quote(t.Data) + " }"
				if strings.Count(chunk, needle) != 1 {
					return fail("text mapping mismatch")
				}
				chunk = strings.Replace(chunk, needle, "{ "+expression+" }", 1)
			}
		case html.SelfClosingTagToken:
			return fail("unsupported layout extension")
		}
		chunks[i] = chunk
	}
	if len(stack) != 0 || inBody || inRow || bodies != 1 || buttons != 1 || links != 6 || len(starts) != 6 || len(ends) != 6 || outerField != len(outerFields) {
		return fail("incomplete directory structure")
	}
	rowTemplate := strings.Join(chunks[starts[0]:ends[0]+1], "")
	separator := tokens[ends[0]+1].token.Data
	for n := 1; n < 6; n++ {
		if !sameDirectoryMappedRow(tokens[starts[0]:ends[0]+1], chunks[starts[0]:ends[0]+1], tokens[starts[n]:ends[n]+1], chunks[starts[n]:ends[n]+1]) {
			return fail("row variants require review")
		}
		if starts[n] != ends[n-1]+2 || tokens[ends[n-1]+1].token.Type != html.TextToken || strings.TrimSpace(separator) != "" || tokens[ends[n-1]+1].token.Data != separator {
			return fail("inconsistent row separator")
		}
	}
	var output strings.Builder
	output.WriteString("package converted\n\ntempl directoryNavigation(data Directory) {\n")
	for i := 0; i < len(chunks); i++ {
		if i == starts[0] {
			output.WriteString("@directoryRows(data)\n")
			i = ends[5]
		} else {
			output.WriteString(chunks[i])
		}
	}
	output.WriteString("}\n\ntempl directoryRow(data Directory, row DirectoryRow) {\n")
	output.WriteString(rowTemplate)
	output.WriteString("}\n")
	defaults["Rows"] = rows
	defaultsJSON, err := json.Marshal(defaults)
	if err != nil {
		return fail("defaults encoding")
	}
	model, err := directoryImplementation.ReadFile("directory_model.go.txt")
	if err != nil {
		return fail("model input")
	}
	rule, err := directoryImplementation.ReadFile("directory.go")
	if err != nil {
		return fail("rule input")
	}
	schema, err := directoryImplementation.ReadFile("directory.cue")
	if err != nil {
		return fail("schema input")
	}
	h := sha256.New()
	for _, input := range [][]byte{rule, model, schema} {
		fmt.Fprintf(h, "%d:", len(input))
		_, _ = h.Write(input)
	}
	model = append(model, []byte("\nconst directoryRowSeparator = "+strconv.Quote(separator)+"\n")...)
	digest := func(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }
	result := DirectoryArtifact{SchemaVersion: 1, Adapter: "p04-directory-navigation-v1", Source: source, Syntax: base.Transformer,
		RuleSHA256: hex.EncodeToString(h.Sum(nil)), SyntaxSHA256: base.Transformer.SHA256,
		TemplSHA256: digest([]byte(output.String())), ModelSHA256: digest(model), DefaultsSHA256: digest(defaultsJSON),
		Templ: output.String(), Model: string(model), Defaults: defaultsJSON, Verification: "not-verified"}
	encoded, err := json.Marshal(result)
	if err != nil {
		return fail("artifact encoding")
	}
	ctx := cuecontext.New()
	if err := ctx.CompileBytes(schema).LookupPath(cue.ParsePath("#DirectoryArtifact")).Unify(ctx.CompileBytes(encoded)).Validate(cue.Concrete(true)); err != nil {
		return fail("generated envelope failed schema")
	}
	return result, nil
}

// Ignore only the literal translator's non-rendering source-offset annotation
// when checking repeated rows. Keep every emitted attribute/text/whitespace byte.
// The selected row retains its original annotations in the generated template.
func sameDirectoryMappedRow(a []directoryToken, ac []string, b []directoryToken, bc []string) bool {
	if len(a) != len(b) || len(a) != len(ac) || len(b) != len(bc) {
		return false
	}
	shape := func(item directoryToken, chunk string) (string, bool) {
		if item.token.Type == html.EndTagToken {
			return chunk, true
		}
		line, rest, ok := strings.Cut(chunk, "\n")
		if !ok {
			return "", false
		}
		indent := line[:len(line)-len(strings.TrimLeft(line, "\t"))]
		annotation := fmt.Sprintf("// source bytes %d..%d", item.mapping.SourceStart, item.mapping.SourceEnd)
		if line != indent+annotation {
			return "", false
		}
		return indent + rest, true
	}
	for i := range a {
		if a[i].token.Type != b[i].token.Type {
			return false
		}
		x, xok := shape(a[i], ac[i])
		y, yok := shape(b[i], bc[i])
		if !xok || !yok || x != y {
			return false
		}
	}
	return true
}

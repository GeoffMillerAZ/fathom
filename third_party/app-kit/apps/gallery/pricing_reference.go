package gallery

import (
	"crypto/sha256"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strconv"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designattempts"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designconvert"
)

// Only first-party metadata is embedded. All licensed HTML stays in the private
// store. This fixed experiment is not a general uploaded-HTML preview host.
//
//go:embed pricing_reference.cue
var pricingReferenceCUE []byte

const pricingReferenceRoute = "/comparisons/p02-pricing"

type pricingFilePin struct {
	SHA256      string `json:"sha256"`
	Size        int    `json:"size"`
	StyleSHA256 string `json:"style_sha256"`
}

type pricingDescriptor struct {
	ReceiptSHA256 string         `json:"receipt_sha256"`
	ReceiptSize   int            `json:"receipt_size"`
	Receipt       pricingReceipt `json:"receipt"`
}

type pricingReceipt struct {
	SchemaVersion  int                       `json:"schema_version"`
	CaseID         string                    `json:"case_id"`
	Scope          string                    `json:"scope"`
	Source         designconvert.Source      `json:"source"`
	AttemptID      string                    `json:"attempt_id"`
	ArtifactSHA256 string                    `json:"artifact_sha256"`
	Transformer    designconvert.Transformer `json:"transformer"`
	Documents      map[string]pricingFilePin `json:"documents"`
	Verification   string                    `json:"verification"`
	Configuration  pricingConfiguration      `json:"configuration"`
}

type pricingConfiguration struct {
	TailwindVersion  string `json:"tailwind_version"`
	TailwindPlatform string `json:"tailwind_platform"`
	TailwindSHA256   string `json:"tailwind_sha256"`
	Font             struct {
		SHA256 string `json:"sha256"`
		Size   int    `json:"size"`
	} `json:"font"`
	FontFamily         string `json:"font_family"`
	FontManifestSHA256 string `json:"font_manifest_sha256"`
	FontContract       string `json:"font_contract"`
	CSSInputSHA256     string `json:"css_input_sha256"`
	CSSOutputSHA256    string `json:"css_output_sha256"`
	ThemePolicy        string `json:"theme_policy"`
	MaxDocumentBytes   int    `json:"max_document_bytes"`
}

func pricingReferenceDescriptor() (pricingDescriptor, error) {
	ctx := cuecontext.New()
	v := ctx.CompileBytes(pricingReferenceCUE).LookupPath(cue.ParsePath("pricingReference"))
	var d pricingDescriptor
	if err := v.Validate(cue.Concrete(true), cue.Final()); err != nil {
		return d, errors.New("prepared reference metadata is invalid")
	}
	if err := v.Decode(&d); err != nil {
		return d, errors.New("prepared reference metadata could not be decoded")
	}
	return d, nil
}

func registerPricingReference(root string, register func(string, bool, http.HandlerFunc)) {
	d, metadataErr := pricingReferenceDescriptor()
	registerPricingReferenceWith(root, register, d, metadataErr)
}

func registerPricingReferenceWith(root string, register func(string, bool, http.HandlerFunc), d pricingDescriptor, metadataErr error) {
	registerPricingCompareAssets(register)
	problem := func(w http.ResponseWriter, r *http.Request) {
		conversionProblem(w, r, false, http.StatusConflict, "Prepared comparison unavailable", "The private P02 fixture is missing, changed or no longer matches its saved conversion. Prepare and review it explicitly; this page never builds or downloads anything.", "/conversions/"+url.PathEscape(d.Receipt.AttemptID))
	}
	open := func(w http.ResponseWriter, r *http.Request) (*os.Root, bool) {
		if metadataErr != nil {
			problem(w, r)
			return nil, false
		}
		prepared, err := openPricingReference(root, d)
		if err != nil {
			problem(w, r)
			return nil, false
		}
		return prepared, true
	}
	register(pricingReferenceRoute, false, func(w http.ResponseWriter, r *http.Request) {
		values, width, previewTheme, err := pricingReferenceQuery(r)
		if err != nil {
			conversionBadQuery(w, r, false)
			return
		}
		prepared, ok := open(w, r)
		if !ok {
			return
		}
		defer prepared.Close()
		panes := []pricingReferencePane{}
		for _, side := range []string{"original", "converted"} {
			name := side + "-" + previewTheme + ".html"
			pin, exists := d.Receipt.Documents[name]
			if !exists {
				problem(w, r)
				return
			}
			if _, err := readPricingFile(prepared, name, pin, 1<<20); err != nil {
				problem(w, r)
				return
			}
			label := "Original markup"
			if side == "converted" {
				label = "Compiled templ markup"
			}
			panes = append(panes, pricingReferencePane{Side: side, Label: label, URL: pricingReferenceRoute + "/document/" + name, Title: fmt.Sprintf("%s, %s, %d pixel viewport", label, previewTheme, width), Bytes: pin.Size})
		}
		view := pricingReferenceView{Descriptor: d, Width: width, PreviewTheme: previewTheme, Theme: values.Get("theme"), Panes: panes, SourceURL: referenceLink("/conversions/"+url.PathEscape(d.Receipt.AttemptID), referenceThemeValues(values))}
		configurePricingCompare(&view, values)
		body, err := htmlf("pricing-reference", pricingReferenceHTML, view)
		if err != nil {
			problem(w, r)
			return
		}
		// This success response alone admits our same-origin prepared documents.
		// Source routes and all errors retain the enclosing frame-denying policy.
		w.Header().Set("Content-Security-Policy", pricingCompareCSP())
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		_, _ = io.WriteString(w, string(body))
	})
	register(pricingReferenceRoute+"/document/{name}", false, func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		pin, exists := d.Receipt.Documents[name]
		if !exists || r.URL.RawQuery != "" || r.URL.EscapedPath() != pricingReferenceRoute+"/document/"+name {
			conversionBadQuery(w, r, false)
			return
		}
		prepared, ok := open(w, r)
		if !ok {
			return
		}
		defer prepared.Close()
		body, err := readPricingFile(prepared, name, pin, 1<<20)
		styleHash, hashErr := hex.DecodeString(pin.StyleSHA256)
		if err != nil || hashErr != nil || len(styleHash) != sha256.Size {
			problem(w, r)
			return
		}
		w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'none'; style-src 'sha256-"+base64.StdEncoding.EncodeToString(styleHash)+"'; font-src data:; img-src 'none'; object-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors 'self'; sandbox")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		w.Header().Set("X-Prepared-Document-SHA256", pin.SHA256)
		w.Header().Set("X-Conversion-Verification", "not-verified")
		_, _ = w.Write(body)
	})
}

func pricingReferenceQuery(r *http.Request) (url.Values, int, string, error) {
	v := r.URL.Query()
	bad := func() (url.Values, int, string, error) { return nil, 0, "", errors.New("invalid comparison query") }
	if r.URL.EscapedPath() != pricingReferenceRoute {
		return bad()
	}
	for key, values := range v {
		if len(values) != 1 || (key != "theme" && key != "viewport" && key != "preview_theme" && key != "layout" && key != "scale" && key != "active") {
			return bad()
		}
	}
	if theme := v.Get("theme"); theme != "" && theme != "light" && theme != "dark" {
		return bad()
	}
	for key, allowed := range map[string][]string{"layout": {"flip", "side", "stack"}, "scale": {"fit", "actual"}, "active": {"original", "converted"}} {
		if values, exists := v[key]; exists {
			valid := false
			for _, value := range allowed {
				valid = valid || values[0] == value
			}
			if !valid {
				return bad()
			}
		}
	}
	width := 1440
	if value, exists := v["viewport"]; exists {
		if value[0] != "390" && value[0] != "1440" {
			return bad()
		}
		width, _ = strconv.Atoi(value[0])
	}
	previewTheme := "light"
	if value, exists := v["preview_theme"]; exists {
		if value[0] != "light" && value[0] != "dark" {
			return bad()
		}
		previewTheme = value[0]
	}
	return v, width, previewTheme, nil
}

func openPricingReference(root string, d pricingDescriptor) (*os.Root, error) {
	if root == "" {
		return nil, os.ErrNotExist
	}
	detail, err := designattempts.Read(root, d.Receipt.AttemptID)
	if err != nil || detail.Replay != "matches-current" || detail.Translation.Artifact == nil || detail.Entry.Source != d.Receipt.Source || detail.Entry.Transformer != d.Receipt.Transformer || detail.Translation.Artifact.SHA256 != d.Receipt.ArtifactSHA256 || detail.Entry.Verification != "not-verified" {
		return nil, errors.New("prepared reference source/artifact identity is stale or invalid")
	}
	info, err := os.Lstat(root)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("private asset root is unavailable")
	}
	r, err := os.OpenRoot(root)
	if err != nil {
		return nil, err
	}
	for _, name := range []string{"previews", "p02-pricing", d.ReceiptSHA256} {
		info, err := r.Lstat(name)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			r.Close()
			return nil, errors.New("prepared reference directory is unavailable")
		}
		next, err := r.OpenRoot(name)
		r.Close()
		if err != nil {
			return nil, err
		}
		r = next
	}
	body, err := readPricingFile(r, "receipt.json", pricingFilePin{SHA256: d.ReceiptSHA256, Size: d.ReceiptSize}, 64<<10)
	if err != nil {
		r.Close()
		return nil, err
	}
	var receipt pricingReceipt
	if err := json.Unmarshal(body, &receipt); err != nil || !reflect.DeepEqual(receipt, d.Receipt) {
		r.Close()
		return nil, errors.New("prepared receipt and admitted metadata disagree")
	}
	return r, nil
}

// Inputs are owner-prepared immutable files, not an upload directory. Root
// confinement and pre/post identity checks reject ordinary replacement/aliasing;
// this is not isolation from a hostile local process concurrently rewriting files.
func readPricingFile(root *os.Root, name string, pin pricingFilePin, limit int) ([]byte, error) {
	bad := errors.New("prepared reference file is missing, changed or not bounded/regular")
	before, err := root.Lstat(name)
	if err != nil || !before.Mode().IsRegular() || pin.Size < 1 || pin.Size > limit || before.Size() != int64(pin.Size) {
		return nil, bad
	}
	f, err := root.Open(name)
	if err != nil {
		return nil, bad
	}
	defer f.Close()
	after, err := f.Stat()
	if err != nil || !after.Mode().IsRegular() || !os.SameFile(before, after) || after.Size() != int64(pin.Size) {
		return nil, bad
	}
	body, err := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	if err != nil || len(body) != pin.Size || fmt.Sprintf("%x", sha256.Sum256(body)) != pin.SHA256 {
		return nil, bad
	}
	return body, nil
}

type pricingReferencePane struct {
	Side, Label, Title, URL string
	Bytes                   int
}

type pricingReferenceView struct {
	Descriptor                             pricingDescriptor
	Width                                  int
	PreviewTheme, Theme                    string
	SourceURL                              string
	Panes                                  []pricingReferencePane
	Layout, Scale, Active, ScriptIntegrity string
	OriginalURL, ConvertedURL              string
}

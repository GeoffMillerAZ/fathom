package gallery

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

const (
	themeLabPerfOrigin      = "http://127.0.0.1:8765"
	themeLabPerfSourceLabel = "cue/tokens/geoff-synthwave/geoff-synthwave.cue"
	themeLabPerfKitRevision = "app-kit-theme-lab-performance-v1"
	themeLabPerfOutputEnv   = "APPKIT_THEME_LAB_PARITY_OUTPUT"
)

type themeLabPerfCase struct {
	name, method, target, body string
	wantStatus                 int
	wantText                   []string
}

type themeLabPerfObservation struct {
	Name       string      `json:"name"`
	Status     int         `json:"status"`
	Headers    [][2]string `json:"headers"`
	BodySHA256 string      `json:"body_sha256"`
	BodyBytes  int         `json:"body_bytes"`
}

type themeLabPerfSnapshot struct {
	SchemaVersion string                    `json:"schema_version"`
	Origin        string                    `json:"origin"`
	SourceLabel   string                    `json:"source_label"`
	SourceSHA256  string                    `json:"source_sha256"`
	KitRevision   string                    `json:"kit_revision"`
	Observations  []themeLabPerfObservation `json:"observations"`
}

func newThemeLabPerfFixture() (http.Handler, []byte, error) {
	source, err := os.ReadFile("../../cue/tokens/geoff-synthwave/geoff-synthwave.cue")
	if err != nil {
		return nil, nil, err
	}
	handler, err := NewThemeLab(themeLabPerfOrigin, source, themeLabPerfSourceLabel, themeLabPerfKitRevision)
	return handler, source, err
}

func themeLabPerfRequest(handler http.Handler, tc themeLabPerfCase) (*httptest.ResponseRecorder, error) {
	req := httptest.NewRequest(tc.method, themeLabPerfOrigin+tc.target, strings.NewReader(tc.body))
	req.Host = "127.0.0.1:8765"
	req.RemoteAddr = "127.0.0.1:43210"
	if tc.method == http.MethodPost {
		req.Header.Set("Origin", themeLabPerfOrigin)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != tc.wantStatus {
		return recorder, fmt.Errorf("%s: status %d, want %d", tc.name, recorder.Code, tc.wantStatus)
	}
	for _, want := range tc.wantText {
		if !bytes.Contains(recorder.Body.Bytes(), []byte(want)) {
			return recorder, fmt.Errorf("%s: response lacks %q", tc.name, want)
		}
	}
	if tc.name == "candidate-export" {
		sum := sha256.Sum256(recorder.Body.Bytes())
		if recorder.Header().Get("Content-Type") != "application/zip" || recorder.Header().Get("X-Content-SHA256") != hex.EncodeToString(sum[:]) {
			return recorder, errors.New("candidate-export: ZIP content identity is missing or inconsistent")
		}
	}
	return recorder, nil
}

func themeLabPerfForm(body []byte) (url.Values, error) {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	var forms []*html.Node
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "form" && themeLabPerfAttr(node, "action") == "/theme-lab" {
			forms = append(forms, node)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(doc)
	if len(forms) != 1 {
		return nil, fmt.Errorf("theme lab response has %d editor forms", len(forms))
	}
	values := url.Values{}
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "input" {
			if name := themeLabPerfAttr(node, "name"); name != "" {
				values.Add(name, themeLabPerfAttr(node, "value"))
			}
		}
		if node.Type == html.ElementNode && node.Data == "select" {
			name := themeLabPerfAttr(node, "name")
			var first, selected *html.Node
			for child := node.FirstChild; child != nil; child = child.NextSibling {
				if child.Type != html.ElementNode || child.Data != "option" {
					continue
				}
				if first == nil {
					first = child
				}
				if themeLabPerfHasAttr(child, "selected") {
					selected = child
				}
			}
			if selected == nil {
				selected = first
			}
			if name != "" && selected != nil {
				values.Add(name, themeLabPerfAttr(selected, "value"))
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(forms[0])
	return values, nil
}

func themeLabPerfAttr(node *html.Node, name string) string {
	for _, attr := range node.Attr {
		if attr.Key == name {
			return attr.Val
		}
	}
	return ""
}

func themeLabPerfHasAttr(node *html.Node, name string) bool {
	for _, attr := range node.Attr {
		if attr.Key == name {
			return true
		}
	}
	return false
}

func themeLabPerfCases(handler http.Handler) ([]themeLabPerfCase, error) {
	baseline := themeLabPerfCase{
		name: "baseline-page", method: http.MethodGet, target: "/theme-lab", wantStatus: http.StatusOK,
		wantText: []string{"Gallery theme lab", themeLabPerfSourceLabel},
	}
	baselineResponse, err := themeLabPerfRequest(handler, baseline)
	if err != nil {
		return nil, err
	}
	form, err := themeLabPerfForm(baselineResponse.Body.Bytes())
	if err != nil {
		return nil, err
	}
	form.Set("radius.md", "0.75rem")
	form.Set("action", "preview")
	candidate := themeLabPerfCase{
		name: "candidate-preview", method: http.MethodPost, target: "/theme-lab", body: form.Encode(), wantStatus: http.StatusOK,
		wantText: []string{"0.75rem", "Candidate preview"},
	}
	candidateResponse, err := themeLabPerfRequest(handler, candidate)
	if err != nil {
		return nil, err
	}
	acknowledged, err := themeLabPerfForm(candidateResponse.Body.Bytes())
	if err != nil {
		return nil, err
	}
	digest := acknowledged.Get("review_digest")
	decoded, decodeErr := hex.DecodeString(digest)
	if decodeErr != nil || len(decoded) != sha256.Size {
		return nil, errors.New("candidate response lacks a SHA-256 review digest")
	}
	acknowledged.Set("action", "baseline")
	sourceComparison := themeLabPerfCase{
		name: "source-comparison", method: http.MethodPost, target: "/theme-lab", body: acknowledged.Encode(), wantStatus: http.StatusOK,
		wantText: []string{"0.75rem", "Source baseline"},
	}
	sourceResponse, err := themeLabPerfRequest(handler, sourceComparison)
	if err != nil {
		return nil, err
	}
	exportForm, err := themeLabPerfForm(sourceResponse.Body.Bytes())
	if err != nil {
		return nil, err
	}
	if exportForm.Get("review_digest") != digest {
		return nil, errors.New("source comparison did not retain the acknowledged digest")
	}
	exportForm.Set("action", "export")
	export := themeLabPerfCase{name: "candidate-export", method: http.MethodPost, target: "/theme-lab", body: exportForm.Encode(), wantStatus: http.StatusOK}

	cases := []themeLabPerfCase{baseline, candidate, sourceComparison, export}
	for _, specimen := range []struct{ id, text string }{{"dashboard", "Dashboard"}, {"components", "Charts and components"}, {"forms", "Form controls"}} {
		for _, scheme := range []string{"light", "dark"} {
			cases = append(cases, themeLabPerfCase{
				name:       "preview-" + specimen.id + "-" + scheme,
				method:     http.MethodGet,
				target:     "/theme-lab/preview?specimen=" + specimen.id + "&scheme=" + scheme,
				wantStatus: http.StatusOK,
				wantText:   []string{specimen.text, `data-theme="` + scheme + `"`},
			})
		}
	}
	invalid := form
	invalid.Set("radius.md", "-1px")
	invalid.Set("action", "preview")
	cases = append(cases, themeLabPerfCase{
		name: "invalid-field", method: http.MethodPost, target: "/theme-lab", body: invalid.Encode(), wantStatus: http.StatusUnprocessableEntity,
		wantText: []string{"-1px", `aria-invalid="true"`},
	})
	stale := exportForm
	stale.Set("radius.md", "1rem")
	stale.Set("action", "export")
	cases = append(cases, themeLabPerfCase{
		name: "stale-digest", method: http.MethodPost, target: "/theme-lab", body: stale.Encode(), wantStatus: http.StatusConflict,
		wantText: []string{"Export refused", "1rem"},
	})
	return cases, nil
}

func themeLabPerfObserve(handler http.Handler, cases []themeLabPerfCase) ([]themeLabPerfObservation, error) {
	observations := make([]themeLabPerfObservation, 0, len(cases))
	for _, tc := range cases {
		response, err := themeLabPerfRequest(handler, tc)
		if err != nil {
			return nil, err
		}
		headers := make([][2]string, 0)
		for name, values := range response.Header() {
			values = slices.Clone(values)
			sort.Strings(values)
			for _, value := range values {
				headers = append(headers, [2]string{http.CanonicalHeaderKey(name), value})
			}
		}
		sort.Slice(headers, func(i, j int) bool {
			if headers[i][0] != headers[j][0] {
				return headers[i][0] < headers[j][0]
			}
			return headers[i][1] < headers[j][1]
		})
		body := response.Body.Bytes()
		sum := sha256.Sum256(body)
		observations = append(observations, themeLabPerfObservation{
			Name: tc.name, Status: response.Code, Headers: headers,
			BodySHA256: hex.EncodeToString(sum[:]), BodyBytes: len(body),
		})
	}
	return observations, nil
}

func BenchmarkThemeLabRequest(b *testing.B) {
	handler, _, err := newThemeLabPerfFixture()
	if err != nil {
		b.Fatal(err)
	}
	cases, err := themeLabPerfCases(handler)
	if err != nil {
		b.Fatal(err)
	}
	// Invalid and stale submissions are parity cases, not normal workload cases.
	for _, tc := range cases[:len(cases)-2] {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			var responseBytes int64
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				response, requestErr := themeLabPerfRequest(handler, tc)
				if requestErr != nil {
					b.Fatal(requestErr)
				}
				responseBytes += int64(response.Body.Len())
			}
			b.ReportMetric(float64(responseBytes)/float64(b.N), "response-bytes/op")
		})
	}
}

func TestThemeLabRequestParitySnapshot(t *testing.T) {
	handler, source, err := newThemeLabPerfFixture()
	if err != nil {
		t.Fatal(err)
	}
	cases, err := themeLabPerfCases(handler)
	if err != nil {
		t.Fatal(err)
	}
	first, err := themeLabPerfObserve(handler, cases)
	if err != nil {
		t.Fatal(err)
	}
	second, err := themeLabPerfObserve(handler, cases)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.EqualFunc(first, second, func(a, b themeLabPerfObservation) bool {
		return a.Name == b.Name && a.Status == b.Status && a.BodySHA256 == b.BodySHA256 && a.BodyBytes == b.BodyBytes && slices.Equal(a.Headers, b.Headers)
	}) {
		t.Fatal("identical Theme Lab requests did not produce deterministic status, headers and body identity")
	}
	if len(first) != 12 {
		t.Fatalf("observed %d cases, want the complete 12-case workload", len(first))
	}
	sum := sha256.Sum256(source)
	snapshot := themeLabPerfSnapshot{
		SchemaVersion: "appkit-theme-lab-parity-v1", Origin: themeLabPerfOrigin,
		SourceLabel: themeLabPerfSourceLabel, SourceSHA256: hex.EncodeToString(sum[:]),
		KitRevision: themeLabPerfKitRevision, Observations: first,
	}
	compact, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	// The private runner compares this stable machine-readable payload across
	// selected binaries. It is an observation, never an accepted golden.
	t.Logf("theme-lab-parity-json=%s", compact)
	if output := os.Getenv(themeLabPerfOutputEnv); output != "" {
		if !filepath.IsAbs(output) || filepath.Clean(output) != output {
			t.Fatalf("%s must name a canonical absolute fresh file", themeLabPerfOutputEnv)
		}
		body, marshalErr := json.MarshalIndent(snapshot, "", "  ")
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		body = append(body, '\n')
		file, openErr := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if openErr != nil {
			t.Fatalf("create exclusive parity observation: %v", openErr)
		}
		if _, err = file.Write(body); err == nil {
			err = file.Sync()
		}
		if closeErr := file.Close(); err == nil {
			err = closeErr
		}
		if err != nil {
			t.Fatalf("write parity observation: %v", err)
		}
	}
}

package designconvert

import (
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/geoffmilleraz/app-kit/internal/designassets"
)

// This opt-in diagnostic translates in memory only. It never saves derivatives,
// runs vendor code, changes a catalog, compiles targets or promotes compatibility.
// The report contains source identities/counts/hashes/codes, never source bodies.
func TestHTMLInventorySyntaxCoverageLive(t *testing.T) {
	root := os.Getenv("APPKIT_PROBE_HTML_COVERAGE_KIT_ROOT")
	if root == "" {
		t.Skip("set APPKIT_PROBE_HTML_COVERAGE_KIT_ROOT to the existing absolute private kit checkout")
	}
	if !filepath.IsAbs(root) {
		t.Fatal("private kit root must be absolute")
	}
	store := filepath.Join(root, "data/design-assets")
	catalog, err := designassets.Load(store)
	if err != nil {
		t.Fatal("private source catalog unavailable or invalid; details omitted")
	}
	inputs := []struct {
		id    string
		count int
	}{
		{"application-ui@b26cbf6e9529c2219452de32b1a9c49ca12d7bc63bca5e48cd286088521ba0e8", 364},
		{"ecommerce@79c4900dedc217e4a1086f25009522430bfe3e494fb5097402b9bb16b0549757", 114},
		{"marketing@8863da80a161b3dd0f3ee2991657edbc29855ff8dc23915dda1f520fd6859675", 179},
	}
	type identity struct {
		ArchiveID string `json:"archive_id"`
		Path      string `json:"path"`
		SHA256    string `json:"sha256"`
	}
	identities := []identity{}
	sizes := map[identity]int{}
	for _, input := range inputs {
		archive, err := designassets.Verify(store, input.id)
		if err != nil {
			t.Fatal("frozen official archive failed full metadata/integrity verification; do not substitute another revision")
		}
		count := 0
		for _, file := range archive.Files {
			if path.Ext(file.Path) == ".html" {
				key := identity{input.id, file.Path, file.SHA256}
				identities = append(identities, key)
				sizes[key] = int(file.Size)
				count++
			}
		}
		if count != input.count {
			t.Fatalf("frozen archive denominator changed: got %d, want %d", count, input.count)
		}
	}
	sort.Slice(identities, func(i, j int) bool {
		if identities[i].ArchiveID != identities[j].ArchiveID {
			return identities[i].ArchiveID < identities[j].ArchiveID
		}
		return identities[i].Path < identities[j].Path
	})
	identityBytes, err := json.Marshal(identities)
	if err != nil {
		t.Fatal(err)
	}
	const inventorySHA = "d57b19cec955cc89c094e304ab7bb6d39ec1188fc88b0d6cb4c99f50ec0580b0"
	if len(identities) != 657 || digest(identityBytes) != inventorySHA {
		t.Fatal("frozen 657-member identity set changed; counts alone are not source binding")
	}
	type row struct {
		Source             identity `json:"source"`
		Status             string   `json:"status"`
		SourceBytes        int      `json:"source_bytes"`
		OutputBytes        int      `json:"output_bytes"`
		OutputSHA256       string   `json:"output_sha256,omitempty"`
		DiagnosticCodes    []string `json:"diagnostic_codes"`
		OmittedDiagnostics int      `json:"omitted_diagnostics"`
	}
	type totals struct {
		Inputs        int `json:"inputs"`
		Translated    int `json:"translated"`
		Unsupported   int `json:"unsupported"`
		SourceRefused int `json:"source_refused"`
		Replayed      int `json:"replayed"`
	}
	report := struct {
		SchemaVersion   int               `json:"schema_version"`
		Scope           string            `json:"scope"`
		Verification    string            `json:"verification"`
		CatalogRevision string            `json:"catalog_revision"`
		InventorySHA256 string            `json:"inventory_sha256"`
		Transformer     Transformer       `json:"transformer"`
		Totals          totals            `json:"totals"`
		Archives        map[string]totals `json:"archives"`
		DiagnosticFiles map[string]int    `json:"diagnostic_files"`
		Rows            []row             `json:"rows"`
	}{1, "official-html-in-memory-syntax-diagnostic", "not-verified", catalog.Revision, inventorySHA,
		Transformer{}, totals{}, map[string]totals{}, map[string]int{}, []row{}}
	for i, original := range identities {
		family := strings.Split(original.ArchiveID, "@")[0]
		aggregate := report.Archives[family]
		aggregate.Inputs++
		report.Totals.Inputs++
		file, err := designassets.ReadFile(store, original.ArchiveID, original.Path)
		if int64(sizes[original]) > designassets.MaxTextBytes {
			if err == nil || !strings.Contains(err.Error(), "exceeds text limit") {
				t.Fatal("oversized source did not retain the existing text-access refusal")
			}
			aggregate.SourceRefused++
			report.Totals.SourceRefused++
			report.Archives[family] = aggregate
			report.DiagnosticFiles["source-size"]++
			report.Rows = append(report.Rows, row{Source: original, Status: "source-size-refused", SourceBytes: sizes[original], DiagnosticCodes: []string{"source-size"}})
			continue // Included in the denominator, never a translated/replayed case.
		}
		if err != nil || file.SHA256 != original.SHA256 {
			t.Fatalf("source %d integrity/read failed; source/error body omitted", i)
		}
		body := []byte(file.Content)
		translation, err := Translate(Source{file.ArchiveID, file.Path, file.SHA256, len(body)}, body)
		if err != nil {
			t.Fatalf("source %d operational translation failure; details omitted", i)
		}
		if err := VerifyReplay(translation, body); err != nil || translation.Verification != "not-verified" {
			t.Fatalf("source %d replay/evidence boundary failed", i)
		}
		if report.Transformer.ID == "" {
			report.Transformer = translation.Transformer
		} else if report.Transformer != translation.Transformer {
			t.Fatal("transformer changed during one coverage run")
		}
		r := row{Source: original, Status: translation.Status, SourceBytes: len(body), DiagnosticCodes: []string{}, OmittedDiagnostics: translation.OmittedDiagnostics}
		aggregate.Replayed++
		report.Totals.Replayed++
		if translation.Status == "translated" {
			if translation.Artifact == nil {
				t.Fatal("translated input lacks an artifact")
			}
			r.OutputBytes, r.OutputSHA256 = len(translation.Artifact.Content), translation.Artifact.SHA256
			aggregate.Translated++
			report.Totals.Translated++
		} else if translation.Status == "unsupported" && translation.Artifact == nil {
			aggregate.Unsupported++
			report.Totals.Unsupported++
			seen := map[string]bool{}
			for _, diagnostic := range translation.Diagnostics {
				if !seen[diagnostic.Code] {
					seen[diagnostic.Code] = true
					r.DiagnosticCodes = append(r.DiagnosticCodes, diagnostic.Code)
					report.DiagnosticFiles[diagnostic.Code]++
				}
			}
			sort.Strings(r.DiagnosticCodes)
		} else {
			t.Fatal("unknown status or unsupported input leaked an artifact")
		}
		report.Archives[family] = aggregate
		report.Rows = append(report.Rows, r)
		if (i+1)%100 == 0 {
			t.Logf("assessed=%d/657; syntax-only, no persisted derivative or target certification", i+1)
		}
	}
	if report.Totals.Inputs != 657 || len(report.Rows) != 657 || report.Totals.Translated+report.Totals.Unsupported+report.Totals.SourceRefused != 657 {
		t.Fatal("coverage denominator or outcome accounting is incomplete")
	}
	after, err := designassets.Load(store)
	if err != nil || after.Revision != catalog.Revision {
		t.Fatal("source catalog changed during diagnostic")
	}
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if output := os.Getenv("APPKIT_PROBE_HTML_COVERAGE_REPORT"); output != "" {
		if !filepath.IsAbs(output) {
			t.Fatal("optional metadata report path must be absolute")
		}
		f, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			t.Fatal("report destination must be a new private file; no overwrite")
		}
		_, writeErr := f.Write(append(encoded, '\n'))
		closeErr := f.Close()
		if writeErr != nil || closeErr != nil {
			t.Fatal("metadata report write failed")
		}
	}
	t.Logf("inputs=%d translated=%d unsupported=%d source_refused=%d replayed=%d diagnostic_files=%v archives=%v report_sha256=%s verified_targets=0 persisted_derivatives=0", report.Totals.Inputs, report.Totals.Translated, report.Totals.Unsupported, report.Totals.SourceRefused, report.Totals.Replayed, report.DiagnosticFiles, report.Archives, digest(append(encoded, '\n')))
}

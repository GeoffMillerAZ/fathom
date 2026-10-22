package main

// doctor_languages.go — O69 (wave P1): the `unsupported_languages` row.
//
// The Long Code Arena bench run indexed bumble-tech/appyx — a real Kotlin
// repository — and found it reporting only ~165 symbols, list_services
// naming its language "javascript", find_symbol returning zero Kotlin
// candidates, and outline coming back empty for a real, non-empty .kt file.
// Kotlin has no tree-sitter grammar or extractor wired into this repo, and
// every .kt/.kts file fell through fusion's dispatch to the generic
// "no extractor registered" branch with no counter anywhere naming what was
// lost. Faking a grammar is out of scope; this row is the disclosure half:
// it reads list_services' unsupported_languages breakdown (internal/mcp's
// platform_tools.go, fed by connector.UnsupportedLanguageForPath) for THIS
// repo's own resolved service and warns by language name and file count
// when there is anything to report, so the gap is visible at `fathom
// doctor` time — not just recoverable after someone notices a suspiciously
// low symbol count for a repo they know isn't small.
//
// Deliberate verdict shape, same house style as checkTypedEdges:
//   - WARN, never FAIL: an unsupported language is a real capability gap,
//     not a misconfiguration — nothing here is "broken" the way a wedged
//     typed-edge pass is. doctorWarn counts as healthy for exit-code
//     purposes (doctor.go), matching that intent.
//   - PASS when list_services reports nothing unsupported for this service.
//   - SKIP, with the reason, for everything doctor cannot honestly judge: a
//     daemon that is not reachable, no resolvable service identity, or a
//     service this daemon process has not ingested (absent from
//     list_services) — never a guessed pass.
import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
)

func checkUnsupportedLanguages(hc *http.Client, baseURL, root, explicitSvc string, daemonUp bool) doctorCheck {
	const name = "unsupported_languages"

	if !daemonUp {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: "daemon not reachable — language coverage is the daemon's own fact and cannot be read from here", Remedy: daemonRestartHint("")}
	}

	svc := explicitSvc
	if svc == "" {
		cands := candidatesWithDaemon(hc, baseURL, root)
		if len(cands) == 0 {
			return doctorCheck{Name: name, Status: doctorSkipped, Reason: "no service identity resolvable for this repo"}
		}
		svc = strings.ToLower(cands[0].name)
	}

	inner, err := callMCPTool(hc, baseURL, "list_services", map[string]any{})
	if err != nil {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: fmt.Sprintf("list_services did not answer: %v", err), probeErr: err}
	}
	var payload struct {
		Services []struct {
			Service              string `json:"service"`
			UnsupportedLanguages []struct {
				Language string `json:"language"`
				Files    int    `json:"files"`
			} `json:"unsupported_languages"`
		} `json:"services"`
	}
	if err := json.Unmarshal(inner, &payload); err != nil {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: fmt.Sprintf("list_services returned an unreadable payload: %v", err)}
	}

	for _, s := range payload.Services {
		if !strings.EqualFold(s.Service, svc) {
			continue
		}
		if len(s.UnsupportedLanguages) == 0 {
			return doctorCheck{Name: name, Status: doctorPass, Reason: fmt.Sprintf("%q has no source files in a known-but-unextracted language", s.Service)}
		}
		rows := append([]struct {
			Language string `json:"language"`
			Files    int    `json:"files"`
		}(nil), s.UnsupportedLanguages...)
		sort.Slice(rows, func(i, j int) bool { return rows[i].Language < rows[j].Language })
		parts := make([]string, 0, len(rows))
		for _, r := range rows {
			parts = append(parts, fmt.Sprintf("%s: %d file(s)", r.Language, r.Files))
		}
		return doctorCheck{
			Name: name, Status: doctorWarn,
			Reason: fmt.Sprintf("%q has source in languages fathom cannot extract (zero symbols, zero call graph): %s", s.Service, strings.Join(parts, ", ")),
			Remedy: "these files have no registered extractor — treat find_symbol/outline/call-graph results for this service as covering only the languages list_services names, not these",
		}
	}
	return doctorCheck{Name: name, Status: doctorSkipped, Reason: fmt.Sprintf("%q not found in list_services (not yet ingested by this daemon)", svc)}
}

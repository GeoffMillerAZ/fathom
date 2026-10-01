package main

// i42_decide_golden_test.go — I42 (judgment-foundations design §4, JF-5): with
// no [decide] block at all (config.Load of no layer, provider none, the default
// sites) every surface the judgment-foundations rows touch prints exactly what
// it printed before them. The golden was captured at the pre-change HEAD
// (2026-10-01, before JF-4..JF-10) and must stay byte-identical through every
// later commit: a diff here is an I42 break, never a golden to refresh.

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/config"
)

const i42UnconfiguredGolden = "testdata/i42/unconfigured-decide.golden"

// TestI42UnconfiguredDecideOutputIsByteIdentical renders, from an
// unconfigured config.Load, the doctor decision_provider row, `fathom certify
// suggest`'s no-provider line, and POST /route's no-port body, and compares
// them with the golden captured before any judgment-foundations change.
func TestI42UnconfiguredDecideOutputIsByteIdentical(t *testing.T) {
	loaded, err := config.Load("", "")
	if err != nil {
		t.Fatalf("config.Load with no layer: %v", err)
	}
	var out bytes.Buffer

	// The doctor's decision_provider row, as the table and as JSON.
	row := decisionProviderVerdict(loaded, nil, "")
	out.WriteString("== doctor decision_provider (table)\n")
	printDoctorTable(&out, "claude", "svc", "/repo", []doctorCheck{row})
	out.WriteString("== doctor decision_provider (json)\n")
	rowJSON, err := json.Marshal(row)
	if err != nil {
		t.Fatal(err)
	}
	out.Write(rowJSON)
	out.WriteString("\n")

	// openDecisionPorts with no provider: no warn, no startup line.
	var openErr bytes.Buffer
	ports := openDecisionPorts(loaded, "", "", "fathom certify suggest", &openErr)
	out.WriteString("== openDecisionPorts stderr\n")
	out.Write(openErr.Bytes())

	// certify suggest's no-provider line.
	var suggestOut, suggestErr bytes.Buffer
	code := runCertifySuggest(nil, ports, func() time.Time { return time.Unix(0, 0).UTC() }, &suggestOut, &suggestErr)
	out.WriteString("== certify suggest stdout\n")
	out.Write(suggestOut.Bytes())
	out.WriteString("== certify suggest stderr\n")
	out.Write(suggestErr.Bytes())
	out.WriteString("== certify suggest exit\n")
	out.WriteString(strconv.Itoa(code) + "\n")

	// POST /route with no port: the fixed none-shaped body, no call.
	rr := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/route", strings.NewReader(`{"prompt":"where is Open defined"}`))
	routeHandler(ports).ServeHTTP(rr, req)
	out.WriteString("== route status\n")
	out.WriteString(http.StatusText(rr.Code) + "\n")
	out.WriteString("== route body\n")
	out.Write(rr.Body.Bytes())

	if *updateGolden {
		if err := os.MkdirAll(filepath.Dir(i42UnconfiguredGolden), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(i42UnconfiguredGolden, out.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(i42UnconfiguredGolden)
	if err != nil {
		t.Fatalf("reading %s: %v (capture it once at the pre-change HEAD with -update)", i42UnconfiguredGolden, err)
	}
	if !bytes.Equal(out.Bytes(), want) {
		t.Errorf("unconfigured [decide] output changed (I42); never refresh this golden to pass:\n--- got\n%s\n--- want\n%s", out.String(), want)
	}
}

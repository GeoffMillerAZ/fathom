package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// decodeCompact is an independent wire decoder. It only knows the published
// tuple grammar and ordinary omitempty behavior, not the encoder's Go structs.
func decodeCompact(t *testing.T, wire []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(wire, &m); err != nil {
		t.Fatal(err)
	}
	version, _ := m["version"].(string)
	if version != "command-guard.compact-report/1" && version != "command-guard.compact-git-report/1" {
		return m
	}
	layout := m["rowFormat"].(string)
	delete(m, "rowFormat")
	if version == "command-guard.compact-report/1" {
		m["version"] = "command-guard.report/1"
	} else {
		m["version"] = "command-guard.git-report/1"
	}
	keys := map[string][]string{
		"file-line-text/1": {"file", "line", "text"},
		"status/1":         {"kind", "path", "xy"},
		"object-diff/1":    {"path", "beforeOid", "afterOid", "before", "after", "beforeOmittedLines", "afterOmittedLines"},
		"history/1":        {"oid", "parentIds", "author", "committer", "message", "messageOmittedBytes"},
	}[layout]
	if keys == nil {
		t.Fatalf("unknown layout %q", layout)
	}
	encodedRows := m["rows"].([]any)
	if len(encodedRows) == 0 {
		delete(m, "rows")
		return m
	}
	decoded := make([]any, 0, len(encodedRows))
	for _, raw := range encodedRows {
		values := raw.([]any)
		if len(values) != len(keys) {
			t.Fatalf("bad arity %d", len(values))
		}
		item := map[string]any{}
		if layout == "object-diff/1" {
			item["kind"] = "change"
		}
		if layout == "history/1" {
			item["kind"] = "commit"
		}
		for i, key := range keys {
			v := values[i]
			if layout != "file-line-text/1" && (v == "" || v == float64(0) || reflect.DeepEqual(v, []any{})) {
				continue
			}
			item[key] = v
		}
		decoded = append(decoded, item)
	}
	m["rows"] = decoded
	return m
}

func assertCompactEqual(t *testing.T, ordinary, compact []byte) {
	t.Helper()
	if len(compact) > len(ordinary) {
		t.Fatalf("compact %d > ordinary %d", len(compact), len(ordinary))
	}
	var want map[string]any
	if err := json.Unmarshal(ordinary, &want); err != nil {
		t.Fatal(err)
	}
	if got := decodeCompact(t, compact); !reflect.DeepEqual(got, want) {
		t.Fatalf("decoded mismatch\ngot %#v\nwant %#v", got, want)
	}
}

func TestCompactInspect(t *testing.T) {
	r := base()
	r.Status, r.Code, r.FilesRead, r.BytesRead, r.BytesScanned = "no-findings", "ok", 1, 4000, 4000
	for i := 1; i <= 400; i++ {
		value := fmt.Sprintf("line %03d: α\t\"quoted\"", i)
		if i == 289 {
			value += " MARKER"
		}
		r.Rows = append(r.Rows, row{File: 0, Line: i, Text: value})
	}
	r.TotalMatches, r.Returned = 400, 400
	ordinary, compact := encode(r, "json"), encode(r, "compact")
	assertCompactEqual(t, ordinary, compact)
	if !bytes.Contains(compact, []byte("MARKER")) || len(compact)*100 > len(ordinary)*85 {
		t.Fatalf("marker/savings: %d vs %d", len(compact), len(ordinary))
	}
	for _, rows := range [][]row{nil, {{File: 0, Line: 1, Text: "quote \" tab\t CRLF\r\n é"}}} {
		r.Rows, r.TotalMatches, r.Returned = rows, len(rows), len(rows)
		assertCompactEqual(t, encode(r, "json"), encode(r, "compact"))
	}
	r.Status, r.Code = "blocked", "rule"
	if !bytes.Equal(encode(r, "json"), encode(r, "compact")) {
		t.Fatal("failure changed")
	}
}

func TestCompactGitLayouts(t *testing.T) {
	baseReport := gitBase()
	baseReport.Status, baseReport.Code, baseReport.Scope = "no-findings", "ok", []string{"."}
	baseReport.AttemptedCalls, baseReport.StartedCalls, baseReport.CompletedCalls = 2, 2, 2
	baseReport.NonAtomic = true
	for _, tc := range []struct {
		name, op string
		rows     []gitRow
	}{
		{"status", "git-status", func() []gitRow {
			a := make([]gitRow, 40)
			for i := range a {
				a[i] = gitRow{Kind: "untracked", Path: fmt.Sprintf("p%02d.txt", i), XY: "??"}
			}
			return a
		}()},
		{"diff", "git-object-diff", []gitRow{{Kind: "change", Path: "a.txt", AfterOID: strings.Repeat("a", 40), Before: []string{}, After: []string{"é", "\"\t"}, AfterOmittedLines: 3}}},
		{"history", "git-history", []gitRow{{Kind: "commit", OID: strings.Repeat("b", 40), ParentIDs: []string{}, Author: "A", Committer: "C", Message: "root\n", MessageOmittedBytes: 2}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := baseReport
			r.Operation, r.Rows, r.Returned = tc.op, tc.rows, len(tc.rows)
			if tc.op == "git-object-diff" {
				r.BaseCommit, r.TargetCommit = strings.Repeat("a", 40), strings.Repeat("b", 40)
			}
			if tc.op == "git-history" {
				r.Scope = nil
				r.TargetCommit = strings.Repeat("b", 40)
			}
			assertCompactEqual(t, encodeGit(r, "json"), encodeGit(r, "compact"))
			if tc.op == "git-status" && len(encodeGit(r, "compact"))*100 > len(encodeGit(r, "json"))*85 {
				t.Fatal("status savings below 15%")
			}
			r.Rows = append([]gitRow{}, tc.rows...)
			r.Rows[0].Message = "unexpected"
			if tc.op == "git-history" {
				r.Rows[0].Path = "unexpected"
			}
			if !bytes.Equal(encodeGit(r, "json"), encodeGit(r, "compact")) {
				t.Fatal("unexpected row field lost")
			}
		})
	}
	baseReport.Operation, baseReport.Rows, baseReport.Returned = "git-status", []gitRow{{Kind: "future", Path: "a"}}, 1
	if !bytes.Equal(encodeGit(baseReport, "json"), encodeGit(baseReport, "compact")) {
		t.Fatal("future kind lost")
	}
}

type shortWriter struct{}

func (shortWriter) Write(p []byte) (int, error) { return len(p) / 2, io.ErrShortWrite }

func TestCompactWriter(t *testing.T) {
	r := base()
	r.Status, r.Code = "no-findings", "ok"
	if emit(shortWriter{}, r, "compact", 0) != 2 {
		t.Fatal("short writer passed")
	}
	g := gitBase()
	g.Status, g.Code, g.Operation = "no-findings", "ok", "git-status"
	if emitGit(shortWriter{}, g, "compact", 0) != 2 {
		t.Fatal("short Git writer passed")
	}
}

func TestCompactGuardedCapAndHiddenFinding(t *testing.T) {
	root, s, p, q := fixture(t)
	var lines []string
	for i := 1; i <= 20; i++ {
		lines = append(lines, fmt.Sprintf("safe line %02d", i))
	}
	content := []byte(strings.Join(lines, "\n") + "\n")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), content, 0600); err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(content)
	q.Files[0].SHA256 = hex.EncodeToString(h[:])
	q.EndLine = 20
	p.Layers[0].Limits.MaxResults = 20
	p.Layers[0].Limits.MaxOutputBytes = 4096
	full := run(root, s, p, q, "json")
	if full.Returned != 20 {
		t.Fatal(full)
	}
	ordinary, compact := encode(full, "json"), encode(full, "compact")
	if len(compact) >= len(ordinary) {
		t.Fatalf("no compact savings %d %d", len(compact), len(ordinary))
	}
	p.Layers[0].Limits.MaxOutputBytes = len(compact) + 1
	jsonView := run(root, s, p, q, "json")
	compactView := run(root, s, p, q, "compact")
	if compactView.Returned != 20 || jsonView.Returned >= compactView.Returned || len(encode(compactView, "compact"))+1 > p.Layers[0].Limits.MaxOutputBytes {
		t.Fatalf("json=%+v compact=%+v", jsonView, compactView)
	}
	assertCompactEqual(t, encode(compactView, "json"), encode(compactView, "compact"))
	content = append(content, []byte("SECRET hidden\n")...)
	if err := os.WriteFile(filepath.Join(root, "a.txt"), content, 0600); err != nil {
		t.Fatal(err)
	}
	h = sha256.Sum256(content)
	q.Files[0].SHA256 = hex.EncodeToString(h[:])
	blocked := run(root, s, p, q, "compact")
	if blocked.Status != "blocked" || blocked.Returned != 0 || !bytes.Equal(encode(blocked, "json"), encode(blocked, "compact")) || bytes.Contains(encode(blocked, "compact"), []byte("SECRET")) {
		t.Fatalf("hidden finding: %+v", blocked)
	}
}

func TestCompactGitNormalizedRepresentationCap(t *testing.T) {
	root, s, p := gitFixture(t)
	q := gitRequest{Version: "command-guard.git-request/1", Kind: "git-status", Paths: []string{"."}}
	var raw strings.Builder
	total := 0
	for i := 0; i < 10; i++ {
		rec := fmt.Sprintf("? p%02d.txt", i)
		total += len(rec)
		raw.WriteString(rec + "\x00")
	}
	runner := fakeGit(nil, raw.String(), "", "")
	full := runGit(root, s, p, q, "compact", runner)
	if full.Status != "no-findings" || full.Returned != 10 {
		t.Fatalf("full: %+v", full)
	}
	reprSize := func(rows []gitRow) int {
		b, _ := json.Marshal(struct {
			Scope []string `json:"scope"`
			Rows  []gitRow `json:"rows"`
		}{q.Paths, rows})
		return len(b)
	}
	p.Layers[0].Limits.MaxTotalBytes = total + reprSize(full.Rows[:5])
	r := runGit(root, s, p, q, "compact", runner)
	if r.Status != "no-findings" || r.Returned != 5 || r.Omitted != 5 || len(encodeGit(r, "compact"))+1 > p.Layers[0].Limits.MaxOutputBytes {
		t.Fatalf("repr cap: %+v", r)
	}
	p.Layers[0].Limits.MaxTotalBytes = total + reprSize([]gitRow{}) - 1
	r = runGit(root, s, p, q, "compact", runner)
	if r.Status != "incomplete" || r.Code != "payload_limit" || r.Returned != 0 {
		t.Fatalf("empty prefix: %+v", r)
	}
}

func TestCompactInspectEntrypoint(t *testing.T) {
	root, err := filepath.Abs("testdata")
	if err != nil {
		t.Fatal(err)
	}
	args := []string{"inspect", "--root", root, "--scope", "testdata/scope.json", "--policy", "testdata/policy.json", "--request", "testdata/request-read.json", "--format", "json"}
	var ordinary, compact bytes.Buffer
	if code := execute(args, &ordinary); code != 0 {
		t.Fatalf("ordinary code %d: %s", code, ordinary.String())
	}
	args[len(args)-1] = "compact"
	if code := execute(args, &compact); code != 0 {
		t.Fatalf("compact code %d: %s", code, compact.String())
	}
	if !bytes.HasSuffix(compact.Bytes(), []byte("\n")) {
		t.Fatal("missing newline")
	}
	assertCompactEqual(t, bytes.TrimSuffix(ordinary.Bytes(), []byte("\n")), bytes.TrimSuffix(compact.Bytes(), []byte("\n")))
}

func TestCompactPrefixBoundaries(t *testing.T) {
	root, s, p, q := fixture(t)
	var lines []string
	for i := 0; i < 100; i++ {
		lines = append(lines, fmt.Sprintf("value%03d", i))
	}
	content := []byte(strings.Join(lines, "\n") + "\n")
	if err := os.WriteFile(filepath.Join(root, "a.txt"), content, 0600); err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(content)
	q.Files[0].SHA256 = hex.EncodeToString(h[:])
	q.EndLine = 100
	p.Layers[0].Limits.MaxFileBytes = 4096
	p.Layers[0].Limits.MaxTotalBytes = 8192
	p.Layers[0].Limits.MaxOutputBytes = 8192
	p.Layers[0].Limits.MaxResults = 100
	full := run(root, s, p, q, "compact")
	if full.Returned != 100 {
		t.Fatal(full)
	}
	for _, n := range []int{9, 10, 99, 100} {
		probe := full
		probe.Rows = full.Rows[:n]
		probe.Returned = n
		probe.Omitted = 100 - n
		probe.Truncated = n < 100
		p.Layers[0].Limits.MaxOutputBytes = len(encode(probe, "compact")) + 1
		got := run(root, s, p, q, "compact")
		if got.Returned != n || got.Omitted != 100-n || len(encode(got, "compact"))+1 > p.Layers[0].Limits.MaxOutputBytes {
			t.Fatalf("boundary %d: %+v", n, got)
		}
	}
}

func BenchmarkCompactInspect400(b *testing.B) {
	r := base()
	r.Status, r.Code = "no-findings", "ok"
	for i := 1; i <= 400; i++ {
		r.Rows = append(r.Rows, row{File: 0, Line: i, Text: fmt.Sprintf("line %03d", i)})
	}
	r.TotalMatches, r.Returned = 400, 400
	for _, format := range []string{"json", "compact"} {
		b.Run(format, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = encode(r, format)
			}
		})
	}
}

func TestCompactUnknownHeaderFallback(t *testing.T) {
	for _, version := range []string{"command-guard.compact-report/1", "command-guard.compact-git-report/1"} {
		ordinary := []byte(`{"version":"future","futureHeader":"must survive","rows":[{"file":0,"line":1,"text":"long enough source row to benefit"}]}`)
		got := compactObject(ordinary, version, "file-line-text/1", [][3]any{{0, 1, "long enough source row to benefit"}})
		if !bytes.Equal(got, ordinary) {
			t.Fatalf("unknown header changed for %s", version)
		}
	}
}

type customCompactRow struct{}

func (*customCompactRow) MarshalJSON() ([]byte, error) {
	return []byte(`{"different":"preserve"}`), nil
}

func TestCompactRowWireEligibility(t *testing.T) {
	allowed := map[string]bool{"file": true, "line": true, "text": true}
	for _, tc := range []struct {
		name string
		row  any
		want bool
	}{
		{"plain", row{File: 0, Line: 1, Text: "value"}, true},
		{"pointer-marshaler", customCompactRow{}, false},
		{"unknown-omitted", struct {
			Text string `json:"text"`
			More string `json:"more,omitempty"`
		}{Text: "value"}, true},
		{"unknown-present", struct {
			Text string `json:"text"`
			More string `json:"more,omitempty"`
		}{Text: "value", More: "preserve"}, false},
		{"unknown-required", struct {
			More string `json:"more"`
		}{}, false},
		{"new-string-option", struct {
			Line int `json:"line,string"`
		}{1}, false},
		{"unfamiliar-kind", struct {
			More bool `json:"more,omitempty"`
		}{}, false},
		{"pointer-row", &row{Text: "value"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := exactRow(tc.row, allowed); got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}

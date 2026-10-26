package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"math/rand/v2"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

const (
	qualifyRulePattern          = "SECRET"
	qualifyDeniedPrefix         = "private"
	qualifyMaxTotal             = 8192
	qualifyMaxResults           = 32
	qualifyScannerProfile       = "gitleaks-stdin-8.30.1"
	qualifyScannerTimeoutMillis = 15000
	qualifyAmbientConfig        = "[allowlist]\npaths = ['.*']\n"
)

type fixture struct {
	id, format                                                              string
	files                                                                   []fileInput
	paths                                                                   []string
	kind                                                                    string
	start, end                                                              int
	literal                                                                 string
	maxFile, maxOutput                                                      int
	wrongHash, missingControl, missingScannerScope, wrongScannerHash, reuse bool
	scanner                                                                 bool
	wantExit                                                                int
	wantStatus, wantCode                                                    string
	wantRows                                                                []fileRow
	wantTotal, wantRead, wantScanned                                        int
	wantScanner                                                             *scannerWant
	actualCompact                                                           bool
	classifier                                                              bool
}
type fileInput struct {
	name    string
	body    []byte
	symlink string
}
type fileRow struct {
	File int    `json:"file"`
	Line int    `json:"line"`
	Text string `json:"text"`
}
type scannerWant struct {
	status, code                                                                         string
	expected, attempted, invoked, completed, submitted, observed, reuseFiles, reuseBytes int
	finding                                                                              bool
	reuse                                                                                bool
}
type externalReport struct {
	ID             string `json:"id"`
	Profile        string `json:"profile"`
	Status         string `json:"status"`
	Code           string `json:"code"`
	ExpectedFiles  int    `json:"expectedFiles"`
	AttemptedFiles int    `json:"attemptedFiles"`
	InvokedFiles   int    `json:"invokedFiles"`
	CompletedFiles int    `json:"completedFiles"`
	BytesSubmitted int    `json:"bytesSubmitted"`
	BytesObserved  int    `json:"bytesObserved"`
	Findings       int    `json:"findings"`
	Reuse          *struct {
		Profile string `json:"profile"`
		Files   int    `json:"files"`
		Bytes   int    `json:"bytes"`
	} `json:"reuse,omitempty"`
}
type candidateReport struct {
	Version            string            `json:"version"`
	Status             string            `json:"status"`
	Code               string            `json:"code"`
	NoActionAuthority  bool              `json:"noActionAuthority"`
	FilesRead          int               `json:"filesRead"`
	BytesRead          int               `json:"bytesRead"`
	BytesScanned       int               `json:"bytesScanned"`
	Scope              string            `json:"scope"`
	Scanner            string            `json:"scanner"`
	ExternalScanner    *externalReport   `json:"externalScanner,omitempty"`
	ExternalClassifier *classifierReport `json:"externalClassifier,omitempty"`
	Limitation         string            `json:"limitation"`
	TotalMatches       int               `json:"totalMatches"`
	Returned           int               `json:"returned"`
	Omitted            int               `json:"omitted"`
	Truncated          bool              `json:"truncated"`
	Rows               []fileRow         `json:"rows,omitempty"`
}

var publicA = []byte("alpha public\nbeta public\n")
var publicB = []byte("gamma public\n")

func sum(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func canary() []byte {
	const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	rng := rand.New(rand.NewPCG(1701, 831))
	v := make([]byte, 36)
	for i := range v {
		v[i] = alphabet[rng.IntN(len(alphabet))]
	}
	return []byte("token = \"ghp_" + string(v) + "\"\n")
}
func baseFixture(id string) fixture {
	f := fixture{id: id, format: "json", files: []fileInput{{name: "a.txt", body: publicA}}, paths: []string{"a.txt"}, kind: "read-range", start: 1, end: 2, maxFile: 4096, maxOutput: 4096, wantStatus: "no-findings", wantCode: "ok", wantRows: []fileRow{{0, 1, "alpha public"}, {0, 2, "beta public"}}, wantTotal: 2, wantRead: len(publicA), wantScanned: len(publicA)}
	switch id {
	case "read-text":
		f.format = "text"
	case "read-compact":
		f.format = "compact"
		f.actualCompact = true
		f.end = 20
		f.files[0].body = []byte(strings.Repeat("public line\n", 20))
		f.wantRead = len(f.files[0].body)
		f.wantScanned = f.wantRead
		f.wantRows = nil
		for i := 1; i <= 20; i++ {
			f.wantRows = append(f.wantRows, fileRow{0, i, "public line"})
		}
		f.wantTotal = 20
	case "ordinary-compact-fallback":
		f.format = "compact"
		f.files[0].body = []byte("z\n")
		f.wantRows = []fileRow{{0, 1, "z"}}
		f.wantTotal = 1
		f.wantRead = 2
		f.wantScanned = 2
	case "search-literal":
		f.kind = "search-literal"
		f.literal = "beta"
		f.wantRows = []fileRow{{0, 2, "beta public"}}
		f.wantTotal = 1
	case "late-regex":
		f.files = append(f.files, fileInput{name: "b.txt", body: []byte("SECRET\n")})
		f.paths = append(f.paths, "b.txt")
		f.wantRows = nil
		f.wantTotal = 0
		f.wantRead = len(publicA) + 7
		f.wantScanned = f.wantRead
		f.wantStatus = "blocked"
		f.wantCode = "rule"
		f.wantExit = 1
	case "wrong-input-hash":
		f.wrongHash = true
		f.wantRows = nil
		f.wantTotal = 0
		f.wantScanned = 0
		f.wantStatus = "incomplete"
		f.wantCode = "invalid"
		f.wantExit = 2
	case "missing-control":
		f.missingControl = true
		f.wantRows = nil
		f.wantTotal = 0
		f.wantRead = 0
		f.wantScanned = 0
		f.wantStatus = "incomplete"
		f.wantCode = "invalid"
		f.wantExit = 2
	case "denied-prefix":
		f.files[0].name = "private/a.txt"
		f.paths[0] = "private/a.txt"
		f.wantRows = nil
		f.wantTotal = 0
		f.wantRead = 0
		f.wantScanned = 0
		f.wantStatus = "blocked"
		f.wantCode = "denied"
		f.wantExit = 1
	case "traversal":
		f.paths[0] = "../a.txt"
		f.wantRows = nil
		f.wantTotal = 0
		f.wantRead = 0
		f.wantScanned = 0
		f.wantStatus = "incomplete"
		f.wantCode = "invalid"
		f.wantExit = 2
	case "case-alias":
		f.paths = []string{"a.txt", "A.txt"}
		f.wantRows = nil
		f.wantTotal = 0
		f.wantRead = 0
		f.wantScanned = 0
		f.wantStatus = "incomplete"
		f.wantCode = "invalid"
		f.wantExit = 2
	case "reserved-name":
		f.paths[0] = "CON.txt"
		f.wantRows = nil
		f.wantTotal = 0
		f.wantRead = 0
		f.wantScanned = 0
		f.wantStatus = "incomplete"
		f.wantCode = "invalid"
		f.wantExit = 2
	case "invalid-utf8":
		f.files[0].body = []byte{0xff, 0xfe}
		f.wantRows = nil
		f.wantTotal = 0
		f.wantRead = 2
		f.wantScanned = 0
		f.wantStatus = "incomplete"
		f.wantCode = "invalid"
		f.wantExit = 2
	case "file-cap":
		f.maxFile = 4
		f.wantRows = nil
		f.wantTotal = 0
		f.wantRead = 5
		f.wantScanned = 0
		f.wantStatus = "incomplete"
		f.wantCode = "invalid"
		f.wantExit = 2
	case "symlink":
		f.files[0].symlink = "target.txt"
		f.wantRows = nil
		f.wantTotal = 0
		f.wantRead = 0
		f.wantScanned = 0
		f.wantStatus = "incomplete"
		f.wantCode = "invalid"
		f.wantExit = 2
	case "tiny-json", "tiny-text", "tiny-compact":
		f.maxOutput = 1
		f.wantRows = nil
		f.wantTotal = 0
		f.wantRead = 0
		f.wantScanned = 0
		f.wantStatus = ""
		f.wantCode = ""
		f.wantExit = 2
		if id == "tiny-text" {
			f.format = "text"
		}
		if id == "tiny-compact" {
			f.format = "compact"
		}
	}
	return f
}
func scannerFixture(id string) fixture {
	f := baseFixture("read-json")
	f.id = id
	f.scanner = true
	f.wantScanner = &scannerWant{status: "complete", code: "verified", expected: 1, attempted: 1, invoked: 1, completed: 1, submitted: len(publicA), observed: len(publicA)}
	switch id {
	case "scanner-empty":
		f.files[0].body = []byte{}
		f.wantRows = nil
		f.wantTotal = 0
		f.wantRead = 0
		f.wantScanned = 0
		f.wantScanner.submitted = 0
		f.wantScanner.observed = 0
	case "scanner-aba-disabled", "scanner-aba-enabled":
		f.files = []fileInput{{name: "a.txt", body: publicA}, {name: "b.txt", body: publicB}, {name: "c.txt", body: publicA}}
		f.paths = []string{"a.txt", "b.txt", "c.txt"}
		f.wantRows = []fileRow{{0, 1, "alpha public"}, {0, 2, "beta public"}, {1, 1, "gamma public"}, {2, 1, "alpha public"}, {2, 2, "beta public"}}
		f.wantTotal = 5
		f.wantRead = 2*len(publicA) + len(publicB)
		f.wantScanned = f.wantRead
		f.wantScanner.expected = 3
		f.wantScanner.attempted = 3
		f.wantScanner.invoked = 3
		f.wantScanner.completed = 3
		f.wantScanner.submitted = f.wantRead
		f.wantScanner.observed = f.wantRead
		if id == "scanner-aba-enabled" {
			f.reuse = true
			f.wantScanner.reuse = true
			f.wantScanner.attempted = 2
			f.wantScanner.invoked = 2
			f.wantScanner.completed = 2
			f.wantScanner.submitted = len(publicA) + len(publicB)
			f.wantScanner.observed = f.wantScanner.submitted
			f.wantScanner.reuseFiles = 1
			f.wantScanner.reuseBytes = len(publicA)
		}
	case "scanner-canary":
		f.files[0].body = canary()
		f.wantRead = len(f.files[0].body)
		f.wantScanned = f.wantRead
		f.wantRows = nil
		f.wantTotal = 0
		f.wantStatus = "blocked"
		f.wantCode = "scanner_finding"
		f.wantExit = 1
		f.wantScanner.status = "finding"
		f.wantScanner.code = "finding"
		f.wantScanner.finding = true
		f.wantScanner.submitted = f.wantRead
		f.wantScanner.observed = f.wantRead
	case "scanner-late-pdf":
		f.files = append(f.files, fileInput{name: "b.pdf", body: []byte("%PDF-1.4\npublic\n")})
		f.paths = append(f.paths, "b.pdf")
		f.wantRead = len(publicA) + len(f.files[1].body)
		f.wantScanned = f.wantRead
		f.wantRows = nil
		f.wantTotal = 0
		f.wantStatus = "incomplete"
		f.wantCode = "scanner_incomplete"
		f.wantExit = 2
		f.wantScanner.status = "incomplete"
		f.wantScanner.code = "coverage_mismatch"
		f.wantScanner.expected = 2
		f.wantScanner.attempted = 2
		f.wantScanner.invoked = 2
		f.wantScanner.completed = 1
		f.wantScanner.submitted = f.wantRead
		f.wantScanner.observed = len(publicA)
	case "scanner-wrong-hash":
		f.wrongScannerHash = true
		f.wantRows = nil
		f.wantTotal = 0
		f.wantStatus = "incomplete"
		f.wantCode = "scanner_incomplete"
		f.wantExit = 2
		f.wantScanner.status = "incomplete"
		f.wantScanner.code = "hash_mismatch"
		f.wantScanner.attempted = 0
		f.wantScanner.invoked = 0
		f.wantScanner.completed = 0
		f.wantScanner.submitted = 0
		f.wantScanner.observed = 0
	case "scanner-missing-scope":
		f.missingScannerScope = true
		f.wantRows = nil
		f.wantTotal = 0
		f.wantRead = 0
		f.wantScanned = 0
		f.wantStatus = "incomplete"
		f.wantCode = "invalid"
		f.wantExit = 2
		f.wantScanner = nil
	case "scanner-ambient-ignore":
		f.files[0].body = append(canary(), []byte("# gitleaks:allow\n")...)
		f.wantRead = len(f.files[0].body)
		f.wantScanned = f.wantRead
		f.wantRows = nil
		f.wantTotal = 0
		f.wantStatus = "blocked"
		f.wantCode = "scanner_finding"
		f.wantExit = 1
		f.wantScanner.status = "finding"
		f.wantScanner.code = "finding"
		f.wantScanner.finding = true
		f.wantScanner.submitted = f.wantRead
		f.wantScanner.observed = f.wantRead
	}
	return f
}

func runCases(ctx context.Context, r *qualification, guard, scanner, work string, call transport) {
	for i := range r.Cases {
		if ctx.Err() != nil {
			break
		}
		id := r.Cases[i].ID
		f := baseFixture(id)
		if r.Profile == "files-loopback-fixture/1" && i >= len(coreIDs) {
			f = classifierFixture(id)
		} else if i >= len(coreIDs) {
			f = scannerFixture(id)
		}
		result := runCase(ctx, f, guard, scanner, work, call)
		r.Cases[i] = result
	}
}
func runCase(ctx context.Context, f fixture, guard, scanner, work string, call transport) caseResult {
	r := caseResult{ID: f.id, Status: "unknown", Code: "setup_failed"}
	caseDir, err := os.MkdirTemp(work, "case-")
	if err != nil {
		return r
	}
	root := filepath.Join(caseDir, "root")
	if err = os.Mkdir(root, 0700); err != nil {
		return r
	}
	if f.id == "scanner-ambient-ignore" {
		// A candidate that reads ambient working-tree configuration will either
		// fail or suppress the canary. The scanner recipe must ignore it.
		if err = os.WriteFile(filepath.Join(caseDir, ".gitleaks.toml"), []byte(qualifyAmbientConfig), 0600); err != nil {
			return r
		}
		if err = os.WriteFile(filepath.Join(root, ".gitleaks.toml"), []byte(qualifyAmbientConfig), 0600); err != nil {
			return r
		}
	}
	for _, file := range f.files {
		path := filepath.Join(root, file.name)
		if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return r
		}
		if file.symlink != "" {
			err = os.WriteFile(filepath.Join(root, file.symlink), file.body, 0600)
			if err == nil {
				err = os.Symlink(file.symlink, path)
			}
		} else {
			err = os.WriteFile(path, file.body, 0600)
		}
		if err != nil {
			return r
		}
	}
	scope := map[string]any{"version": "command-guard.scope/1", "requiredLayerIds": []string{"qualify"}, "requiredRuleIds": []string{"synthetic"}}
	if f.missingControl {
		scope["requiredLayerIds"] = []string{"missing"}
	}
	policy := map[string]any{"version": "command-guard.policy/1", "layers": []any{map[string]any{"id": "qualify", "allowedOperations": []string{"read-range", "search-literal"}, "limits": map[string]int{"maxFileBytes": f.maxFile, "maxTotalBytes": qualifyMaxTotal, "maxOutputBytes": f.maxOutput, "maxResults": qualifyMaxResults}, "deniedPathPrefixes": []string{qualifyDeniedPrefix}}}, "rules": []any{map[string]string{"id": "synthetic", "pattern": qualifyRulePattern}}}
	var peer *fixturePeer
	if f.classifier {
		peer, err = startFixturePeer(f)
		if err != nil {
			return r
		}
		defer peer.close()
		policy["classifier"] = classifierPolicyForFixture(peer.endpoint)
	}
	if f.scanner {
		if !f.missingScannerScope {
			scope["requiredScannerIds"] = []string{"gitleaks"}
		}
		sh := sumFilePin(scanner)
		if f.wrongScannerHash {
			sh = strings.Repeat("0", 64)
			if sh == sumFilePin(scanner) {
				sh = strings.Repeat("1", 64)
			}
		}
		policy["scanner"] = map[string]any{"kind": "gitleaks", "profile": qualifyScannerProfile, "executable": scanner, "sha256": sh, "timeoutMillis": qualifyScannerTimeoutMillis, "maxReportBytes": outputCap, "reuseEqualInputs": f.reuse}
	}
	files := make([]map[string]string, 0, len(f.paths))
	for _, p := range f.paths {
		h := sum(f.files[0].body)
		for _, x := range f.files {
			if x.name == p {
				h = sum(x.body)
				break
			}
		}
		if f.wrongHash {
			h = strings.Repeat("0", 64)
		}
		files = append(files, map[string]string{"path": p, "sha256": h})
	}
	request := map[string]any{"version": "command-guard.request/1", "kind": f.kind, "files": files}
	if f.classifier {
		policyRaw, _ := json.Marshal(policy)
		if f.id == "classifier-policy-drift" {
			scope["classifierPolicySHA256"] = sum(policyRaw)
			policy["classifier"] = classifierPolicyForFixture(peer.endpoint)
			policy["classifier"].(map[string]any)["maxTokens"] = fixtureMaxTokens + 1
		} else {
			scope["classifierPolicySHA256"] = sum(policyRaw)
		}
		scope["requiredClassifierIds"] = []string{"semantic-concern"}
		if f.id == "classifier-unpinned-input" {
			scope["permittedClassifierSHA256"] = []string{strings.Repeat("0", 64)}
		} else {
			scope["permittedClassifierSHA256"] = []string{sum(f.files[0].body)}
		}
	}
	if f.kind == "read-range" {
		request["startLine"] = f.start
		request["endLine"] = f.end
	} else {
		request["literal"] = f.literal
	}
	for name, value := range map[string]any{"scope.json": scope, "policy.json": policy, "request.json": request} {
		b, _ := json.Marshal(value)
		if err = os.WriteFile(filepath.Join(caseDir, name), b, 0600); err != nil {
			return r
		}
	}
	args := []string{"inspect", "--root", root, "--scope", filepath.Join(caseDir, "scope.json"), "--policy", filepath.Join(caseDir, "policy.json"), "--request", filepath.Join(caseDir, "request.json"), "--format", f.format}
	o := call(ctx, guard, caseDir, args)
	if !o.started {
		r.Code = "start_failed"
		return r
	}
	if o.code != "" {
		r.Code = o.code
		return r
	}
	if f.maxOutput == 1 {
		if o.exit == 2 && len(o.stdout) == 0 && len(o.stderr) == 0 {
			r.Status = "pass"
			r.Code = "oracle_met"
		} else {
			r.Status = "fail"
			r.Code = "oracle_mismatch"
		}
		return r
	}
	if o.exit != f.wantExit || len(o.stderr) != 0 || len(o.stdout) == 0 {
		r.Status = "fail"
		r.Code = "oracle_mismatch"
		return r
	}
	if f.classifier {
		if !checkClassifierCase(o.stdout, f, peer) {
			r.Status = "fail"
			r.Code = "oracle_mismatch"
			return r
		}
	} else if !checkReport(o.stdout, f) {
		r.Status = "fail"
		r.Code = "oracle_mismatch"
		return r
	}
	r.Status = "pass"
	r.Code = "oracle_met"
	return r
}
func sumFilePin(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return strings.Repeat("0", 64)
	}
	return sum(b)
}

func checkReport(b []byte, f fixture) bool {
	if len(b) == 0 || b[len(b)-1] != '\n' {
		return false
	}
	var raw map[string]json.RawMessage
	if !validJSON(b, &raw) {
		return false
	}
	var got candidateReport
	if f.actualCompact {
		if !exactKeys(raw, compactKeys) {
			return false
		}
		var v string
		if json.Unmarshal(raw["version"], &v) != nil || v != "command-guard.compact-report/1" {
			return false
		}
		if json.Unmarshal(raw["rowFormat"], &v) != nil || v != "file-line-text/1" {
			return false
		}
		var tuples [][]json.RawMessage
		if json.Unmarshal(raw["rows"], &tuples) != nil {
			return false
		}
		rows := make([]fileRow, 0, len(tuples))
		for _, t := range tuples {
			if len(t) != 3 {
				return false
			}
			var x fileRow
			if json.Unmarshal(t[0], &x.File) != nil || json.Unmarshal(t[1], &x.Line) != nil || json.Unmarshal(t[2], &x.Text) != nil {
				return false
			}
			rows = append(rows, x)
		}
		delete(raw, "rowFormat")
		raw["version"], _ = json.Marshal("command-guard.report/1")
		raw["rows"], _ = json.Marshal(rows)
	} else if !exactKeys(raw, ordinaryKeys) {
		return false
	}
	if !strictCandidateShape(raw, f) {
		return false
	}
	clean, _ := json.Marshal(raw)
	if !validJSON(clean, &got) {
		return false
	}
	if got.Version != "command-guard.report/1" || got.Status != f.wantStatus || got.Code != f.wantCode || !got.NoActionAuthority || got.FilesRead != wantFilesRead(f) || got.BytesRead != f.wantRead || got.BytesScanned != f.wantScanned || got.Scope != "selected-files-only" || got.Limitation != "not-proof-of-secret-or-pii-absence" || got.TotalMatches != f.wantTotal || got.Returned != len(f.wantRows) || got.Omitted != 0 || got.Truncated || !reflect.DeepEqual(got.Rows, f.wantRows) {
		return false
	}
	if len(f.wantRows) == 0 {
		if _, ok := raw["rows"]; ok {
			return false
		}
	}
	if f.scanner && !f.missingScannerScope {
		if got.Scanner != "configured-regex-and-gitleaks" || got.ExternalScanner == nil || !checkScanner(*got.ExternalScanner, *f.wantScanner) {
			return false
		}
	} else if got.Scanner != "configured-regex-only" || got.ExternalScanner != nil {
		return false
	}
	return true
}

func strictCandidateShape(raw map[string]json.RawMessage, f fixture) bool {
	if f.scanner && !f.missingScannerScope {
		v, ok := raw["externalScanner"]
		if !ok {
			return false
		}
		var m map[string]json.RawMessage
		if !validJSON(v, &m) {
			return false
		}
		keys := []string{"id", "profile", "status", "code", "expectedFiles", "attemptedFiles", "invokedFiles", "completedFiles", "bytesSubmitted", "bytesObserved", "findings", "reuse"}
		if !allowedKeys(m, keys) {
			return false
		}
		for _, k := range keys[:11] {
			if _, ok := m[k]; !ok {
				return false
			}
		}
		if f.wantScanner.reuse {
			var reuse map[string]json.RawMessage
			if !validJSON(m["reuse"], &reuse) || !allowedKeys(reuse, []string{"profile", "files", "bytes"}) || len(reuse) != 3 {
				return false
			}
		} else if _, ok := m["reuse"]; ok {
			return false
		}
	} else if _, ok := raw["externalScanner"]; ok {
		return false
	}
	if rows, ok := raw["rows"]; ok {
		var a []map[string]json.RawMessage
		if !validJSON(rows, &a) {
			return false
		}
		for _, m := range a {
			if len(m) != 3 || !allowedKeys(m, []string{"file", "line", "text"}) {
				return false
			}
		}
	}
	return true
}
func wantFilesRead(f fixture) int {
	if f.wantRead == 0 && f.wantScanned == 0 && (f.missingControl || f.missingScannerScope || f.id == "denied-prefix" || f.id == "traversal" || f.id == "case-alias" || f.id == "reserved-name") {
		return 0
	}
	if f.id == "symlink" {
		return 0
	}
	if f.id == "file-cap" {
		return 0
	}
	return len(f.files)
}
func checkScanner(g externalReport, w scannerWant) bool {
	if g.ID != "gitleaks" || g.Profile != "gitleaks-stdin-8.30.1" || g.Status != w.status || g.Code != w.code || g.ExpectedFiles != w.expected || g.AttemptedFiles != w.attempted || g.InvokedFiles != w.invoked || g.CompletedFiles != w.completed || g.BytesSubmitted != w.submitted || g.BytesObserved != w.observed {
		return false
	}
	if w.finding {
		if g.Findings < 1 {
			return false
		}
	} else if g.Findings != 0 {
		return false
	}
	if w.reuse {
		return g.Reuse != nil && g.Reuse.Profile == "invocation-exact/1" && g.Reuse.Files == w.reuseFiles && g.Reuse.Bytes == w.reuseBytes
	}
	return g.Reuse == nil
}

var ordinaryKeys = []string{"version", "status", "code", "noActionAuthority", "filesRead", "bytesRead", "bytesScanned", "scope", "scanner", "externalScanner", "limitation", "totalMatches", "returned", "omitted", "truncated", "rows"}
var compactKeys = append(append([]string(nil), ordinaryKeys...), "rowFormat")

func exactKeys(m map[string]json.RawMessage, allow []string) bool {
	if !allowedKeys(m, allow) {
		return false
	}
	for _, k := range []string{"version", "status", "code", "noActionAuthority", "filesRead", "bytesRead", "bytesScanned", "scope", "scanner", "limitation", "totalMatches", "returned", "omitted", "truncated"} {
		if _, ok := m[k]; !ok {
			return false
		}
	}
	return true
}
func allowedKeys(m map[string]json.RawMessage, allow []string) bool {
	allowed := map[string]bool{}
	for _, k := range allow {
		allowed[k] = true
	}
	for k := range m {
		if !allowed[k] {
			return false
		}
	}
	return true
}

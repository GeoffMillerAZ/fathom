// Command command-guard inspects selected files with optional pinned scanner checks.
// Concurrency: each invocation owns its state; no shared mutable state.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	dlp "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/dlp-evaluator"
)

const (
	maxFile   = 4 << 20
	maxTotal  = 64 << 20
	maxOutput = 1 << 20
)

type scope struct {
	Version                   string   `json:"version"`
	RequiredLayerIDs          []string `json:"requiredLayerIds"`
	RequiredRuleIDs           []string `json:"requiredRuleIds"`
	RequiredScannerIDs        []string `json:"requiredScannerIds,omitempty"`
	RequiredGitProfiles       []string `json:"requiredGitProfiles,omitempty"`
	RequiredClassifierIDs     []string `json:"requiredClassifierIds,omitempty"`
	PermittedClassifierSHA256 []string `json:"permittedClassifierSHA256,omitempty"`
	ClassifierPolicySHA256    string   `json:"classifierPolicySHA256,omitempty"`
}
type limits struct {
	MaxFileBytes   int `json:"maxFileBytes"`
	MaxTotalBytes  int `json:"maxTotalBytes"`
	MaxOutputBytes int `json:"maxOutputBytes"`
	MaxResults     int `json:"maxResults"`
}
type layer struct {
	ID                 string   `json:"id"`
	AllowedOperations  []string `json:"allowedOperations"`
	Limits             limits   `json:"limits"`
	DeniedPathPrefixes []string `json:"deniedPathPrefixes"`
}
type rule struct {
	ID      string `json:"id"`
	Pattern string `json:"pattern"`
}
type policy struct {
	Version      string            `json:"version"`
	Layers       []layer           `json:"layers"`
	Rules        []rule            `json:"rules"`
	Scanner      *scannerPolicy    `json:"scanner,omitempty"`
	Git          *gitPolicy        `json:"git,omitempty"`
	Classifier   *classifierPolicy `json:"classifier,omitempty"`
	sourceSHA256 string
}
type fileSpec struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}
type request struct {
	Version   string     `json:"version"`
	Kind      string     `json:"kind"`
	Files     []fileSpec `json:"files"`
	StartLine int        `json:"startLine,omitempty"`
	EndLine   int        `json:"endLine,omitempty"`
	Literal   string     `json:"literal,omitempty"`
}
type row struct {
	File int    `json:"file"`
	Line int    `json:"line"`
	Text string `json:"text"`
}
type report struct {
	Version            string             `json:"version"`
	Status             string             `json:"status"`
	Code               string             `json:"code"`
	NoActionAuthority  bool               `json:"noActionAuthority"`
	FilesRead          int                `json:"filesRead"`
	BytesRead          int                `json:"bytesRead"`
	BytesScanned       int                `json:"bytesScanned"`
	Scope              string             `json:"scope"`
	Scanner            string             `json:"scanner"`
	ExternalScanner    *scannerReceipt    `json:"externalScanner,omitempty"`
	ExternalClassifier *classifierReceipt `json:"externalClassifier,omitempty"`
	Limitation         string             `json:"limitation"`
	TotalMatches       int                `json:"totalMatches"`
	Returned           int                `json:"returned"`
	Omitted            int                `json:"omitted"`
	Truncated          bool               `json:"truncated"`
	Rows               []row              `json:"rows,omitempty"`
	outputCap          int
}

func base() report {
	return report{Version: "command-guard.report/1", Status: "incomplete", Code: "invalid", NoActionAuthority: true, Scope: "selected-files-only", Scanner: "configured-regex-only", Limitation: "not-proof-of-secret-or-pii-absence", outputCap: maxOutput}
}
func validID(s string) bool {
	return regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`).MatchString(s)
}
func validPath(s string) bool {
	if s == "" || len(s) > 1024 || strings.ContainsAny(s, `\:`) || strings.HasPrefix(s, "/") || path.Clean(s) != s {
		return false
	}
	for _, p := range strings.Split(s, "/") {
		if p == "" || p == "." || p == ".." || strings.HasSuffix(p, ".") || strings.HasSuffix(p, " ") {
			return false
		}
		for _, r := range p {
			// Portable names are ASCII until filesystem aliasing is qualified.
			if r < 0x20 || r > 0x7e || strings.ContainsRune(`<>"|?*`, r) {
				return false
			}
		}
		base := strings.ToUpper(strings.SplitN(p, ".", 2)[0])
		if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9' {
			return false
		}
	}
	return true
}
func validText(s string) bool {
	if !utf8.ValidString(s) {
		return false
	}
	for i, r := range s {
		if r == '\r' {
			if i+1 >= len(s) || s[i+1] != '\n' {
				return false
			}
			continue
		}
		if r == '\t' || r == '\n' {
			continue
		}
		if r == 0 || unicode.IsControl(r) || unicode.Is(unicode.Cf, r) || unicode.Is(unicode.Cs, r) || unicode.Is(unicode.Co, r) || unicode.Is(unicode.Zl, r) || unicode.Is(unicode.Zp, r) {
			return false
		}
	}
	return true
}
func ids(got []string, min, max int) (map[string]bool, bool) {
	if len(got) < min || len(got) > max {
		return nil, false
	}
	m := map[string]bool{}
	for _, s := range got {
		if !validID(s) || m[s] {
			return nil, false
		}
		m[s] = true
	}
	return m, true
}
func equalIDs(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for x := range a {
		if !b[x] {
			return false
		}
	}
	return true
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func validate(s scope, p policy, q request) (limits, []string, *dlp.Evaluator, string) {
	if !classifierBuildEnabled && selectedClassifierControls(s, p) {
		return limits{}, nil, nil, "unsupported"
	}
	if !gitControlsMatch(s, p) || !validClassifierControls(s, p) {
		return limits{}, nil, nil, "invalid"
	}
	cap, denied, ev, code := validateControls(s, p, q.Kind)
	if code != "" {
		return cap, denied, ev, code
	}
	if q.Version != "command-guard.request/1" {
		return limits{}, nil, nil, "invalid"
	}
	if q.Kind != "read-range" && q.Kind != "search-literal" || len(q.Files) < 1 || len(q.Files) > 32 {
		return limits{}, nil, nil, "invalid"
	}
	if q.Kind == "read-range" {
		if q.StartLine < 1 || q.StartLine > 1000000 || q.EndLine < q.StartLine || q.EndLine > 1000000 || q.Literal != "" {
			return limits{}, nil, nil, "invalid"
		}
	} else if q.StartLine != 0 || q.EndLine != 0 || q.Literal == "" || len(q.Literal) > 1024 || strings.ContainsAny(q.Literal, "\r\n") || !validText(q.Literal) {
		return limits{}, nil, nil, "invalid"
	}
	seen := map[string]bool{}
	for _, f := range q.Files {
		folded := strings.ToLower(f.Path)
		if !validPath(f.Path) || seen[folded] || len(f.SHA256) != 64 {
			return limits{}, nil, nil, "invalid"
		}
		seen[folded] = true
		for _, ch := range f.SHA256 {
			if ch < '0' || ch > '9' && ch < 'a' || ch > 'f' {
				return limits{}, nil, nil, "invalid"
			}
		}
		if p.Classifier != nil && !classifierPermitted(s, f.SHA256) {
			return limits{}, nil, nil, "invalid"
		}
		for _, d := range denied {
			d = strings.ToLower(d)
			if folded == d || strings.HasPrefix(folded, d+"/") {
				return cap, denied, ev, "denied"
			}
		}
	}
	return cap, denied, ev, ""
}
func validateControls(s scope, p policy, operation string) (limits, []string, *dlp.Evaluator, string) {
	empty := limits{}
	if !validScannerControls(s, p) {
		return empty, nil, nil, "invalid"
	}
	if s.Version != "command-guard.scope/1" || p.Version != "command-guard.policy/1" {
		return empty, nil, nil, "invalid"
	}
	ls, ok := ids(s.RequiredLayerIDs, 1, 16)
	if !ok {
		return empty, nil, nil, "invalid"
	}
	rs, ok := ids(s.RequiredRuleIDs, 1, 64)
	if !ok {
		return empty, nil, nil, "invalid"
	}
	if len(p.Layers) < 1 || len(p.Layers) > 16 || len(p.Rules) < 1 || len(p.Rules) > 64 {
		return empty, nil, nil, "invalid"
	}
	lgot := map[string]bool{}
	denied := []string{}
	var cap limits
	allowed := true
	for i, l := range p.Layers {
		if !validID(l.ID) || lgot[l.ID] {
			return empty, nil, nil, "invalid"
		}
		lgot[l.ID] = true
		if len(l.AllowedOperations) < 1 || len(l.AllowedOperations) > 5 {
			return empty, nil, nil, "invalid"
		}
		ops := map[string]bool{}
		for _, op := range l.AllowedOperations {
			if op != "read-range" && op != "search-literal" && op != "git-status" && op != "git-object-diff" && op != "git-history" || ops[op] {
				return empty, nil, nil, "invalid"
			}
			ops[op] = true
		}
		if !ops[operation] {
			allowed = false
		}
		c := l.Limits
		if c.MaxFileBytes < 1 || c.MaxFileBytes > maxFile || c.MaxTotalBytes < 1 || c.MaxTotalBytes > maxTotal || c.MaxOutputBytes < 1 || c.MaxOutputBytes > maxOutput || c.MaxResults < 1 || c.MaxResults > 1000 {
			return empty, nil, nil, "invalid"
		}
		if i == 0 {
			cap = c
		} else {
			cap.MaxFileBytes = min(cap.MaxFileBytes, c.MaxFileBytes)
			cap.MaxTotalBytes = min(cap.MaxTotalBytes, c.MaxTotalBytes)
			cap.MaxOutputBytes = min(cap.MaxOutputBytes, c.MaxOutputBytes)
			cap.MaxResults = min(cap.MaxResults, c.MaxResults)
		}
		if len(l.DeniedPathPrefixes) > 64 {
			return empty, nil, nil, "invalid"
		}
		for _, d := range l.DeniedPathPrefixes {
			if !validPath(d) {
				return empty, nil, nil, "invalid"
			}
			denied = append(denied, d)
		}
	}
	if !equalIDs(ls, lgot) {
		return empty, nil, nil, "invalid"
	}
	rgot := map[string]bool{}
	rules := make([]dlp.DLPRule, 0, len(p.Rules))
	for _, r := range p.Rules {
		if !validID(r.ID) || rgot[r.ID] || len(r.Pattern) < 1 || len(r.Pattern) > 2048 || !validText(r.Pattern) {
			return empty, nil, nil, "invalid"
		}
		rgot[r.ID] = true
		rules = append(rules, dlp.DLPRule{ID: r.ID, Pattern: r.Pattern, RiskBand: dlp.RiskHigh})
	}
	if !equalIDs(rs, rgot) {
		return empty, nil, nil, "invalid"
	}
	ev, e := dlp.NewEvaluator(rules)
	if e != nil {
		return empty, nil, nil, "invalid"
	}
	if !allowed {
		return cap, denied, ev, "denied"
	}
	return cap, denied, ev, ""
}
func run(rootPath string, s scope, p policy, q request, format string) report {
	return runWithScanner(rootPath, s, p, q, format, scannerCommand)
}
func runWithScanner(rootPath string, s scope, p policy, q request, format string, invoke scannerRunner) report {
	r := base()
	cap, _, ev, code := validate(s, p, q)
	if cap.MaxOutputBytes > 0 {
		r.outputCap = cap.MaxOutputBytes
	}
	if code != "" {
		r.Code = code
		if code == "denied" {
			r.Status = "blocked"
		}
		return r
	}
	if p.Scanner != nil {
		r.Scanner = "configured-regex-and-gitleaks"
		r.ExternalScanner = &scannerReceipt{ID: "gitleaks", Profile: "gitleaks-stdin-8.30.1", Status: "not-invoked", Code: "not_invoked", ExpectedFiles: len(q.Files)}
	}
	if p.Classifier != nil {
		r.ExternalClassifier = newClassifierReceipt(len(q.Files))
	}
	if !filepath.IsAbs(rootPath) {
		return r
	}
	root, e := os.OpenRoot(rootPath)
	if e != nil {
		return r
	}
	defer root.Close()
	data := make([][]byte, 0, len(q.Files))
	found := false
	for _, f := range q.Files {
		remaining := cap.MaxTotalBytes - r.BytesRead
		if remaining < 0 {
			return r
		}
		readCap := min(cap.MaxFileBytes, remaining)
		b, read, e := readSelected(root, f.Path, readCap)
		r.BytesRead += read
		if e != nil {
			return r
		}
		r.FilesRead++
		if !validText(string(b)) {
			return r
		}
		sum := sha256.Sum256(b)
		if hex.EncodeToString(sum[:]) != f.SHA256 {
			return r
		}
		if ev.Evaluate(string(b)).MaxRisk == dlp.RiskHigh {
			found = true
		}
		r.BytesScanned += len(b)
		data = append(data, b)
	}
	if found {
		r.Status = "blocked"
		r.Code = "rule"
		return r
	}
	if p.Scanner != nil {
		if !scanSelectedWithRunner(*p.Scanner, data, r.ExternalScanner, invoke) {
			r.Code = "scanner_incomplete"
			return r
		}
		if r.ExternalScanner.Findings > 0 {
			r.Status = "blocked"
			r.Code = "scanner_finding"
			return r
		}
	}
	if p.Classifier != nil {
		status := classifySelected(*p.Classifier, data, r.ExternalClassifier)
		if status != "no-concern" {
			if status == "concern" {
				r.Status = "blocked"
				r.Code = "classifier_concern"
			} else {
				r.Code = "classifier_incomplete"
			}
			return r
		}
	}
	r.Status = "no-findings"
	r.Code = "ok"
	projectionBytes := 0
	projectionFull := false
	for i, b := range data {
		lines := strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n")
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		for n, t := range lines {
			ln := n + 1
			match := q.Kind == "read-range" && ln >= q.StartLine && ln <= q.EndLine || q.Kind == "search-literal" && strings.Contains(t, q.Literal)
			if match {
				r.TotalMatches++
				if !projectionFull && len(r.Rows) < cap.MaxResults && projectionBytes+len(t) <= cap.MaxOutputBytes {
					r.Rows = append(r.Rows, row{File: i, Line: ln, Text: t})
					projectionBytes += len(t)
				} else {
					projectionFull = true
				}
			}
		}
	}
	// Encoding only a bounded candidate prefix keeps large lines and many
	// matches from causing repeated multi-megabyte marshaling.
	candidates := r.Rows
	lo, hi := 0, len(candidates)
	for lo < hi {
		mid := lo + (hi-lo+1)/2
		r.Rows = candidates[:mid]
		r.Returned = mid
		r.Omitted = r.TotalMatches - mid
		r.Truncated = r.Omitted > 0
		if len(encode(r, format))+1 <= cap.MaxOutputBytes {
			lo = mid
		} else {
			hi = mid - 1
		}
	}
	r.Rows = candidates[:lo]
	r.Returned = lo
	r.Omitted = r.TotalMatches - r.Returned
	r.Truncated = r.Omitted > 0
	b := encode(r, format)
	if len(b)+1 > cap.MaxOutputBytes {
		if p.Classifier != nil {
			r.Status = "incomplete"
			r.Code = "output_limit"
			r.Rows = nil
			r.Returned = 0
			r.Omitted = r.TotalMatches
			r.Truncated = r.Omitted > 0
		} else {
			r = base()
			r.outputCap = cap.MaxOutputBytes
			r.Code = "output_limit"
		}
	}
	return r
}

// readSelected pins each opened component to the identity observed by Lstat.
// Opening only a single child name at each step prevents a renamed ancestor
// from redirecting a later multi-component open into a denied in-root path.
func readSelected(root *os.Root, name string, cap int) ([]byte, int, error) {
	if cap < 0 {
		return nil, 0, os.ErrInvalid
	}
	parts := strings.Split(name, "/")
	current := root
	defer func() {
		if current != root {
			current.Close()
		}
	}()
	for _, part := range parts[:len(parts)-1] {
		before, e := current.Lstat(part)
		if e != nil || !before.IsDir() || before.Mode()&os.ModeSymlink != 0 {
			return nil, 0, os.ErrInvalid
		}
		child, e := current.OpenRoot(part)
		if e != nil {
			return nil, 0, e
		}
		after, e := child.Stat(".")
		if e != nil || !os.SameFile(before, after) {
			child.Close()
			return nil, 0, os.ErrInvalid
		}
		if current != root {
			current.Close()
		}
		current = child
	}
	last := parts[len(parts)-1]
	before, e := current.Lstat(last)
	if e != nil || !before.Mode().IsRegular() {
		return nil, 0, os.ErrInvalid
	}
	h, e := current.OpenFile(last, os.O_RDONLY|nonblock, 0)
	if e != nil {
		return nil, 0, e
	}
	defer h.Close()
	after, e := h.Stat()
	if e != nil || !after.Mode().IsRegular() || !os.SameFile(before, after) {
		return nil, 0, os.ErrInvalid
	}
	b, e := io.ReadAll(io.LimitReader(h, int64(cap+1)))
	if e != nil || len(b) > cap {
		return nil, len(b), os.ErrInvalid
	}
	return b, len(b), nil
}
func execute(args []string, w io.Writer) int {
	if len(args) == 1 && (args[0] == "help" || args[0] == "--help") || len(args) == 2 && (args[0] == "inspect" || args[0] == "git-inspect") && (args[1] == "--help" || args[1] == "-h") {
		usage := "command-guard inspect --root ABS --scope FILE --policy FILE --request FILE --format json|text|compact\n" +
			"command-guard git-inspect --root ABS --scope FILE --policy FILE --request FILE --format json|text|compact\n" +
			"Git requests: git-status, git-object-diff, git-history (registered protected metadata required).\n" + classifierCapabilityLine
		if n, e := io.WriteString(w, usage); e != nil || n != len(usage) {
			return 2
		}
		return 0
	}
	if len(args) > 0 && args[0] == "git-inspect" {
		return executeGit(args[1:], w)
	}
	r := base()
	if len(args) < 1 || args[0] != "inspect" {
		return emit(w, r, "json", 2)
	}
	fs := flag.NewFlagSet("inspect", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var root, sp, pp, qp, format string
	fs.StringVar(&root, "root", "", "")
	fs.StringVar(&sp, "scope", "", "")
	fs.StringVar(&pp, "policy", "", "")
	fs.StringVar(&qp, "request", "", "")
	fs.StringVar(&format, "format", "json", "")
	if fs.Parse(args[1:]) != nil || len(fs.Args()) != 0 || root == "" || sp == "" || pp == "" || qp == "" || format != "json" && format != "text" && format != "compact" {
		return emit(w, r, "json", 2)
	}
	var s scope
	var p policy
	var q request
	if strictFile(sp, &s) != nil || strictFile(pp, &p) != nil || strictFile(qp, &q) != nil {
		return emit(w, r, format, 2)
	}
	r = run(root, s, p, q, format)
	code := 2
	if r.Status == "no-findings" {
		code = 0
	} else if r.Status == "blocked" {
		code = 1
	}
	return emit(w, r, format, code)
}
func emit(w io.Writer, r report, format string, code int) int {
	b := encode(r, format)
	b = append(b, '\n')
	if len(b) > r.outputCap {
		return 2
	}
	if n, e := w.Write(b); e != nil || n != len(b) {
		return 2
	}
	return code
}
func encode(r report, format string) []byte {
	if format == "text" {
		b, _ := json.MarshalIndent(r, "", "  ")
		return b
	}
	b, _ := json.Marshal(r)
	if format == "compact" {
		return compactInspect(r, b)
	}
	return b
}
func main() { os.Exit(execute(os.Args[1:], os.Stdout)) }

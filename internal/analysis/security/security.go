// Package security provides static-analysis scanners that detect security
// anti-patterns in source code: secrets, shell-out calls, and unsafe HTTP
// client usage.
//
// Go files are analysed via go/ast; other languages are covered by regex
// applied to the raw source text.
//
// All exported functions are pure, deterministic, and safe for concurrent use.
// Each scanner returns a []Finding sorted by (File, Line, Rule).
package security

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"math"
	"regexp"
	"sort"
	"strings"
)

// Finding is the result of a single scanner hit.
type Finding struct {
	File    string
	Line    int
	Rule    string
	Message string
}

// sortFindings sorts findings by (File, Line, Rule) for determinism.
func sortFindings(fs []Finding) {
	sort.Slice(fs, func(i, j int) bool {
		if fs[i].File != fs[j].File {
			return fs[i].File < fs[j].File
		}
		if fs[i].Line != fs[j].Line {
			return fs[i].Line < fs[j].Line
		}
		return fs[i].Rule < fs[j].Rule
	})
}

// parseFile parses Go source and returns (fset, *ast.File).  On error both
// are still returned (fset is usable, ast.File is nil).
func parseFile(path string, src interface{}) (*token.FileSet, *ast.File) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, path, src, 0)
	if err != nil {
		return fset, nil
	}
	return fset, f
}

// lineNumber returns the 1-based line number for the byte offset pos within
// the text.  Falls back to 0 if src is not a string or []byte.
func lineNumber(src interface{}, pos int) int {
	var text string
	switch v := src.(type) {
	case string:
		text = v
	case []byte:
		text = string(v)
	default:
		return 0
	}
	if pos < 0 || pos > len(text) {
		return 0
	}
	return strings.Count(text[:pos], "\n") + 1
}

// ─── Entropy helper ──────────────────────────────────────────────────────────

// ShannonEntropy computes the Shannon entropy (bits) of the given string.
// An empty string returns 0.
func ShannonEntropy(s string) float64 {
	if len(s) == 0 {
		return 0
	}
	freq := make(map[rune]int)
	for _, c := range s {
		freq[c]++
	}
	n := float64(len([]rune(s)))
	var h float64
	for _, count := range freq {
		p := float64(count) / n
		h -= p * math.Log2(p)
	}
	return h
}

// entropyThreshold is the minimum Shannon entropy for a string to be considered
// high-entropy (potential secret).  Empirically, random base64 tokens score
// ≥ 4.5 bits; English prose and short slugs score well below 3.5 bits.
const entropyThreshold = 4.5

// minSecretLen is the minimum length for a high-entropy string to be flagged.
const minSecretLen = 20

// ─── SecretScan ──────────────────────────────────────────────────────────────

// Known-pattern regexes compiled once at package init.
var (
	// JWT: header starts with eyJ (base64 of '{"')
	reJWT = regexp.MustCompile(`eyJ[A-Za-z0-9\-_]+\.[A-Za-z0-9\-_]+\.[A-Za-z0-9\-_]+`)
	// AWS access key IDs
	reAWSKey = regexp.MustCompile(`AKIA[0-9A-Z]{16}`)
	// PEM private key headers
	rePEMHeader = regexp.MustCompile(`-----BEGIN (?:RSA |EC |DSA |OPENSSH )?PRIVATE KEY-----`)
	// Generic assignment patterns: api_key=VALUE, password=VALUE, secret=VALUE
	reGenericSecret = regexp.MustCompile(`(?i)(?:api[_\-]?key|password|passwd|secret|token|credentials?)\s*[=:]\s*["']?([A-Za-z0-9+/\-_\.]{8,})["']?`)
)

// SecretScan reports strings in src that match known secret patterns or exhibit
// high Shannon entropy in a credential-like context.
//
// For Go source, string literals are extracted via AST. For other file types
// the raw src text is scanned with regexes.
func SecretScan(path string, src interface{}) []Finding {
	var findings []Finding

	fset, f := parseFile(path, src)
	if f != nil {
		// AST path: scan string literals.
		ast.Inspect(f, func(n ast.Node) bool {
			bl, ok := n.(*ast.BasicLit)
			if !ok || bl.Kind != token.STRING {
				return true
			}
			val := strings.Trim(bl.Value, "`\"")
			pos := fset.Position(bl.Pos())
			findings = append(findings, checkSecretValue(path, pos.Line, val)...)
			return true
		})
		// Also scan assignment value expressions for generic patterns.
		ast.Inspect(f, func(n ast.Node) bool {
			as, ok := n.(*ast.AssignStmt)
			if !ok {
				return true
			}
			// Check LHS idents for secret-like names.
			for i, lhs := range as.Lhs {
				ident, ok := lhs.(*ast.Ident)
				if !ok {
					continue
				}
				if !isSecretIdent(ident.Name) {
					continue
				}
				if i >= len(as.Rhs) {
					continue
				}
				rbl, ok := as.Rhs[i].(*ast.BasicLit)
				if !ok || rbl.Kind != token.STRING {
					continue
				}
				val := strings.Trim(rbl.Value, "`\"")
				if len(val) >= 4 {
					pos := fset.Position(rbl.Pos())
					findings = append(findings, Finding{
						File:    path,
						Line:    pos.Line,
						Rule:    "secret-assignment",
						Message: fmt.Sprintf("credential-named variable %q assigned a literal string", ident.Name),
					})
				}
			}
			return true
		})
	} else {
		// Regex path for non-Go or unparseable source.
		srcStr := toStr(src)
		findings = append(findings, regexSecretScan(path, srcStr)...)
	}

	// Dedup by (File, Line, Rule).
	findings = dedup(findings)
	sortFindings(findings)
	return findings
}

// checkSecretValue applies pattern + entropy checks to a single string value.
func checkSecretValue(path string, line int, val string) []Finding {
	var fs []Finding
	if reJWT.MatchString(val) {
		fs = append(fs, Finding{File: path, Line: line, Rule: "secret-jwt", Message: "JWT token literal detected"})
	}
	if reAWSKey.MatchString(val) {
		fs = append(fs, Finding{File: path, Line: line, Rule: "secret-aws-key", Message: "AWS access key ID detected"})
	}
	if rePEMHeader.MatchString(val) {
		fs = append(fs, Finding{File: path, Line: line, Rule: "secret-private-key", Message: "PEM private key header detected"})
	}
	// High-entropy check.
	if len(val) >= minSecretLen && ShannonEntropy(val) >= entropyThreshold {
		fs = append(fs, Finding{
			File:    path,
			Line:    line,
			Rule:    "secret-high-entropy",
			Message: fmt.Sprintf("high-entropy string (%.2f bits) of length %d", ShannonEntropy(val), len(val)),
		})
	}
	return fs
}

// isSecretIdent returns true if the identifier name looks credential-like.
func isSecretIdent(name string) bool {
	lower := strings.ToLower(name)
	keywords := []string{"password", "passwd", "secret", "apikey", "api_key", "token", "credential", "auth"}
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// regexSecretScan applies all secret patterns to raw text line by line.
func regexSecretScan(path, src string) []Finding {
	var findings []Finding
	lines := strings.Split(src, "\n")
	for i, line := range lines {
		lineNo := i + 1
		if reJWT.MatchString(line) {
			findings = append(findings, Finding{File: path, Line: lineNo, Rule: "secret-jwt", Message: "JWT token literal detected"})
		}
		if reAWSKey.MatchString(line) {
			findings = append(findings, Finding{File: path, Line: lineNo, Rule: "secret-aws-key", Message: "AWS access key ID detected"})
		}
		if rePEMHeader.MatchString(line) {
			findings = append(findings, Finding{File: path, Line: lineNo, Rule: "secret-private-key", Message: "PEM private key header detected"})
		}
		if m := reGenericSecret.FindStringSubmatch(line); m != nil && len(m) > 1 {
			val := m[1]
			if len(val) >= 4 {
				findings = append(findings, Finding{
					File:    path,
					Line:    lineNo,
					Rule:    "secret-assignment",
					Message: fmt.Sprintf("credential pattern %q found", strings.TrimSpace(line)),
				})
			}
		}
		// High entropy on non-trivial tokens (split on whitespace/quotes).
		for _, tok := range tokenize(line) {
			if len(tok) >= minSecretLen && ShannonEntropy(tok) >= entropyThreshold {
				findings = append(findings, Finding{
					File:    path,
					Line:    lineNo,
					Rule:    "secret-high-entropy",
					Message: fmt.Sprintf("high-entropy string (%.2f bits)", ShannonEntropy(tok)),
				})
				break // one per line
			}
		}
	}
	return findings
}

// tokenize splits a line into candidate secret tokens.
var reToken = regexp.MustCompile(`[A-Za-z0-9+/\-_\.=]{20,}`)

func tokenize(line string) []string {
	return reToken.FindAllString(line, -1)
}

// toStr converts src to string.
func toStr(src interface{}) string {
	switch v := src.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	}
	return ""
}

// dedup removes duplicate findings (same File/Line/Rule).
func dedup(fs []Finding) []Finding {
	seen := make(map[string]bool, len(fs))
	out := fs[:0]
	for _, f := range fs {
		key := fmt.Sprintf("%s\x00%d\x00%s", f.File, f.Line, f.Rule)
		if !seen[key] {
			seen[key] = true
			out = append(out, f)
		}
	}
	return out
}

// ─── ShellOut ────────────────────────────────────────────────────────────────

// Go AST selectors for os/exec.
var shellGoSelectors = map[string]bool{
	"Command":        true,
	"CommandContext": true,
}

// Regex for non-Go languages.
var (
	reSubprocess   = regexp.MustCompile(`(?m)\bsubprocess\s*\.\s*(?:call|run|Popen|check_output|check_call)\s*\(`)
	reChildProcess = regexp.MustCompile(`(?m)\brequire\s*\(\s*['"]child_process['"]\s*\)|\bspawn\s*\(|\.exec\s*\(|\.execSync\s*\(|\.spawn\s*\(`)
)

// ShellOut finds calls to exec.Command / exec.CommandContext in Go source and
// subprocess/child_process patterns in other languages.
func ShellOut(path string, src interface{}) []Finding {
	var findings []Finding

	fset, f := parseFile(path, src)
	if f != nil {
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if !shellGoSelectors[sel.Sel.Name] {
				return true
			}
			// Check that the receiver is an "exec" identifier (os/exec import
			// alias is typically "exec").
			pkg, ok := sel.X.(*ast.Ident)
			if !ok || pkg.Name != "exec" {
				return true
			}
			pos := fset.Position(call.Pos())
			findings = append(findings, Finding{
				File:    path,
				Line:    pos.Line,
				Rule:    "shell-out",
				Message: fmt.Sprintf("exec.%s() — shell-out call site", sel.Sel.Name),
			})
			return true
		})
	}

	// Always apply cross-lang regex to the raw text too (catches shell-outs in
	// non-Go files and also in template strings inside Go).
	srcStr := toStr(src)
	if srcStr == "" {
		// src was already consumed as ast — re-read not possible, skip regex.
	}
	for _, re := range []*regexp.Regexp{reSubprocess, reChildProcess} {
		for _, loc := range re.FindAllStringIndex(srcStr, -1) {
			line := lineNumber(srcStr, loc[0])
			snippet := strings.TrimSpace(srcStr[loc[0]:loc[1]])
			if len(snippet) > 80 {
				snippet = snippet[:80]
			}
			findings = append(findings, Finding{
				File:    path,
				Line:    line,
				Rule:    "shell-out",
				Message: fmt.Sprintf("shell-out pattern: %s", snippet),
			})
		}
	}

	findings = dedup(findings)
	sortFindings(findings)
	return findings
}

// ─── HTTPClientCalls ─────────────────────────────────────────────────────────

// Go selectors for net/http.
var httpGoSelectors = map[string]bool{
	"Get":      true,
	"Post":     true,
	"PostForm": true,
	"Head":     true,
}

// Regex for non-Go HTTP client patterns.
var (
	reFetch       = regexp.MustCompile(`(?m)\bfetch\s*\(`)
	reRequestsGet = regexp.MustCompile(`(?m)\brequests\s*\.\s*(?:get|post|put|delete|patch|head|request)\s*\(`)
	reAxios       = regexp.MustCompile(`(?m)\baxios\s*\.\s*(?:get|post|put|delete|patch|head|request)\s*\(`)
)

// HTTPClientCalls finds HTTP client call sites in Go source and common
// HTTP library patterns in other languages.
func HTTPClientCalls(path string, src interface{}) []Finding {
	var findings []Finding

	fset, f := parseFile(path, src)
	if f != nil {
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			pos := fset.Position(call.Pos())

			switch fun := call.Fun.(type) {
			case *ast.SelectorExpr:
				// Catch .Do() on any receiver (http.Client variable, literal, etc.).
				if fun.Sel.Name == "Do" {
					findings = append(findings, Finding{
						File:    path,
						Line:    pos.Line,
						Rule:    "http-client",
						Message: "HTTP client .Do() call",
					})
					return true
				}
				pkg, ok := fun.X.(*ast.Ident)
				if !ok {
					return true
				}
				if pkg.Name == "http" && httpGoSelectors[fun.Sel.Name] {
					findings = append(findings, Finding{
						File:    path,
						Line:    pos.Line,
						Rule:    "http-client",
						Message: fmt.Sprintf("http.%s() call", fun.Sel.Name),
					})
				}
			}
			return true
		})
	}

	// Cross-lang regex scan.
	srcStr := toStr(src)
	for _, re := range []*regexp.Regexp{reFetch, reRequestsGet, reAxios} {
		for _, loc := range re.FindAllStringIndex(srcStr, -1) {
			line := lineNumber(srcStr, loc[0])
			snippet := strings.TrimSpace(srcStr[loc[0]:loc[1]])
			if len(snippet) > 60 {
				snippet = snippet[:60]
			}
			findings = append(findings, Finding{
				File:    path,
				Line:    line,
				Rule:    "http-client",
				Message: fmt.Sprintf("HTTP client call: %s", snippet),
			})
		}
	}

	findings = dedup(findings)
	sortFindings(findings)
	return findings
}

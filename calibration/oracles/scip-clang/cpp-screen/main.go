// cpp-screen measures parser syntax nodes over a reading's exact C++ tenancy.
// It is a bounded diagnostic, not an oracle or accuracy reading.
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"hash"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"runtime/debug"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/extractor/tsengine"
)

const (
	maxJSONBytes   = 256 << 20
	maxFiles       = 20000
	maxSourceBytes = 16 << 20
	maxTotalBytes  = 512 << 20
	maxJSONTokens  = 2_000_000
)

type limits struct {
	JSONBytes   int64
	Files       int
	SourceBytes int64
	TotalBytes  int64
}
type config struct {
	Subject, SubjectName, Reading, Unmatched, Out string
	Limits                                        *limits // tests inject small bounds; nil uses production caps
	ProductRevision                               string  // tests supply a synthetic identity; production reads build VCS
	SubjectRevision                               string  // tests supply a synthetic pin; production reads checkout HEAD
	Candidate                                     bool
	CandidateLock, CandidateModule                string
	ProductRoot                                   string // tests supply an isolated Git repo; production uses this checkout
}

// candidatePaths is frozen to this C++ audit, not a reusable allowance for
// later product or build changes. Recorded screens are built directly with
// scripts/fathom-go.sh, never through Taskfile build targets.
var candidatePaths = map[string]bool{
	"Taskfile.yml": true,
	"calibration/oracles/scip-clang/cpp-audit.py":                 true,
	"calibration/oracles/scip-clang/cpp-audit_test.py":            true,
	"calibration/oracles/scip-clang/build-cpp-candidates.py":      true,
	"calibration/oracles/scip-clang/build-cpp-candidates_test.py": true,
	"cmd/indexbench/langretime.go":                                true,
	"cmd/indexbench/langretime_load_test.go":                      true,
	"docs/assessments/2026-10-26-core-t1-cpp-ceiling.md":          true,
	"docs/assessments/README.md":                                  true,
	"docs/plans/language-breadth.md":                              true,
	"docs/bench/cpp-historical-holdout.yaml":                      true,
	"calibration/corpus.d/cpp.yaml":                               true,
	".fathom/observations/O1509.md":                               true,

	"internal/extractor/tsengine/grammars/grammars.lock.json":            true,
	"internal/extractor/tsengine/grammars/cpp.wasm.gz":                   true,
	"internal/extractor/tsengine/grammars/PROVENANCE.md":                 true,
	"calibration/oracles/scip-clang/cpp-screen/main.go":                  true,
	"calibration/oracles/scip-clang/cpp-screen/main_test.go":             true,
	"calibration/oracles/scip-clang/cpp-screen/README.md":                true,
	"calibration/oracles/scip-clang/cpp-screen/parser_audit.go":          true,
	"calibration/oracles/scip-clang/cpp-screen/parser_audit_disabled.go": true,
	"calibration/oracles/scip-clang/cpp-screen/parser_audit_test.go":     true,
	"calibration/oracles/scip-clang/build-willbooster.py":                true,
	"calibration/oracles/scip-clang/build-willbooster_test.py":           true,
	"calibration/oracles/scip-clang/cpp-report.py":                       true,
	"calibration/oracles/scip-clang/cpp-report_test.py":                  true,
	"internal/extractor/tsengine/grammar_audit.go":                       true,
	"internal/extractor/tsengine/grammar_audit_test.go":                  true,
}

func gitBytes(root string, args ...string) ([]byte, error) {
	return exec.Command("git", append([]string{"-C", root}, args...)...).Output()
}
func candidateDiffAllowed(root, baseline, candidate string) error {
	return candidateDiffAllowedForMode(root, baseline, candidate, false)
}

func candidateDiffAllowedForMode(root, baseline, candidate string, external bool) error {
	if !fullSHA(baseline) || !fullSHA(candidate) || baseline == candidate {
		return errors.New("candidate_revisions")
	}
	head, err := git(root, "rev-parse", "HEAD")
	if err != nil || head != candidate {
		return errors.New("candidate_stale")
	}
	dirty, err := git(root, "status", "--porcelain", "--untracked-files=all")
	if err != nil || dirty != "" {
		return errors.New("candidate_dirty")
	}
	if err := exec.Command("git", "-C", root, "merge-base", "--is-ancestor", baseline, candidate).Run(); err != nil {
		return errors.New("candidate_not_descendant")
	}
	raw, err := gitBytes(root, "diff", "--name-only", "--no-renames", "-z", baseline, candidate, "--")
	if err != nil || len(raw) == 0 {
		return errors.New("candidate_no_diff")
	}
	changed := map[string]bool{}
	for _, name := range bytes.Split(bytes.TrimSuffix(raw, []byte{0}), []byte{0}) {
		p := string(name)
		if !candidatePaths[p] && !(strings.HasPrefix(p, "docs/bench/readings/cpp-audit-2026-10-26/") && strings.HasSuffix(p, ".json")) {
			return errors.New("candidate_unrelated_change")
		}
		if external && (p == "internal/extractor/tsengine/grammars/grammars.lock.json" || p == "internal/extractor/tsengine/grammars/cpp.wasm.gz") {
			return errors.New("candidate_embedded_grammar_change")
		}
		if p == "internal/extractor/tsengine/grammar_audit.go" || p == "internal/extractor/tsengine/grammar_audit_test.go" {
			content, err := gitBytes(root, "show", candidate+":"+p)
			if err != nil || !bytes.HasPrefix(content, []byte("//go:build fathom_grammar_audit\n")) {
				return errors.New("candidate_audit_tag")
			}
		}
		changed[p] = true
	}
	lockPath := "internal/extractor/tsengine/grammars/grammars.lock.json"
	blobPath := "internal/extractor/tsengine/grammars/cpp.wasm.gz"
	if changed[lockPath] != changed[blobPath] {
		return errors.New("candidate_lock_blob_pair")
	}
	if changed[lockPath] {
		old, err := gitBytes(root, "show", baseline+":"+lockPath)
		if err != nil {
			return errors.New("candidate_old_lock")
		}
		newLock, err := gitBytes(root, "show", candidate+":"+lockPath)
		if err != nil {
			return errors.New("candidate_new_lock")
		}
		if err := onlyCppLockChanged(old, newLock); err != nil {
			return err
		}
	}
	return nil
}

func onlyCppLockChanged(old, newLock []byte) error {
	decode := func(b []byte) (map[string]any, map[string]map[string]any, error) {
		var whole map[string]any
		if err := json.Unmarshal(b, &whole); err != nil {
			return nil, nil, err
		}
		rows, ok := whole["grammars"].([]any)
		if !ok {
			return nil, nil, errors.New("candidate_lock_shape")
		}
		delete(whole, "grammars")
		byBlob := map[string]map[string]any{}
		for _, v := range rows {
			row, ok := v.(map[string]any)
			if !ok {
				return nil, nil, errors.New("candidate_lock_shape")
			}
			blob, ok := row["blob"].(string)
			if !ok || blob == "" || byBlob[blob] != nil {
				return nil, nil, errors.New("candidate_lock_shape")
			}
			byBlob[blob] = row
		}
		if byBlob["cpp"] == nil {
			return nil, nil, errors.New("candidate_missing_cpp")
		}
		return whole, byBlob, nil
	}
	oldTop, oldRows, err := decode(old)
	if err != nil {
		return errors.New("candidate_old_lock_invalid")
	}
	newTop, newRows, err := decode(newLock)
	if err != nil {
		return errors.New("candidate_new_lock_invalid")
	}
	if !reflect.DeepEqual(oldTop, newTop) || len(oldRows) != len(newRows) {
		return errors.New("candidate_other_lock_change")
	}
	for blob, row := range oldRows {
		if blob != "cpp" && !reflect.DeepEqual(row, newRows[blob]) {
			return errors.New("candidate_other_grammar")
		}
	}
	if reflect.DeepEqual(oldRows["cpp"], newRows["cpp"]) {
		return errors.New("candidate_cpp_lock_unchanged")
	}
	return nil
}

type partition struct {
	Error   int `json:"error_containing"`
	Missing int `json:"missing_containing"`
	Both    int `json:"both"`
	OnlyE   int `json:"error_only"`
	OnlyM   int `json:"missing_only"`
	Union   int `json:"union"`
	Faults  int `json:"faults"`
}
type receipt struct {
	Status                    string    `json:"status"`
	Mode                      string    `json:"mode,omitempty"`
	BaselineRevision          string    `json:"baseline_revision,omitempty"`
	FailureCode               string    `json:"failure_code,omitempty"`
	CountsComplete            bool      `json:"counts_complete"`
	Files                     int       `json:"files"`
	Bytes                     int64     `json:"bytes"`
	Written                   partition `json:"written"`
	Selected                  partition `json:"selected"`
	ReadingSHA256             string    `json:"reading_sha256,omitempty"`
	UnmatchedSHA256           string    `json:"unmatched_sha256,omitempty"`
	PopulationSHA256          string    `json:"population_sha256,omitempty"`
	SourceSHA256              string    `json:"source_sha256,omitempty"`
	ProductRevision           string    `json:"product_revision,omitempty"`
	ProductBinarySHA256       string    `json:"product_binary_sha256,omitempty"`
	GrammarBlobSHA256         string    `json:"grammar_blob_sha256,omitempty"`
	GrammarEntryPoint         string    `json:"grammar_entry_point,omitempty"`
	GrammarCommit             string    `json:"grammar_commit,omitempty"`
	AuditGrammarVersion       string    `json:"audit_grammar_version,omitempty"`
	AuditRuntimeVersion       string    `json:"audit_runtime_version,omitempty"`
	AuditGrammarArchiveSHA256 string    `json:"audit_grammar_archive_sha256,omitempty"`
	AuditRuntimeArchiveSHA256 string    `json:"audit_runtime_archive_sha256,omitempty"`
	Preprocess                bool      `json:"preprocess"`
	PreprocessSelected        int       `json:"preprocess_selected"`
}

type reading struct {
	Dialect      string `json:"dialect"`
	FathomCommit string `json:"fathom_commit"`
	Grammar      struct {
		Blob       string `json:"blob"`
		BlobSHA256 string `json:"blob_sha256"`
	} `json:"grammar"`
	Subjects []struct {
		Name  string `json:"name"`
		SHA   string `json:"sha"`
		Files int    `json:"files"`
		Bytes int64  `json:"bytes"`
	} `json:"subjects"`
}
type unmatched struct {
	TenancyFiles []string `json:"tenancy_files"`
}

func bounds(c config) limits {
	if c.Limits != nil {
		return *c.Limits
	}
	return limits{maxJSONBytes, maxFiles, maxSourceBytes, maxTotalBytes}
}
func digest(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
func fullSHA(s string) bool {
	if len(s) != 40 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}
func sortStrings(s []string) { sort.Strings(s) }
func failure(r receipt, code string) (receipt, error) {
	r.Status = "failed"
	r.FailureCode = code
	r.CountsComplete = false
	return r, errors.New(code)
}

type parserFailure struct {
	Code, Phase string
	Ordinal     int
	PeakPages   uint32
	Cause       error
}

func (e parserFailure) Unwrap() error { return e.Cause }
func (e parserFailure) Error() string {
	message := strings.TrimSuffix(strings.SplitN(e.Cause.Error(), "\n", 2)[0], "\r")
	// tsengine errors describe parser/runtime state. Any other text, or an
	// engine error containing path-like text, is reduced to its type. Guest
	// stack traces after the first line never enter stderr.
	if !strings.HasPrefix(message, "tsengine:") || strings.ContainsAny(message, "/\\\r\n") || len(message) > 512 {
		message = fmt.Sprintf("%T (detail redacted)", e.Cause)
	}
	return fmt.Sprintf("%s phase=%s ordinal=%d peak_pages=%d cause=%s", e.Code, e.Phase, e.Ordinal, e.PeakPages, message)
}
func parserFault(r receipt, code, phase string, ordinal int, cause error) (receipt, error) {
	r.Status = "failed"
	r.FailureCode = code
	r.CountsComplete = false
	return r, parserFailure{Code: code, Phase: phase, Ordinal: ordinal, PeakPages: tsengine.PeakModulePages(), Cause: cause}
}

// jsonValue rejects duplicate object keys and excessive nesting before the
// narrowly typed decode. The byte limit applies before tokenization.
func jsonValue(d *json.Decoder, depth, tenancyCap, depthCap, tokenCap int, tokens *int) error {
	if depth > depthCap {
		return errors.New("json_depth")
	}
	*tokens++
	if *tokens > tokenCap {
		return errors.New("json_tokens")
	}
	token, err := d.Token()
	if err != nil {
		return err
	}
	switch token {
	case json.Delim('{'):
		keys := map[string]bool{}
		for d.More() {
			*tokens++
			if *tokens > tokenCap {
				return errors.New("json_tokens")
			}
			k, err := d.Token()
			if err != nil {
				return err
			}
			key, ok := k.(string)
			if !ok || keys[key] {
				return errors.New("json_duplicate_or_invalid_key")
			}
			keys[key] = true
			if depth == 0 && key == "tenancy_files" && tenancyCap >= 0 {
				if err := jsonTenancy(d, tenancyCap, tokenCap, tokens); err != nil {
					return err
				}
			} else if err := jsonValue(d, depth+1, -1, depthCap, tokenCap, tokens); err != nil {
				return err
			}
		}
		_, err = d.Token()
		return err
	case json.Delim('['):
		for d.More() {
			if err := jsonValue(d, depth+1, -1, depthCap, tokenCap, tokens); err != nil {
				return err
			}
		}
		_, err = d.Token()
		return err
	default:
		return nil
	}
}
func jsonTenancy(d *json.Decoder, cap, tokenCap int, tokens *int) error {
	start, err := d.Token()
	if err != nil || start != json.Delim('[') {
		return errors.New("tenancy_array")
	}
	n := 0
	for d.More() {
		if n >= cap {
			return errors.New("files_cap")
		}
		*tokens++
		if *tokens > tokenCap {
			return errors.New("json_tokens")
		}
		v, err := d.Token()
		if err != nil {
			return err
		}
		if _, ok := v.(string); !ok {
			return errors.New("tenancy_path_type")
		}
		n++
	}
	_, err = d.Token()
	return err
}
func boundedJSON(file string, cap int64, tenancyCap int, dst any) (string, error) {
	if cap < 0 {
		return "", errors.New("json_cap")
	}
	f, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer f.Close()
	before, err := f.Stat()
	if err != nil {
		return "", err
	}
	if !before.Mode().IsRegular() || before.Size() > cap {
		return "", errors.New("json_cap_or_type")
	}
	h := sha256.New()
	counted := &io.LimitedReader{R: f, N: cap + 1}
	d := json.NewDecoder(io.TeeReader(counted, h))
	tokens := 0
	if err := jsonValue(d, 0, tenancyCap, 64, maxJSONTokens, &tokens); err != nil {
		return "", err
	}
	if _, err := d.Token(); err != io.EOF {
		return "", errors.New("json_trailing")
	}
	if cap+1-counted.N > cap {
		return "", errors.New("json_cap")
	}
	first := hex.EncodeToString(h.Sum(nil))
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	h2 := sha256.New()
	counted = &io.LimitedReader{R: f, N: cap + 1}
	d = json.NewDecoder(io.TeeReader(counted, h2))
	if err := d.Decode(dst); err != nil {
		return "", err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return "", errors.New("json_trailing")
	}
	after, err := f.Stat()
	if err != nil || cap+1-counted.N > cap || !os.SameFile(before, after) || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) || hex.EncodeToString(h2.Sum(nil)) != first {
		return "", errors.New("json_cap")
	}
	return first, nil
}
func validPath(p string) bool {
	return p != "" && !path.IsAbs(p) && !strings.Contains(p, "\\") && path.Clean(p) == p && p != "." && !strings.HasPrefix(p, "../") && p != ".." && !strings.ContainsRune(p, '\x00')
}
func safeFile(root, p string, cap int64) ([]byte, error) {
	if !validPath(p) {
		return nil, errors.New("path")
	}
	current := root
	for _, part := range strings.Split(p, "/") {
		current = filepath.Join(current, part)
		st, err := os.Lstat(current)
		if err != nil {
			return nil, err
		}
		if st.Mode()&os.ModeSymlink != 0 {
			return nil, errors.New("symlink")
		}
	}
	before, err := os.Stat(current)
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() || before.Size() > cap {
		return nil, errors.New("source_cap_or_type")
	}
	f, err := os.Open(current)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(before, opened) {
		return nil, errors.New("source_changed")
	}
	b, err := io.ReadAll(io.LimitReader(f, cap+1))
	if err != nil {
		return nil, err
	}
	after, err := os.Stat(current)
	if err != nil || int64(len(b)) > cap || int64(len(b)) != before.Size() || !os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) || before.Size() != after.Size() {
		return nil, errors.New("source_changed")
	}
	return b, nil
}
func add(p *partition, e, m bool) {
	if e {
		p.Error++
	}
	if m {
		p.Missing++
	}
	if e && m {
		p.Both++
	} else if e {
		p.OnlyE++
	} else if m {
		p.OnlyM++
	}
	if e || m {
		p.Union++
	}
}
func has(q *tsengine.Query, t *tsengine.Tree) (bool, error) {
	matches, err := q.Run(t)
	return len(matches) > 0, err
}
func writeHash(h hash.Hash, p string, b []byte) {
	_, _ = io.WriteString(h, p)
	_, _ = h.Write([]byte{0})
	s := sha256.Sum256(b)
	_, _ = h.Write(s[:])
	_, _ = h.Write([]byte{0})
}
func git(dir string, args ...string) (string, error) {
	command := exec.Command("git", append([]string{"-C", dir}, args...)...)
	b, err := command.Output()
	return strings.TrimSpace(string(b)), err
}
func productIdentity(c config) (string, string, error) {
	if c.ProductRevision != "" {
		return c.ProductRevision, "synthetic", nil
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "", "", errors.New("build_info")
	}
	var revision, modified string
	for _, s := range info.Settings {
		if s.Key == "vcs.revision" {
			revision = s.Value
		}
		if s.Key == "vcs.modified" {
			modified = s.Value
		}
	}
	if revision == "" {
		return "", "", errors.New("build_revision")
	}
	live, err := git(".", "rev-parse", "HEAD")
	if err != nil || live != revision {
		return "", "", errors.New("stale_binary")
	}
	if modified != "false" {
		return "", "", errors.New("dirty_binary")
	}
	exe, err := os.Executable()
	if err != nil {
		return "", "", err
	}
	f, err := os.Open(exe)
	if err != nil {
		return "", "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err = io.Copy(h, f); err != nil {
		return "", "", err
	}
	return revision, hex.EncodeToString(h.Sum(nil)), nil
}
func measure(c config) (receipt, error) {
	r := receipt{Status: "failed"}
	external := c.CandidateLock != "" || c.CandidateModule != ""
	if external {
		r.Mode = "external-candidate-diagnostic"
		if !c.Candidate || c.CandidateLock == "" || c.CandidateModule == "" {
			return failure(r, "candidate_arguments")
		}
	}
	if c.Candidate {
		if !external {
			r.Mode = "candidate-diagnostic"
		}
	}
	lim := bounds(c)
	if c.Subject == "" || c.SubjectName == "" || c.Reading == "" || c.Unmatched == "" || lim.JSONBytes < 0 || lim.Files < 0 || lim.SourceBytes < 0 || lim.TotalBytes < 0 {
		return failure(r, "arguments")
	}
	var read reading
	var err error
	if r.ReadingSHA256, err = boundedJSON(c.Reading, lim.JSONBytes, -1, &read); err != nil {
		return failure(r, "reading_invalid")
	}
	var match unmatched
	if r.UnmatchedSHA256, err = boundedJSON(c.Unmatched, lim.JSONBytes, lim.Files, &match); err != nil {
		return failure(r, "unmatched_invalid")
	}
	var selected int = -1
	for i := range read.Subjects {
		if read.Subjects[i].Name == c.SubjectName {
			if selected >= 0 {
				return failure(r, "duplicate_subject")
			}
			selected = i
		}
	}
	if read.Dialect != "cpp" || selected < 0 || !fullSHA(read.FathomCommit) || read.Subjects[selected].Files <= 0 || read.Subjects[selected].Bytes <= 0 || len(match.TenancyFiles) != read.Subjects[selected].Files || len(match.TenancyFiles) > lim.Files {
		return failure(r, "population_mismatch")
	}
	if c.Candidate {
		r.BaselineRevision = read.FathomCommit
	}
	subject := read.Subjects[selected]
	st, err := os.Lstat(c.Subject)
	if err != nil || !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
		return failure(r, "subject_invalid")
	}
	if !fullSHA(subject.SHA) {
		return failure(r, "subject_revision")
	}
	if c.SubjectRevision != "" {
		if c.SubjectRevision != subject.SHA {
			return failure(r, "subject_revision")
		}
	} else {
		head, err := git(c.Subject, "rev-parse", "HEAD")
		if err != nil || head != subject.SHA {
			return failure(r, "subject_revision")
		}
		dirty, err := git(c.Subject, "status", "--porcelain", "--untracked-files=no")
		if err != nil || dirty != "" {
			return failure(r, "subject_changed")
		}
	}
	pop := sha256.New()
	for i, p := range match.TenancyFiles {
		if !validPath(p) || (i > 0 && match.TenancyFiles[i-1] >= p) {
			return failure(r, "population_paths")
		}
		_, _ = io.WriteString(pop, p+"\n")
	}
	r.PopulationSHA256 = hex.EncodeToString(pop.Sum(nil))
	r.ProductRevision, r.ProductBinarySHA256, err = productIdentity(c)
	if err != nil {
		return failure(r, "product_identity")
	}
	if !fullSHA(r.ProductRevision) {
		return failure(r, "product_revision_mismatch")
	}
	if c.Candidate {
		root := c.ProductRoot
		if root == "" {
			root = "."
		}
		if err := candidateDiffAllowedForMode(root, read.FathomCommit, r.ProductRevision, external); err != nil {
			return failure(r, "candidate_diff_refused")
		}
	} else if r.ProductRevision != read.FathomCommit {
		return failure(r, "product_revision_mismatch")
	}
	lock, err := tsengine.LoadGrammarLock()
	if err != nil {
		return failure(r, "grammar_lock")
	}
	var audit *auditCandidate
	if external {
		audit, err = loadAuditCandidate(c)
		if err != nil {
			return failure(r, "candidate_module_refused")
		}
		r.AuditGrammarVersion = audit.GrammarVersion
		r.AuditRuntimeVersion = audit.RuntimeVersion
		r.AuditGrammarArchiveSHA256 = audit.GrammarArchiveSHA256
		r.AuditRuntimeArchiveSHA256 = audit.RuntimeArchiveSHA256
	}
	var parser *tsengine.Parser
	tsengine.ResetPeakModulePages()
	var errorQ, missingQ *tsengine.Query
	defer func() {
		if missingQ != nil {
			missingQ.Close()
		}
		if errorQ != nil {
			errorQ.Close()
		}
		if parser != nil {
			parser.Close()
		}
	}()
	sourceHash := sha256.New()
	fileHashes := make([]string, 0, len(match.TenancyFiles))
	var total int64
	for index, p := range match.TenancyFiles {
		src, err := safeFile(c.Subject, p, lim.SourceBytes)
		if err != nil {
			r.Written.Faults++
			r.Selected.Faults++
			return failure(r, "source_fault")
		}
		total += int64(len(src))
		if total > lim.TotalBytes {
			return failure(r, "total_cap")
		}
		target, err := extractor.ProbeTargetForSource("cpp", p, src)
		if err != nil {
			r.Written.Faults++
			r.Selected.Faults++
			return failure(r, "target_fault")
		}
		if parser == nil {
			row, ok := lock.Row(target.Blob)
			if !ok || target.Blob != "cpp" || target.EntryPoint != "tree_sitter_cpp" || row.BlobSHA256 == "" || (!c.Candidate && ((read.Grammar.Blob != "" && read.Grammar.Blob != target.Blob) || (read.Grammar.BlobSHA256 != "" && read.Grammar.BlobSHA256 != row.BlobSHA256))) {
				return failure(r, "grammar_mismatch")
			}
			if external {
				r.GrammarBlobSHA256 = audit.ModuleSHA256
			} else {
				r.GrammarBlobSHA256 = row.BlobSHA256
				r.GrammarCommit = row.Commit
			}
			r.GrammarEntryPoint = target.EntryPoint
			r.Preprocess = target.Preprocess != nil
			if external {
				parser, err = openAuditParser(context.Background(), audit)
			} else {
				parser, err = tsengine.NewParser(context.Background(), target.Blob, target.EntryPoint)
			}
			if err != nil {
				return parserFault(r, "parser_fault", "raw", index+1, err)
			}
			errorQ, err = parser.CompileQuery("(ERROR) @error")
			if err != nil {
				return parserFault(r, "query_fault", "raw", index+1, err)
			}
			missingQ, err = parser.CompileQuery("(MISSING) @missing")
			if err != nil {
				return parserFault(r, "query_fault", "raw", index+1, err)
			}
		} else if target.Blob != "cpp" || target.EntryPoint != r.GrammarEntryPoint || (target.Preprocess != nil) != r.Preprocess {
			return failure(r, "grammar_variant")
		}
		raw, err := parser.Parse(src)
		if err != nil {
			r.Written.Faults++
			r.Selected.Faults++
			return parserFault(r, "parse_fault", "raw", index+1, err)
		}
		e1, eerr := has(errorQ, raw)
		m1, merr := has(missingQ, raw)
		raw.Close()
		if eerr != nil || merr != nil {
			r.Written.Faults++
			r.Selected.Faults++
			return parserFault(r, "query_fault", "raw", index+1, errors.Join(eerr, merr))
		}
		selected, err := extractor.ProbeParse(parser, target, src)
		if err != nil {
			r.Selected.Faults++
			return parserFault(r, "parse_fault", "selected", index+1, err)
		}
		e2, eerr := has(errorQ, selected.Tree)
		m2, merr := has(missingQ, selected.Tree)
		selected.Tree.Close()
		if eerr != nil || merr != nil {
			r.Selected.Faults++
			return parserFault(r, "query_fault", "selected", index+1, errors.Join(eerr, merr))
		}
		add(&r.Written, e1, m1)
		add(&r.Selected, e2, m2)
		if selected.Candidate > 0 {
			r.PreprocessSelected++
		}
		writeHash(sourceHash, p, src)
		fileHashes = append(fileHashes, digest(src))
		r.Files++
		r.Bytes = total
	}
	if r.Bytes != subject.Bytes {
		return failure(r, "reading_bytes")
	}
	// A second bounded pass catches source changes during the full measurement.
	for i, p := range match.TenancyFiles {
		src, err := safeFile(c.Subject, p, lim.SourceBytes)
		if err != nil || digest(src) != fileHashes[i] {
			r.Selected.Faults++
			r.Written.Faults++
			return failure(r, "source_changed")
		}
	}
	var readingAgain reading
	readingHash, err := boundedJSON(c.Reading, lim.JSONBytes, -1, &readingAgain)
	if err != nil || readingHash != r.ReadingSHA256 {
		return failure(r, "reading_changed")
	}
	var unmatchedAgain unmatched
	unmatchedHash, err := boundedJSON(c.Unmatched, lim.JSONBytes, lim.Files, &unmatchedAgain)
	if err != nil || unmatchedHash != r.UnmatchedSHA256 {
		return failure(r, "unmatched_changed")
	}
	if c.SubjectRevision == "" {
		head, err := git(c.Subject, "rev-parse", "HEAD")
		if err != nil || head != subject.SHA {
			return failure(r, "subject_changed")
		}
		dirty, err := git(c.Subject, "status", "--porcelain", "--untracked-files=no")
		if err != nil || dirty != "" {
			return failure(r, "subject_changed")
		}
	}
	r.SourceSHA256 = hex.EncodeToString(sourceHash.Sum(nil))
	r.Status = "complete"
	r.CountsComplete = true
	return r, nil
}
func readBlob(lock tsengine.GrammarLock, digest string) string {
	for _, row := range lock.Grammars {
		if row.BlobSHA256 == digest {
			return row.Blob
		}
	}
	return ""
}

func under(p, root string) bool {
	rel, err := filepath.Rel(root, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
func canonical(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	var tail []string
	cur := abs
	for {
		if _, err := os.Lstat(cur); err == nil {
			break
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			return "", errors.New("path")
		}
		tail = append(tail, filepath.Base(cur))
		cur = parent
	}
	cur, err = filepath.EvalSymlinks(cur)
	if err != nil {
		return "", err
	}
	for i := len(tail) - 1; i >= 0; i-- {
		cur = filepath.Join(cur, tail[i])
	}
	return cur, nil
}
func validateOutput(out string) error {
	if out == "" {
		return errors.New("output")
	}
	repo, err := git(".", "rev-parse", "--show-toplevel")
	if err != nil {
		return err
	}
	common, err := git(repo, "rev-parse", "--git-common-dir")
	if err != nil {
		return err
	}
	if !filepath.IsAbs(common) {
		common = filepath.Join(repo, common)
	}
	common, err = filepath.EvalSymlinks(common)
	if err != nil {
		return err
	}
	roots := []string{filepath.Dir(common), repo}
	dest, err := canonical(out)
	if err != nil {
		return err
	}
	for _, root := range roots {
		command := exec.Command("git", "-C", root, "check-ignore", "-q", "workspace/")
		if err := command.Run(); err != nil {
			return errors.New("workspace_not_ignored")
		}
		wr, err := canonical(filepath.Join(root, "workspace"))
		if err != nil {
			return err
		}
		if under(dest, wr) && dest != wr {
			return nil
		}
	}
	return errors.New("output_outside_workspace")
}
func validateOutputFor(c config) error {
	if err := validateOutput(c.Out); err != nil {
		return err
	}
	if _, err := os.Lstat(c.Out); err == nil {
		return errors.New("output_exists")
	} else if !os.IsNotExist(err) {
		return errors.New("output_unavailable")
	}
	dest, err := canonical(c.Out)
	if err != nil {
		return err
	}
	for _, input := range []string{c.Subject, c.Reading, c.Unmatched, c.CandidateLock, c.CandidateModule} {
		if input == "" {
			continue
		}
		p, err := canonical(input)
		if err != nil {
			return err
		}
		if under(dest, p) || under(p, dest) {
			return errors.New("output_overlaps_input")
		}
	}
	exe, err := os.Executable()
	if err == nil {
		p, err := canonical(exe)
		if err == nil && (under(dest, p) || under(p, dest)) {
			return errors.New("output_overlaps_tool")
		}
	}
	return nil
}
func run(c config) error {
	if err := validateOutputFor(c); err != nil {
		return errors.New("output_refused")
	}
	r, measureErr := measure(c)
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(c.Out), 0700); err != nil {
		return errors.New("output_write")
	}
	temporary, err := os.CreateTemp(filepath.Dir(c.Out), ".cpp-screen-*")
	if err != nil {
		return errors.New("output_write")
	}
	defer os.Remove(temporary.Name())
	if _, err = temporary.Write(append(b, '\n')); err != nil {
		temporary.Close()
		return errors.New("output_write")
	}
	if err = temporary.Close(); err != nil {
		return errors.New("output_write")
	}
	// Link publishes the complete temp file with O_EXCL semantics. An existing
	// receipt can never be silently replaced, even if it appears mid-run.
	if err = os.Link(temporary.Name(), c.Out); err != nil {
		return errors.New("output_write")
	}
	return measureErr
}
func main() {
	c := config{}
	flag.StringVar(&c.Subject, "subject", "", "subject checkout")
	flag.StringVar(&c.SubjectName, "subject-name", "", "fixed public reading subject name")
	flag.StringVar(&c.Reading, "reading", "", "private JSON reading")
	flag.StringVar(&c.Unmatched, "unmatched", "", "private JSON unmatched receipt")
	flag.StringVar(&c.Out, "out", "", "ignored workspace receipt path")
	flag.BoolVar(&c.Candidate, "candidate", false, "audit-only candidate parser diagnostic against a baseline reading")
	flag.StringVar(&c.CandidateLock, "candidate-lock", "", "private external C++ audit lock under ignored workspace")
	flag.StringVar(&c.CandidateModule, "candidate-module", "", "private external C++ raw WASM under ignored workspace")
	flag.Parse()
	if err := run(c); err != nil {
		fmt.Fprintln(os.Stderr, "cpp-screen:", err)
		os.Exit(1)
	}
}

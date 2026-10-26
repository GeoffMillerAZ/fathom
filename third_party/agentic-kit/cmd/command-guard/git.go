package main

import (
	"bytes"
	"context"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	dlp "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/dlp-evaluator"
)

// Git inspection owns one invocation's process state. It has no mutable package seams.
type gitPolicy struct {
	Profile         string        `json:"profile"`
	Executable      string        `json:"executable"`
	SHA256          string        `json:"sha256"`
	ExpectedVersion string        `json:"expectedVersion"`
	TimeoutMillis   int           `json:"timeoutMillis"`
	Repository      gitRepository `json:"repository"`
}
type gitRepository struct {
	Kind                 string `json:"kind"`
	Root                 string `json:"root"`
	GitDir               string `json:"gitDir"`
	CommonDir            string `json:"commonDir"`
	ConfigSHA256         string `json:"configSHA256"`
	WorktreeConfigSHA256 string `json:"worktreeConfigSHA256"`
	CommonDirFileSHA256  string `json:"commonDirFileSHA256"`
}
type gitRequest struct {
	Version    string   `json:"version"`
	Kind       string   `json:"kind"`
	Paths      []string `json:"paths,omitempty"`
	Base       string   `json:"base,omitempty"`
	Target     string   `json:"target,omitempty"`
	MaxCommits int      `json:"maxCommits,omitempty"`
}
type gitRow struct {
	Kind                string   `json:"kind"`
	Path                string   `json:"path,omitempty"`
	XY                  string   `json:"xy,omitempty"`
	OID                 string   `json:"oid,omitempty"`
	ParentIDs           []string `json:"parentIds,omitempty"`
	Author              string   `json:"author,omitempty"`
	Committer           string   `json:"committer,omitempty"`
	Message             string   `json:"message,omitempty"`
	BeforeOID           string   `json:"beforeOid,omitempty"`
	AfterOID            string   `json:"afterOid,omitempty"`
	Before              []string `json:"before,omitempty"`
	After               []string `json:"after,omitempty"`
	BeforeOmittedLines  int      `json:"beforeOmittedLines,omitempty"`
	AfterOmittedLines   int      `json:"afterOmittedLines,omitempty"`
	MessageOmittedBytes int      `json:"messageOmittedBytes,omitempty"`
}
type gitReport struct {
	Version             string          `json:"version"`
	Status              string          `json:"status"`
	Code                string          `json:"code"`
	Operation           string          `json:"operation,omitempty"`
	NoActionAuthority   bool            `json:"noActionAuthority"`
	Scope               []string        `json:"scope,omitempty"`
	BaseCommit          string          `json:"baseCommit,omitempty"`
	TargetCommit        string          `json:"targetCommit,omitempty"`
	Scanner             string          `json:"scanner"`
	ExternalScanner     *scannerReceipt `json:"externalScanner,omitempty"`
	AttemptedCalls      int             `json:"attemptedCalls"`
	StartedCalls        int             `json:"startedCalls"`
	CompletedCalls      int             `json:"completedCalls"`
	BytesScanned        int             `json:"bytesScanned"`
	Returned            int             `json:"returned"`
	Omitted             int             `json:"omitted"`
	OmittedLines        int             `json:"omittedLines"`
	OmittedBytes        int             `json:"omittedBytes"`
	ProjectionTruncated bool            `json:"projectionTruncated"`
	HasMore             bool            `json:"hasMore"`
	NonAtomic           bool            `json:"nonAtomic"`
	Limitation          string          `json:"limitation"`
	Rows                []gitRow        `json:"rows,omitempty"`
	outputCap           int
}

func gitBase() gitReport {
	return gitReport{Version: "command-guard.git-report/1", Status: "incomplete", Code: "invalid", NoActionAuthority: true, Scanner: "configured-regex-only", Limitation: "selected-source-only; no-action-authority", outputCap: maxOutput}
}

var oidRE = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)

func hash64(s string) bool {
	return len(s) == 64 && strings.IndexFunc(s, func(r rune) bool { return r < '0' || r > '9' && r < 'a' || r > 'f' }) < 0
}
func cleanAbsolute(s string) bool {
	return filepath.IsAbs(s) && filepath.Clean(s) == s && len(s) <= 4096 && validText(s) && !strings.ContainsAny(s, "\r\n\t")
}
func validGitPolicy(root string, s scope, p policy) bool {
	if !gitControlsMatch(s, p) || p.Git == nil {
		return false
	}
	g := p.Git
	r := g.Repository
	if g.Profile != "git-read/1" || !cleanAbsolute(g.Executable) || !hash64(g.SHA256) || g.TimeoutMillis < 1 || g.TimeoutMillis > 30000 || len(g.ExpectedVersion) < 12 || len(g.ExpectedVersion) > 128 || !strings.HasPrefix(g.ExpectedVersion, "git version ") || !validText(g.ExpectedVersion) || strings.ContainsAny(g.ExpectedVersion, "\r\n\t") {
		return false
	}
	if !cleanAbsolute(root) || !cleanAbsolute(r.Root) || !cleanAbsolute(r.GitDir) || !cleanAbsolute(r.CommonDir) || root != r.Root || !hash64(r.ConfigSHA256) {
		return false
	}
	if r.Kind != "worktree" && r.Kind != "bare" || r.Kind == "bare" && (r.Root != r.GitDir || r.Root != r.CommonDir) {
		return false
	}
	for _, v := range []string{r.WorktreeConfigSHA256, r.CommonDirFileSHA256} {
		if v != "absent" && !hash64(v) {
			return false
		}
	}
	return true
}
func gitControlsMatch(s scope, p policy) bool {
	if len(s.RequiredGitProfiles) == 0 {
		return p.Git == nil
	}
	return len(s.RequiredGitProfiles) == 1 && s.RequiredGitProfiles[0] == "git-read/1" && p.Git != nil
}
func validateGitRequest(q gitRequest, raw []byte) bool {
	if q.Version != "command-guard.git-request/1" {
		return false
	}
	var m map[string]json.RawMessage
	if json.Unmarshal(raw, &m) != nil {
		return false
	}
	has := func(k string) bool { _, ok := m[k]; return ok }
	switch q.Kind {
	case "git-status":
		if !has("paths") || has("base") || has("target") || has("maxCommits") {
			return false
		}
	case "git-object-diff":
		if !has("paths") || !has("base") || !has("target") || has("maxCommits") || !gitRef(q.Base) || !gitRef(q.Target) {
			return false
		}
	case "git-history":
		if has("paths") || has("base") || !has("target") || !has("maxCommits") || !gitRef(q.Target) || q.MaxCommits < 1 || q.MaxCommits > 32 {
			return false
		}
	default:
		return false
	}
	if q.Kind != "git-history" {
		if len(q.Paths) < 1 || len(q.Paths) > 32 {
			return false
		}
		seen := map[string]bool{}
		for _, s := range q.Paths {
			if s != "." && !validPath(s) || seen[strings.ToLower(s)] {
				return false
			}
			seen[strings.ToLower(s)] = true
		}
	}
	return true
}
func gitRef(s string) bool { return s == "HEAD" || oidRE.MatchString(s) }
func selected(path string, selectors []string) bool {
	for _, s := range selectors {
		if s == "." || s == path || strings.HasPrefix(path, s+"/") {
			return true
		}
	}
	return false
}
func deniedGit(path string, denied []string) bool {
	f := strings.ToLower(path)
	for _, d := range denied {
		d = strings.ToLower(d)
		if f == d || strings.HasPrefix(f, d+"/") {
			return true
		}
	}
	return false
}
func selectorOverlapsDenied(selector string, denied []string) bool {
	for _, d := range denied {
		if selector == "." || deniedGit(selector, []string{d}) || strings.HasPrefix(strings.ToLower(d), strings.ToLower(selector)+"/") {
			return true
		}
	}
	return false
}

type gitExec func(context.Context, string, string, []string, []string, int) scannerRunResult

func gitCommand(ctx context.Context, exe, dir string, env, args []string, cap int) scannerRunResult {
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Dir = dir
	cmd.Env = env
	cmd.WaitDelay = 100 * time.Millisecond
	out := &cappedWriter{cap: cap}
	errout := &cappedWriter{cap: 64 << 10}
	cmd.Stdout = out
	cmd.Stderr = errout
	if e := cmd.Start(); e != nil {
		return scannerRunResult{code: "execution_incomplete"}
	}
	e := cmd.Wait()
	if out.exceeded || errout.exceeded {
		return scannerRunResult{started: true, code: "output_limit"}
	}
	if ctx.Err() != nil {
		return scannerRunResult{started: true, code: "deadline"}
	}
	if e == nil {
		return scannerRunResult{out: out.b.Bytes(), errout: errout.b.Bytes(), started: true}
	}
	if x, ok := e.(*exec.ExitError); ok {
		return scannerRunResult{out: out.b.Bytes(), errout: errout.b.Bytes(), exit: x.ExitCode(), started: true}
	}
	return scannerRunResult{started: true, code: "execution_incomplete"}
}
func protectedPath(path string) bool {
	if !cleanAbsolute(path) {
		return false
	}
	p := path
	for {
		st, e := os.Lstat(p)
		if e != nil || st.Mode()&os.ModeSymlink != 0 {
			return false
		}
		if p == filepath.Dir(p) {
			break
		}
		p = filepath.Dir(p)
	}
	return true
}
func protectedFile(path string, limit int, want string, optional bool) ([]byte, string, bool) {
	if want == "absent" && optional {
		_, e := os.Lstat(path)
		return nil, "absent", errors.Is(e, os.ErrNotExist)
	}
	if !protectedPath(path) {
		return nil, "", false
	}
	st, e := os.Lstat(path)
	if e != nil || !st.Mode().IsRegular() || st.Size() > int64(limit) {
		return nil, "", false
	}
	f, e := os.OpenFile(path, os.O_RDONLY|nonblock, 0)
	if e != nil {
		return nil, "", false
	}
	defer f.Close()
	actual, e := f.Stat()
	if e != nil || !os.SameFile(st, actual) {
		return nil, "", false
	}
	b, e := io.ReadAll(io.LimitReader(f, int64(limit+1)))
	if e != nil || len(b) > limit {
		return nil, "", false
	}
	h := sha256.Sum256(b)
	sum := hex.EncodeToString(h[:])
	return b, sum, sum == want
}
func probeAbsent(path string) bool { _, e := os.Lstat(path); return errors.Is(e, os.ErrNotExist) }
func protectedInventory(ctx context.Context, root string, count *int) bool {
	if probeAbsent(root) {
		return true
	}
	if !protectedPath(root) {
		return false
	}
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		*count += 1
		if *count > 65536 || ctx.Err() != nil {
			return os.ErrInvalid
		}
		if d.Type()&os.ModeSymlink != 0 || strings.HasSuffix(d.Name(), ".promisor") {
			return os.ErrInvalid
		}
		if !d.IsDir() {
			info, e := d.Info()
			if e != nil || !info.Mode().IsRegular() {
				return os.ErrInvalid
			}
		}
		return nil
	})
	return err == nil
}
func gitEnv(home string, r gitRepository) []string {
	e := gitBaseEnv(home)
	e = append(e, "GIT_DIR="+r.GitDir, "GIT_COMMON_DIR="+r.CommonDir, "GIT_OBJECT_DIRECTORY="+filepath.Join(r.CommonDir, "objects"), "GIT_INDEX_FILE="+filepath.Join(r.GitDir, "index"), "GIT_ASKPASS="+filepath.Join(home, "absent"), "GIT_SSH_COMMAND=false", "GIT_PROTOCOL_FROM_USER=0", "GIT_TRACE=0", "GIT_TRACE_SETUP=0", "GIT_TRACE_PERFORMANCE=0")
	if r.Kind == "worktree" {
		e = append(e, "GIT_WORK_TREE="+r.Root)
	}
	return e
}
func gitPrivateEnv(home string) []string {
	e := gitBaseEnv(home)
	return append(e, "GIT_DIR="+filepath.Join(home, "empty-git"), "GIT_COMMON_DIR="+filepath.Join(home, "empty-git"))
}
func gitBaseEnv(home string) []string {
	e := scannerEnv(home)
	return append(e, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+filepath.Join(home, "empty-config"), "GIT_ATTR_NOSYSTEM=1", "GIT_LITERAL_PATHSPECS=1", "GIT_NO_REPLACE_OBJECTS=1", "GIT_NO_LAZY_FETCH=1", "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0", "GIT_ALLOW_PROTOCOL=", "LC_ALL=C")
}

func gitPrefix(home string) []string {
	return []string{"--no-pager", "--no-optional-locks", "--literal-pathspecs", "--no-lazy-fetch", "--no-replace-objects", "-c", "core.hooksPath=" + filepath.Join(home, "hooks"), "-c", "core.fsmonitor=false", "-c", "core.sparseCheckout=false", "-c", "core.attributesFile=" + filepath.Join(home, "attributes"), "-c", "diff.external=", "-c", "diff.trustExitCode=false", "-c", "gc.auto=0", "-c", "maintenance.auto=false", "-c", "protocol.allow=never", "-c", "fetch.fsckObjects=true", "-c", "log.showSignature=false", "-c", "commit.gpgSign=false", "-c", "tag.gpgSign=false", "-c", "core.pager=cat"}
}

type gitSession struct {
	ctx            context.Context
	exe, home      string
	env            []string
	run            gitExec
	report         *gitReport
	fileCap        int
	totalCap       int
	payload        int
	resolvedHead   string
	selectedBase   string
	selectedTarget string
}

func (g *gitSession) call(cap int, args ...string) ([]byte, bool) {
	if g.ctx.Err() != nil {
		g.report.Code = "deadline"
		return nil, false
	}
	g.report.AttemptedCalls++
	all := append(gitPrefix(g.home), args...)
	res := g.run(g.ctx, g.exe, g.home, g.env, all, cap)
	if res.started {
		g.report.StartedCalls++
	}
	if res.code != "" || res.exit != 0 || len(res.errout) != 0 {
		g.report.Code = "git_incomplete"
		if res.code != "" {
			g.report.Code = res.code
		}
		return nil, false
	}
	g.report.CompletedCalls++
	return res.out, true
}
func (g *gitSession) optionalRef(ref, format string) (string, bool) {
	if g.ctx.Err() != nil {
		g.report.Code = "deadline"
		return "", false
	}
	g.report.AttemptedCalls++
	args := append(gitPrefix(g.home), "rev-parse", "--verify", "--quiet", "--end-of-options", ref)
	res := g.run(g.ctx, g.exe, g.home, g.env, args, 128)
	if res.started {
		g.report.StartedCalls++
	}
	if res.code != "" || len(res.errout) != 0 {
		g.report.Code = "git_incomplete"
		return "", false
	}
	if res.exit == 1 && len(res.out) == 0 {
		g.report.CompletedCalls++
		return "unborn", true
	}
	if res.exit != 0 {
		g.report.Code = "git_incomplete"
		return "", false
	}
	id := strings.TrimSuffix(string(res.out), "\n")
	if !fullOID(id, format) || string(res.out) != id+"\n" {
		g.report.Code = "source_invalid"
		return "", false
	}
	g.report.CompletedCalls++
	return id, true
}
func (g *gitSession) headState(r gitRepository, format string) (string, bool) {
	p := filepath.Join(r.GitDir, "HEAD")
	if !protectedPath(p) {
		g.report.Code = "metadata_invalid"
		return "", false
	}
	f, e := os.OpenFile(p, os.O_RDONLY|nonblock, 0)
	if e != nil {
		g.report.Code = "metadata_invalid"
		return "", false
	}
	b, e := io.ReadAll(io.LimitReader(f, 4097))
	f.Close()
	if e != nil || len(b) > 4096 || len(b) == 0 {
		g.report.Code = "metadata_invalid"
		return "", false
	}
	s := string(b)
	if !validSource(s) || !strings.HasSuffix(s, "\n") {
		g.report.Code = "metadata_invalid"
		return "", false
	}
	if strings.HasPrefix(s, "ref: ") {
		ref := strings.TrimSuffix(strings.TrimPrefix(s, "ref: "), "\n")
		if !strings.HasPrefix(ref, "refs/heads/") || !validPath(ref) {
			g.report.Code = "metadata_invalid"
			return "", false
		}
		id, ok := g.optionalRef(ref, format)
		return ref + ":" + id, ok
	}
	id := strings.TrimSuffix(s, "\n")
	if !fullOID(id, format) {
		g.report.Code = "metadata_invalid"
		return "", false
	}
	return "detached:" + id, true
}
func (g *gitSession) version(expected string) bool {
	b, ok := g.call(128, "version")
	return ok && string(b) == expected+"\n"
}
func (g *gitSession) config(blob []byte, kind string, primary bool) bool {
	f := filepath.Join(g.home, "config-copy")
	if e := os.WriteFile(f, blob, 0600); e != nil {
		return false
	}
	defer os.Remove(f)
	b, ok := g.call(2<<20, "config", "--file", f, "--no-includes", "-z", "--list")
	if !ok {
		return false
	}
	seen := map[string]bool{}
	for _, rec := range bytes.Split(bytes.TrimSuffix(b, []byte{0}), []byte{0}) {
		if len(rec) == 0 {
			return false
		}
		kv := bytes.SplitN(rec, []byte{'\n'}, 2)
		if len(kv) != 2 {
			return false
		}
		k, v := string(kv[0]), string(kv[1])
		if !validText(k) || !validText(v) || seen[k] {
			return false
		}
		seen[k] = true
		switch k {
		case "core.repositoryformatversion":
			if v != "0" && v != "1" {
				return false
			}
		case "core.bare", "core.filemode", "core.logallrefupdates", "core.ignorecase", "core.precomposeunicode", "core.symlinks", "core.autocrlf":
			if v != "true" && v != "false" && v != "input" {
				return false
			}
			if k == "core.bare" && ((kind == "bare") != (v == "true")) {
				return false
			}
		case "extensions.objectformat":
			if v != "sha1" && v != "sha256" {
				return false
			}
		case "extensions.worktreeconfig":
			if v != "true" {
				return false
			}
		case "user.name", "user.email", "init.defaultbranch":
			if v == "" || len(v) > 1024 {
				return false
			}
		default:
			if !safeUnusedConfig(k, v) {
				return false
			}
		}
	}
	return len(b) > 0 && b[len(b)-1] == 0 && (!primary || seen["core.repositoryformatversion"] && seen["core.bare"])
}
func safeUnusedConfig(k, v string) bool {
	if len(v) > 4096 {
		return false
	}
	parts := strings.Split(k, ".")
	if len(parts) != 3 || !validID(parts[1]) {
		return false
	}
	if parts[0] == "remote" && (parts[2] == "url" || parts[2] == "fetch") {
		return true
	}
	if parts[0] == "branch" && (parts[2] == "remote" || parts[2] == "merge") {
		return true
	}
	return false
}
func metadata(ctx context.Context, r gitRepository) ([]byte, string, bool) {
	for _, p := range []string{r.Root, r.GitDir, r.CommonDir, filepath.Join(r.CommonDir, "objects")} {
		if !protectedPath(p) {
			return nil, "", false
		}
		st, e := os.Stat(p)
		if e != nil || !st.IsDir() {
			return nil, "", false
		}
	}
	for _, p := range []string{filepath.Join(r.CommonDir, "objects", "info", "alternates"), filepath.Join(r.CommonDir, "info", "grafts"), filepath.Join(r.CommonDir, "shallow"), filepath.Join(r.GitDir, "shallow"), filepath.Join(r.CommonDir, "objects", "info", "http-alternates")} {
		if !probeAbsent(p) {
			return nil, "", false
		}
	}
	if !probeAbsent(filepath.Join(r.CommonDir, "refs", "replace")) {
		return nil, "", false
	}
	count := 0
	for _, p := range []string{filepath.Join(r.CommonDir, "refs"), filepath.Join(r.GitDir, "refs"), filepath.Join(r.CommonDir, "objects"), filepath.Join(r.CommonDir, "info")} {
		if !protectedInventory(ctx, p, &count) {
			return nil, "", false
		}
	}
	for _, p := range []string{filepath.Join(r.CommonDir, "packed-refs"), filepath.Join(r.GitDir, "packed-refs")} {
		if !probeAbsent(p) {
			if !protectedPath(p) {
				return nil, "", false
			}
			st, e := os.Lstat(p)
			if e != nil || !st.Mode().IsRegular() || st.Size() > 16<<20 {
				return nil, "", false
			}
		}
	}
	commonFile := filepath.Join(r.GitDir, "commondir")
	_, _, ok := protectedFile(commonFile, 4096, r.CommonDirFileSHA256, true)
	if !ok {
		return nil, "", false
	}
	if r.CommonDirFileSHA256 == "absent" && r.GitDir != r.CommonDir {
		return nil, "", false
	}
	if r.CommonDirFileSHA256 != "absent" {
		b, _, _ := protectedFile(commonFile, 4096, r.CommonDirFileSHA256, false)
		v := strings.TrimSuffix(string(b), "\n")
		resolved := v
		if !filepath.IsAbs(v) {
			resolved = filepath.Join(r.GitDir, v)
		}
		if filepath.Clean(resolved) != r.CommonDir {
			return nil, "", false
		}
	}
	config, _, ok := protectedFile(filepath.Join(r.CommonDir, "config"), 1<<20, r.ConfigSHA256, false)
	if !ok {
		return nil, "", false
	}
	_, _, ok = protectedFile(filepath.Join(r.GitDir, "config.worktree"), 1<<20, r.WorktreeConfigSHA256, true)
	if !ok {
		return nil, "", false
	}
	if r.Kind == "worktree" {
		p := filepath.Join(r.GitDir, "index")
		if !probeAbsent(p) {
			if !protectedPath(p) {
				return nil, "", false
			}
			st, e := os.Stat(p)
			if e != nil || !st.Mode().IsRegular() || st.Size() > 16<<20 {
				return nil, "", false
			}
		}
	}
	fp, ok := gitFingerprint(r)
	if !ok {
		return nil, "", false
	}
	return config, fp, true
}
func gitFingerprint(r gitRepository) (string, bool) {
	h := sha256.New()
	for _, item := range []struct {
		p        string
		cap      int
		required bool
	}{{filepath.Join(r.CommonDir, "config"), 1 << 20, true}, {filepath.Join(r.GitDir, "config.worktree"), 1 << 20, false}, {filepath.Join(r.GitDir, "commondir"), 4096, false}, {filepath.Join(r.GitDir, "index"), 16 << 20, false}, {filepath.Join(r.GitDir, "HEAD"), 4096, true}} {
		if probeAbsent(item.p) {
			if item.required {
				return "", false
			}
			h.Write([]byte("absent:" + item.p))
			continue
		}
		if !protectedPath(item.p) {
			return "", false
		}
		st, e := os.Lstat(item.p)
		if e != nil || !st.Mode().IsRegular() || st.Size() > int64(item.cap) {
			return "", false
		}
		f, e := os.OpenFile(item.p, os.O_RDONLY|nonblock, 0)
		if e != nil {
			return "", false
		}
		b, e := io.ReadAll(io.LimitReader(f, int64(item.cap+1)))
		f.Close()
		if e != nil || len(b) > item.cap {
			return "", false
		}
		h.Write([]byte(item.p))
		h.Write(b)
	}
	return hex.EncodeToString(h.Sum(nil)), true
}
func oidHash(kind string, b []byte, format string) string {
	header := []byte(fmt.Sprintf("%s %d\x00", kind, len(b)))
	if format == "sha256" {
		h := sha256.New()
		h.Write(header)
		h.Write(b)
		return hex.EncodeToString(h.Sum(nil))
	}
	h := sha1.New()
	h.Write(header)
	h.Write(b)
	return hex.EncodeToString(h.Sum(nil))
}
func fullOID(s, format string) bool {
	return oidRE.MatchString(s) && len(s) == map[string]int{"sha1": 40, "sha256": 64}[format]
}
func validSource(s string) bool { return validText(s) && !strings.ContainsRune(s, '\x00') }
func scanGit(data [][]byte, repr []byte, ev *dlp.Evaluator, p policy, g *gitSession, scanner scannerRunner) bool {
	data = append(data, repr)
	found := false
	for _, b := range data {
		if g.ctx.Err() != nil {
			g.report.Code = "deadline"
			return false
		}
		if !validSource(string(b)) {
			g.report.Code = "source_invalid"
			return false
		}
		g.report.BytesScanned += len(b)
		if ev.Evaluate(string(b)).MaxRisk == dlp.RiskHigh {
			found = true
		}
	}
	if found {
		g.report.Status = "blocked"
		g.report.Code = "rule"
		return false
	}
	if p.Scanner != nil {
		g.report.Scanner = "configured-regex-and-gitleaks"
		g.report.ExternalScanner = &scannerReceipt{ID: "gitleaks", Profile: "gitleaks-stdin-8.30.1", Status: "not-invoked", Code: "not_invoked", ExpectedFiles: len(data)}
		if !scanSelectedWithContext(g.ctx, *p.Scanner, data, g.report.ExternalScanner, scanner) {
			g.report.Code = "scanner_incomplete"
			return false
		}
		if g.report.ExternalScanner.Findings > 0 {
			g.report.Status = "blocked"
			g.report.Code = "scanner_finding"
			return false
		}
	}
	return true
}
func executeGit(args []string, w io.Writer) int {
	r := gitBase()
	fs := flag.NewFlagSet("git-inspect", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var root, sp, pp, qp, format string
	fs.StringVar(&root, "root", "", "")
	fs.StringVar(&sp, "scope", "", "")
	fs.StringVar(&pp, "policy", "", "")
	fs.StringVar(&qp, "request", "", "")
	fs.StringVar(&format, "format", "json", "")
	if fs.Parse(args) != nil || len(fs.Args()) != 0 || root == "" || sp == "" || pp == "" || qp == "" || (format != "json" && format != "text" && format != "compact") {
		return emitGit(w, r, "json", 2)
	}
	var s scope
	var p policy
	var q gitRequest
	if strictFile(sp, &s) != nil || strictFile(pp, &p) != nil || strictFile(qp, &q) != nil {
		return emitGit(w, r, format, 2)
	}
	if len(s.RequiredClassifierIDs) > 0 || len(s.PermittedClassifierSHA256) > 0 || s.ClassifierPolicySHA256 != "" || p.Classifier != nil {
		r.Code = "unsupported"
		return emitGit(w, r, format, 2)
	}
	r = runGit(root, s, p, q, format, gitCommand)
	code := 2
	if r.Status == "no-findings" {
		code = 0
	} else if r.Status == "blocked" {
		code = 1
	}
	return emitGit(w, r, format, code)
}
func emitGit(w io.Writer, r gitReport, format string, code int) int {
	b := encodeGit(r, format)
	b = append(b, '\n')
	if len(b) > r.outputCap {
		return 2
	}
	n, e := w.Write(b)
	if e != nil || n != len(b) {
		return 2
	}
	return code
}
func encodeGit(r gitReport, format string) []byte {
	if format == "text" {
		b, _ := json.MarshalIndent(r, "", "  ")
		return b
	}
	b, _ := json.Marshal(r)
	if format == "compact" {
		return compactGit(r, b)
	}
	return b
}
func runGit(root string, s scope, p policy, q gitRequest, format string, runner gitExec) (r gitReport) {
	return runGitWithScanner(root, s, p, q, format, runner, scannerCommand)
}
func runGitWithScanner(root string, s scope, p policy, q gitRequest, format string, runner gitExec, scanner scannerRunner) (r gitReport) {
	r = gitBase()
	if len(s.RequiredClassifierIDs) > 0 || len(s.PermittedClassifierSHA256) > 0 || s.ClassifierPolicySHA256 != "" || p.Classifier != nil {
		r.Code = "unsupported"
		return
	}
	if !validGitPolicy(root, s, p) || !validScannerControls(s, p) {
		return
	}
	r.Operation = q.Kind
	cap, denied, ev, code := validateControls(s, p, q.Kind)
	if cap.MaxOutputBytes > 0 {
		r.outputCap = cap.MaxOutputBytes
	}
	if code != "" {
		r.Code = code
		if code == "denied" {
			r.Status = "blocked"
		}
		return
	}
	for _, path := range q.Paths {
		if selectorOverlapsDenied(path, denied) {
			r.Status = "blocked"
			r.Code = "denied"
			return
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(p.Git.TimeoutMillis)*time.Millisecond)
	defer cancel()
	g := &gitSession{ctx: ctx, run: runner, report: &r, fileCap: min(cap.MaxFileBytes, maxFile), totalCap: min(cap.MaxTotalBytes, maxTotal)}
	g.ExeInit(p.Git)
	if g.exe == "" {
		return
	}
	defer func() {
		if e := os.RemoveAll(filepath.Dir(g.exe)); e != nil {
			r.Status = "incomplete"
			r.Code = "cleanup_failed"
			r.Scope = nil
			r.BaseCommit = ""
			r.TargetCommit = ""
			r.Rows = nil
			r.Returned = 0
			r.Omitted = 0
		}
	}()
	if !g.version(p.Git.ExpectedVersion) {
		if r.Code == "invalid" {
			r.Code = "version_mismatch"
		}
		return
	}
	cfg, before, ok := metadata(ctx, p.Git.Repository)
	if !ok {
		r.Code = "metadata_invalid"
		if ctx.Err() != nil {
			r.Code = "deadline"
		}
		return
	}
	if !g.config(cfg, p.Git.Repository.Kind, true) {
		if r.Code == "invalid" {
			r.Code = "config_invalid"
		}
		return
	}
	if p.Git.Repository.WorktreeConfigSHA256 != "absent" {
		wcfg, _, valid := protectedFile(filepath.Join(p.Git.Repository.GitDir, "config.worktree"), 1<<20, p.Git.Repository.WorktreeConfigSHA256, false)
		if !valid || !g.config(wcfg, p.Git.Repository.Kind, false) {
			if r.Code == "invalid" {
				r.Code = "config_invalid"
			}
			return
		}
	}
	g.env = gitEnv(g.home, p.Git.Repository)
	b, ok := g.call(64, "rev-parse", "--show-object-format=storage")
	if !ok {
		return
	}
	formatID := strings.TrimSuffix(string(b), "\n")
	if formatID != "sha1" && formatID != "sha256" {
		r.Code = "format_unsupported"
		return
	}
	var rows []gitRow
	var source [][]byte
	var more bool
	var headBefore string
	if q.Kind == "git-status" {
		headBefore, ok = g.headState(p.Git.Repository, formatID)
		if !ok {
			return
		}
	}
	switch q.Kind {
	case "git-status":
		if p.Git.Repository.Kind == "bare" {
			r.Code = "unsupported"
			return
		}
		rows, source, ok = g.status(q, formatID, denied)
		r.NonAtomic = true
		r.Limitation = "live-worktree-non-atomic; ignored-and-submodules-excluded; no-clean-or-absence-inference"
	case "git-object-diff":
		rows, source, ok = g.diff(q, formatID, denied)
		r.Limitation = "selected-immutable-objects-only"
	case "git-history":
		rows, source, more, ok = g.history(q, formatID)
		r.HasMore = more
		r.Limitation = "bounded-selected-history; older-unexamined"
	default:
		return
	}
	if !ok {
		return
	}
	if q.Kind == "git-status" {
		headAfter, valid := g.headState(p.Git.Repository, formatID)
		if !valid || headBefore != headAfter {
			r.Code = "state_drift"
			return
		}
	}
	cfg2, after, ok := metadata(ctx, p.Git.Repository)
	if !ok || !bytes.Equal(cfg, cfg2) || before != after {
		r.Code = "state_drift"
		if ctx.Err() != nil {
			r.Code = "deadline"
		}
		return
	}
	// The complete selected payload is scanned before any view is emitted.
	total := 0
	for _, b := range source {
		total += len(b)
		if len(b) > min(cap.MaxFileBytes, maxFile) || total > cap.MaxTotalBytes || total > maxTotal {
			r.Code = "payload_limit"
			return
		}
	}
	// Bound view and scan its exact normalized source-data representation.
	for _, row := range rows {
		r.OmittedLines += row.BeforeOmittedLines + row.AfterOmittedLines
		r.OmittedBytes += row.MessageOmittedBytes
	}
	r.ProjectionTruncated = r.OmittedLines > 0 || r.OmittedBytes > 0
	projected := rows
	reserve := 0
	if p.Scanner != nil {
		reserve = 1024
	}
	// Compact may fit more rows in the final wire encoding. Its candidate
	// prefix must also fit the existing normalized scan representation before
	// that representation is scanned; otherwise a shorter prefix may be valid.
	projectionRepr := func(prefix []gitRow) []byte {
		b, _ := json.Marshal(struct {
			Scope        []string `json:"scope"`
			Rows         []gitRow `json:"rows"`
			BaseCommit   string   `json:"baseCommit,omitempty"`
			TargetCommit string   `json:"targetCommit,omitempty"`
		}{q.Paths, prefix, g.selectedBase, g.selectedTarget})
		return b
	}
	for {
		probe := r
		probe.Scope = q.Paths
		probe.BaseCommit = g.selectedBase
		probe.TargetCommit = g.selectedTarget
		probe.Status = "no-findings"
		probe.Code = "ok"
		probe.Rows = projected
		probe.Returned = len(projected)
		probe.Omitted = len(rows) - len(projected)
		probe.ProjectionTruncated = r.ProjectionTruncated || probe.Omitted > 0
		fitsRepr := true
		if format == "compact" {
			reprLen := len(projectionRepr(projected))
			fitsRepr = reprLen <= maxTotal && reprLen+total <= cap.MaxTotalBytes
		}
		if len(projected) <= cap.MaxResults && len(encodeGit(probe, format))+1+reserve <= cap.MaxOutputBytes && fitsRepr {
			break
		}
		if len(projected) == 0 {
			r.Code = "output_limit"
			if !fitsRepr {
				r.Code = "payload_limit"
			}
			return
		}
		projected = projected[:len(projected)-1]
	}
	repr := projectionRepr(projected)
	if len(repr) > maxTotal || len(repr)+total > cap.MaxTotalBytes {
		r.Code = "payload_limit"
		return
	}
	if !scanGit(source, repr, ev, p, g, scanner) {
		return
	}
	if ctx.Err() != nil {
		r.Code = "deadline"
		return
	}
	r.Status = "no-findings"
	r.Code = "ok"
	r.Scope = append([]string{}, q.Paths...)
	r.BaseCommit = g.selectedBase
	r.TargetCommit = g.selectedTarget
	r.Returned = len(projected)
	r.Omitted = len(rows) - len(projected)
	r.ProjectionTruncated = r.ProjectionTruncated || r.Omitted > 0
	r.Rows = projected
	if len(encodeGit(r, format))+1 > cap.MaxOutputBytes {
		r.Status = "incomplete"
		r.Code = "output_limit"
		r.Scope = nil
		r.BaseCommit = ""
		r.TargetCommit = ""
		r.Rows = nil
		r.Returned = 0
		r.Omitted = 0
	}
	return
}
func (g *gitSession) ExeInit(p *gitPolicy) {
	exe, home, cleanup, code := executableSnapshot(g.ctx, p.Executable, p.SHA256)
	if code != "" {
		g.report.Code = code
		return
	}
	_ = cleanup
	g.exe = exe
	g.home = home
	g.env = gitPrivateEnv(home)
	if runtime.GOOS == "windows" {
		g.report.Code = "unsupported"
		g.exe = ""
		cleanup()
		return
	}
	if e := os.WriteFile(filepath.Join(home, "empty-config"), nil, 0600); e != nil {
		g.report.Code = "preparation_failed"
		g.exe = ""
		cleanup()
		return
	}
	if e := os.WriteFile(filepath.Join(home, "attributes"), nil, 0600); e != nil {
		g.report.Code = "preparation_failed"
		g.exe = ""
		cleanup()
		return
	}
	if e := os.Mkdir(filepath.Join(home, "hooks"), 0700); e != nil {
		g.report.Code = "preparation_failed"
		g.exe = ""
		cleanup()
		return
	}
	if e := os.Mkdir(filepath.Join(home, "empty-git"), 0700); e != nil {
		g.report.Code = "preparation_failed"
		g.exe = ""
		cleanup()
		return
	}
}
func (g *gitSession) resolve(ref, format string) (string, bool) {
	if ref != "HEAD" {
		if !fullOID(ref, format) {
			g.report.Code = "invalid"
			return "", false
		}
		return ref, true
	}
	if g.resolvedHead != "" {
		return g.resolvedHead, true
	}
	b, ok := g.call(128, "rev-parse", "--verify", "HEAD^{commit}")
	if !ok {
		return "", false
	}
	oid := strings.TrimSuffix(string(b), "\n")
	if !fullOID(oid, format) {
		g.report.Code = "source_invalid"
		return "", false
	}
	g.resolvedHead = oid
	return oid, true
}
func (g *gitSession) object(kind, oid, format string, limit int) ([]byte, bool) {
	if !fullOID(oid, format) {
		g.report.Code = "source_invalid"
		return nil, false
	}
	t, ok := g.call(32, "cat-file", "-t", oid)
	if !ok {
		return nil, false
	}
	if string(t) != kind+"\n" {
		g.report.Code = "source_invalid"
		return nil, false
	}
	n, ok := g.call(32, "cat-file", "-s", oid)
	if !ok {
		return nil, false
	}
	size, e := strconv.Atoi(strings.TrimSuffix(string(n), "\n"))
	if e != nil || size < 0 || size > limit || size > g.fileCap || size > g.totalCap-g.payload {
		g.report.Code = "payload_limit"
		return nil, false
	}
	b, ok := g.call(size+1, "cat-file", kind, oid)
	if !ok {
		return nil, false
	}
	if len(b) != size || oidHash(kind, b, format) != oid {
		g.report.Code = "source_invalid"
		return nil, false
	}
	g.payload += size
	return b, true
}
func (g *gitSession) status(q gitRequest, format string, denied []string) ([]gitRow, [][]byte, bool) {
	args := []string{"-c", "status.renames=false", "status", "--porcelain=v2", "-z", "--untracked-files=all", "--ignore-submodules=all", "--no-renames", "--"}
	args = append(args, q.Paths...)
	b, ok := g.call(16<<20, args...)
	if !ok {
		return nil, nil, false
	}
	if len(b) > 0 && b[len(b)-1] != 0 {
		g.report.Code = "source_invalid"
		return nil, nil, false
	}
	rows := []gitRow{}
	source := [][]byte{}
	var records [][]byte
	if len(b) > 0 {
		records = bytes.Split(b[:len(b)-1], []byte{0})
	}
	for _, item := range records {
		rec := string(item)
		var path, xy, kind string
		switch {
		case strings.HasPrefix(rec, "1 "):
			parts := strings.SplitN(rec, " ", 9)
			if len(parts) != 9 || len(parts[1]) != 2 || parts[2] != "N..." {
				g.report.Code = "unsupported"
				return nil, nil, false
			}
			xy = parts[1]
			path = parts[8]
			kind = "tracked"
			for _, m := range parts[3:6] {
				if m != "100644" && m != "100755" && m != "000000" {
					g.report.Code = "unsupported"
					return nil, nil, false
				}
			}
			for _, id := range parts[6:8] {
				if !fullOID(id, format) {
					g.report.Code = "source_invalid"
					return nil, nil, false
				}
			}
		case strings.HasPrefix(rec, "? "):
			path = rec[2:]
			xy = "??"
			kind = "untracked"
		case strings.HasPrefix(rec, "! "):
			continue
		case strings.HasPrefix(rec, "2 "), strings.HasPrefix(rec, "u "):
			g.report.Code = "unsupported"
			return nil, nil, false
		default:
			g.report.Code = "source_invalid"
			return nil, nil, false
		}
		if !validPath(path) || !validSource(path) {
			g.report.Code = "source_invalid"
			return nil, nil, false
		}
		if !selected(path, q.Paths) {
			g.report.Code = "source_invalid"
			return nil, nil, false
		}
		if deniedGit(path, denied) {
			g.report.Status = "blocked"
			g.report.Code = "denied"
			return nil, nil, false
		}
		if len(rows) >= 32 {
			g.report.Code = "result_limit"
			return nil, nil, false
		}
		if kind == "tracked" && !validXY(xy) {
			g.report.Code = "unsupported"
			return nil, nil, false
		}
		rows = append(rows, gitRow{Kind: kind, Path: path, XY: xy})
		source = append(source, []byte(rec))
	}
	return rows, source, true
}
func validXY(x string) bool {
	if len(x) != 2 {
		return false
	}
	for _, c := range x {
		if !strings.ContainsRune(" .MADRCU?", c) {
			return false
		}
	}
	return true
}
func regularMode(s string) bool { return s == "100644" || s == "100755" }
func zeroOID(s string) bool     { return (len(s) == 40 || len(s) == 64) && strings.Trim(s, "0") == "" }
func (g *gitSession) diff(q gitRequest, format string, denied []string) ([]gitRow, [][]byte, bool) {
	base, ok := g.resolve(q.Base, format)
	if !ok {
		return nil, nil, false
	}
	target, ok := g.resolve(q.Target, format)
	if !ok {
		return nil, nil, false
	}
	g.selectedBase = base
	g.selectedTarget = target
	for _, oid := range []string{base, target} {
		if _, ok = g.object("commit", oid, format, maxFile); !ok {
			return nil, nil, false
		}
	}
	args := []string{"diff-tree", "--raw", "-r", "-z", "--no-renames", "--no-ext-diff", "--no-textconv", "--no-abbrev", "--no-commit-id", base, target, "--"}
	args = append(args, q.Paths...)
	b, ok := g.call(16<<20, args...)
	if !ok {
		return nil, nil, false
	}
	if len(b) > 0 && b[len(b)-1] != 0 {
		g.report.Code = "source_invalid"
		return nil, nil, false
	}
	var items [][]byte
	if len(b) > 0 {
		items = bytes.Split(b[:len(b)-1], []byte{0})
	}
	rows := []gitRow{}
	source := [][]byte{}
	for i := 0; i < len(items); i += 2 {
		if i+1 >= len(items) {
			g.report.Code = "source_invalid"
			return nil, nil, false
		}
		hdr := string(items[i])
		path := string(items[i+1])
		f := strings.Fields(hdr)
		if len(f) != 5 || strings.Join(f, " ") != hdr || !strings.HasPrefix(f[0], ":") || len(f[4]) != 1 || !strings.Contains("AMD", f[4]) || !validPath(path) {
			g.report.Code = "source_invalid"
			return nil, nil, false
		}
		if !selected(path, q.Paths) {
			g.report.Code = "source_invalid"
			return nil, nil, false
		}
		if deniedGit(path, denied) {
			g.report.Status = "blocked"
			g.report.Code = "denied"
			return nil, nil, false
		}
		source = append(source, []byte(hdr), []byte(path))
		if len(rows) >= 32 {
			g.report.Code = "result_limit"
			return nil, nil, false
		}
		oldMode, newMode := strings.TrimPrefix(f[0], ":"), f[1]
		oldID, newID := f[2], f[3]
		if oldMode != "000000" && !regularMode(oldMode) || newMode != "000000" && !regularMode(newMode) || oldMode == "000000" && !zeroOID(oldID) || newMode == "000000" && !zeroOID(newID) {
			g.report.Code = "unsupported"
			return nil, nil, false
		}
		if oldMode != "000000" && !fullOID(oldID, format) || newMode != "000000" && !fullOID(newID, format) {
			g.report.Code = "source_invalid"
			return nil, nil, false
		}
		row := gitRow{Kind: "change", Path: path}
		if oldMode != "000000" {
			blob, ok := g.object("blob", oldID, format, maxFile)
			if !ok {
				return nil, nil, false
			}
			if !validSource(string(blob)) {
				g.report.Code = "unsupported"
				return nil, nil, false
			}
			source = append(source, blob)
			row.BeforeOID = oldID
			row.Before, row.BeforeOmittedLines = lineView(blob)
		}
		if newMode != "000000" {
			blob, ok := g.object("blob", newID, format, maxFile)
			if !ok {
				return nil, nil, false
			}
			if !validSource(string(blob)) {
				g.report.Code = "unsupported"
				return nil, nil, false
			}
			source = append(source, blob)
			row.AfterOID = newID
			row.After, row.AfterOmittedLines = lineView(blob)
		}
		rows = append(rows, row)
	}
	return rows, source, true
}
func lineView(b []byte) ([]string, int) {
	view := make([]string, 0, 8)
	omitted := 0
	stopped := false
	for len(b) > 0 {
		n := bytes.IndexByte(b, '\n')
		line := b
		if n >= 0 {
			line = b[:n]
			b = b[n+1:]
		} else {
			b = nil
		}
		if len(line) > 0 && line[len(line)-1] == '\r' {
			line = line[:len(line)-1]
		}
		if !stopped && len(view) < 8 && len(line) <= 512 {
			view = append(view, string(line))
		} else {
			stopped = true
			omitted++
		}
	}
	return view, omitted
}
func (g *gitSession) history(q gitRequest, format string) ([]gitRow, [][]byte, bool, bool) {
	target, ok := g.resolve(q.Target, format)
	if !ok {
		return nil, nil, false, false
	}
	g.selectedTarget = target
	if _, ok = g.object("commit", target, format, maxFile); !ok {
		return nil, nil, false, false
	}
	b, ok := g.call((q.MaxCommits+1)*70, "rev-list", "--topo-order", "--max-count="+strconv.Itoa(q.MaxCommits+1), target)
	if !ok {
		return nil, nil, false, false
	}
	if len(b) == 0 || b[len(b)-1] != '\n' {
		g.report.Code = "source_invalid"
		return nil, nil, false, false
	}
	ids := strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
	if len(ids) < 1 || len(ids) > q.MaxCommits+1 || ids[0] != target {
		g.report.Code = "source_invalid"
		return nil, nil, false, false
	}
	seen := map[string]bool{}
	for _, id := range ids {
		if !fullOID(id, format) || seen[id] {
			g.report.Code = "source_invalid"
			return nil, nil, false, false
		}
		seen[id] = true
	}
	more := len(ids) > q.MaxCommits
	if more {
		ids = ids[:q.MaxCommits]
	}
	rows := []gitRow{}
	source := [][]byte{}
	for _, id := range ids {
		raw, ok := g.object("commit", id, format, maxFile)
		if !ok {
			return nil, nil, false, false
		}
		row, ok := parseCommit(id, raw, format)
		if !ok {
			g.report.Code = "source_invalid"
			return nil, nil, false, false
		}
		rows = append(rows, row)
		source = append(source, raw)
	}
	return rows, source, more, true
}
func parseCommit(id string, b []byte, format string) (gitRow, bool) {
	row := gitRow{Kind: "commit", OID: id}
	if !validSource(string(b)) {
		return row, false
	}
	parts := bytes.SplitN(b, []byte("\n\n"), 2)
	if len(parts) != 2 || len(parts[0]) > 8192 {
		return row, false
	}
	seen := map[string]bool{}
	signature := false
	for _, line := range strings.Split(string(parts[0]), "\n") {
		if strings.HasPrefix(line, " ") {
			if !signature {
				return row, false
			}
			continue
		}
		signature = false
		kv := strings.SplitN(line, " ", 2)
		if len(kv) != 2 {
			return row, false
		}
		key, val := kv[0], kv[1]
		switch key {
		case "tree":
			if seen[key] || !fullOID(val, format) {
				return row, false
			}
			seen[key] = true
		case "parent":
			if !fullOID(val, format) || len(row.ParentIDs) >= 32 {
				return row, false
			}
			row.ParentIDs = append(row.ParentIDs, val)
		case "author":
			if seen[key] || !validSource(val) {
				return row, false
			}
			seen[key] = true
			row.Author = val
		case "committer":
			if seen[key] || !validSource(val) {
				return row, false
			}
			seen[key] = true
			row.Committer = val
		case "encoding":
			if seen[key] || val != "UTF-8" && val != "utf-8" {
				return row, false
			}
			seen[key] = true
		case "gpgsig", "gpgsig-sha256", "mergetag":
			if (key != "mergetag" && seen[key]) || val == "" {
				return row, false
			}
			seen[key] = true
			signature = true
		default:
			return row, false
		}
	}
	if !seen["tree"] || !seen["author"] || !seen["committer"] {
		return row, false
	}
	if len(parts[1]) <= 4096 {
		row.Message = string(parts[1])
	} else {
		row.MessageOmittedBytes = len(parts[1])
	}
	return row, true
}

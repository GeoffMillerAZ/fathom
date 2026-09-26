package main

// console_since.go — the readers behind "since you last looked" (OC-18,
// console.SinceLastLook): the console's page-view log for the last look,
// each project's remote-tracking reflog for pushes, the call log's binary
// stamps for deploys, the daemon's record for its start, and the commits
// that added bench readings for measurements. Every reader is a file read
// or one bounded `git` call per repository; nothing is written.

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/geoffmilleraz/fathom/internal/console"
)

// sinceSources names one gather's inputs.
type sinceSources struct {
	baseURL  string // the daemon whose record names its start
	callLog  string // the resolved call log (deploys)
	viewsLog string // the console's page-view log (the last look)
	model    observatoryModel
	now      time.Time
}

// gatherSince reads every ledger and projects the list.
func gatherSince(ctx context.Context, src sinceSources) console.Since {
	in := console.SinceInputs{Now: src.now, Limit: 50}

	// The last look: the previous view of Now.
	views := console.SourceRead{Name: "page views", Path: src.viewsLog}
	switch data, err := os.ReadFile(src.viewsLog); {
	case src.viewsLog == "":
		views.State, views.Reason = console.StateAbsent, "no page-view log is configured"
		in.LastLookNote = "no page-view log is configured, so no last look is known — everything below is listed as new"
	case os.IsNotExist(err):
		views.State, views.Reason = console.StateAbsent, "no page-view log yet"
		in.LastLookNote = "no view of Now is recorded yet — everything below is listed as new"
	case err != nil:
		views.State, views.Reason = console.StateError, err.Error()
		in.LastLookNote = "the page-view log could not be read — everything below is listed as new"
	default:
		views.State = console.StatePresent
		if t, ok := console.LastLook(data, "home", src.now); ok {
			in.LastLook = t
			in.LastLookNote = "the previous view of Now in " + src.viewsLog
		} else {
			in.LastLookNote = "no earlier view of Now is recorded — everything below is listed as new"
		}
	}
	in.Sources = append(in.Sources, views)

	// Deploys, and the daemon's own start.
	deploys := console.SourceRead{Name: "call log", Path: src.callLog}
	var deployChanges []console.Change
	if src.callLog == "" {
		deploys.State, deploys.Reason = console.StateAbsent, "no call log was resolved"
	} else if scan, err := scanDeploys(src.callLog); err != nil {
		if os.IsNotExist(err) {
			deploys.State, deploys.Reason = console.StateAbsent, "no call log yet"
		} else {
			deploys.State, deploys.Reason = console.StateError, err.Error()+tccWedgeHint(src.callLog)
		}
	} else {
		deploys.State = console.StatePresent
		deployChanges = console.DeployChanges(scan, "the call log's fathom_version stamps")
	}
	in.Sources = append(in.Sources, deploys)
	record := console.SourceRead{Name: "daemon record"}
	if rec, ok := readDaemonRecordFor(src.baseURL); ok {
		record.State = console.StatePresent
		restart, updated := console.DaemonStart(rec, deployChanges, "the daemon's record ("+rec.Addr+")")
		deployChanges = updated
		if restart != nil {
			in.Changes = append(in.Changes, *restart)
		}
	} else {
		record.State, record.Reason = console.StateAbsent, "no daemon record for "+src.baseURL
	}
	in.Sources = append(in.Sources, record)
	in.Changes = append(in.Changes, deployChanges...)

	// Pushes and measurements, per project root.
	pushes := console.SourceRead{Name: "git reflogs", State: console.StateAbsent, Reason: "no project has a remote-tracking reflog"}
	readings := console.SourceRead{Name: "bench readings", State: console.StateAbsent, Reason: "no project records bench readings"}
	var pushErrs, readErrs []string
	since := src.now.Add(-console.SinceMaxWindow - 24*time.Hour)
	for _, s := range src.model.Services {
		_, commonDir, ok := gitDirs(s.Path)
		if !ok {
			continue
		}
		ref, data, err := remoteTrackingReflog(commonDir)
		switch {
		case err != nil && !os.IsNotExist(err):
			pushErrs = append(pushErrs, s.Service+": "+err.Error())
		case err == nil:
			pushes.State, pushes.Reason = console.StatePresent, ""
			entries := recentReflog(console.ParseReflog(data), since)
			subjects := commitSubjects(ctx, s.Path, entries)
			in.Changes = append(in.Changes, console.PushChanges(s.Service, ref, entries, func(c string) string { return subjects[c] })...)
		}
		if changes, found, err := readingChanges(ctx, s.Service, s.Path, s.RepoHead, since); err != nil {
			readErrs = append(readErrs, s.Service+": "+err.Error())
		} else if found {
			readings.State, readings.Reason = console.StatePresent, ""
			in.Changes = append(in.Changes, changes...)
		}
	}
	if len(pushErrs) > 0 {
		pushes.State, pushes.Reason = console.StateError, strings.Join(pushErrs, "; ")
	}
	if len(readErrs) > 0 {
		readings.State, readings.Reason = console.StateError, strings.Join(readErrs, "; ")
	}
	in.Sources = append(in.Sources, pushes, readings)
	return console.SinceLastLook(in)
}

// ─── deploys: an incremental scan of the call log ──────────────────────

// deployScans remembers each call log's scan and how far it has read, so a
// growing log is read once and then only by its new rows.
var deployScans struct {
	sync.Mutex
	m map[string]*deployScanState
}

type deployScanState struct {
	offset int64
	scan   console.DeployScan
}

// scanDeploys returns the deploy scan of the log at p, reading only what
// was appended since the last call (a log that shrank is rescanned).
func scanDeploys(p string) (console.DeployScan, error) {
	deployScans.Lock()
	defer deployScans.Unlock()
	if deployScans.m == nil {
		deployScans.m = map[string]*deployScanState{}
	}
	fi, err := os.Stat(p)
	if err != nil {
		return console.DeployScan{}, err
	}
	st := deployScans.m[p]
	if st == nil || fi.Size() < st.offset {
		st = &deployScanState{}
		deployScans.m[p] = st
	}
	if fi.Size() > st.offset {
		f, err := os.Open(p)
		if err != nil {
			return console.DeployScan{}, err
		}
		defer f.Close()
		if _, err := f.Seek(st.offset, io.SeekStart); err != nil {
			return console.DeployScan{}, err
		}
		data, err := io.ReadAll(io.LimitReader(f, fi.Size()-st.offset))
		if err != nil {
			return console.DeployScan{}, err
		}
		st.scan.Feed(data)
		st.offset += int64(len(data))
	}
	out := st.scan
	out.Order = append([]console.VersionSeen{}, st.scan.Order...)
	return out, nil
}

// ─── pushes: the remote-tracking reflog ────────────────────────────────

// gitDirs resolves a project root's git directory and its common directory
// (a worktree keeps its refs' logs in the common one). ok is false when the
// root is not itself a git work tree's top — a directory inside another
// repository is not that repository.
func gitDirs(root string) (gitDir, commonDir string, ok bool) {
	if root == "" {
		return "", "", false
	}
	dotGit := filepath.Join(root, ".git")
	fi, err := os.Stat(dotGit)
	if err != nil {
		return "", "", false
	}
	gitDir = dotGit
	if !fi.IsDir() {
		raw, err := os.ReadFile(dotGit)
		if err != nil {
			return "", "", false
		}
		line := strings.TrimSpace(string(raw))
		if !strings.HasPrefix(line, "gitdir:") {
			return "", "", false
		}
		gitDir = strings.TrimSpace(strings.TrimPrefix(line, "gitdir:"))
		if !filepath.IsAbs(gitDir) {
			gitDir = filepath.Join(root, gitDir)
		}
	}
	commonDir = gitDir
	if raw, err := os.ReadFile(filepath.Join(gitDir, "commondir")); err == nil {
		c := strings.TrimSpace(string(raw))
		if !filepath.IsAbs(c) {
			c = filepath.Join(gitDir, c)
		}
		commonDir = filepath.Clean(c)
	}
	return gitDir, commonDir, true
}

// remoteTrackingReflog reads origin's default branch's reflog: the branch
// origin/HEAD names, else main, else master.
func remoteTrackingReflog(commonDir string) (ref string, data []byte, err error) {
	branches := []string{"main", "master"}
	if raw, rerr := os.ReadFile(filepath.Join(commonDir, "refs", "remotes", "origin", "HEAD")); rerr == nil {
		if b := strings.TrimPrefix(strings.TrimSpace(string(raw)), "ref: refs/remotes/origin/"); b != "" && !strings.ContainsAny(b, " \n") {
			branches = append([]string{b}, branches...)
		}
	}
	err = os.ErrNotExist
	for _, b := range branches {
		data, err = os.ReadFile(filepath.Join(commonDir, "logs", "refs", "remotes", "origin", filepath.FromSlash(b)))
		if err == nil {
			return "origin/" + b, data, nil
		}
		if !os.IsNotExist(err) {
			return "origin/" + b, nil, err
		}
	}
	return "", nil, err
}

// recentReflog keeps the entries at or after since.
func recentReflog(entries []console.ReflogEntry, since time.Time) []console.ReflogEntry {
	var out []console.ReflogEntry
	for _, e := range entries {
		if !e.At.Before(since) {
			out = append(out, e)
		}
	}
	return out
}

// commitSubjectCache maps a commit to its subject (commits are immutable).
var commitSubjectCache struct {
	sync.Mutex
	m map[string]string
}

// commitSubjects names each entry's new commit by its subject: one bounded
// `git log --no-walk` for the commits not yet named.
func commitSubjects(ctx context.Context, root string, entries []console.ReflogEntry) map[string]string {
	commitSubjectCache.Lock()
	if commitSubjectCache.m == nil {
		commitSubjectCache.m = map[string]string{}
	}
	out := map[string]string{}
	var missing []string
	for _, e := range entries {
		if s, ok := commitSubjectCache.m[e.New]; ok {
			out[e.New] = s
		} else if isHexCommit(e.New) {
			missing = append(missing, e.New)
		}
	}
	commitSubjectCache.Unlock()
	if len(missing) == 0 {
		return out
	}
	sort.Strings(missing)
	if len(missing) > 64 {
		missing = missing[len(missing)-64:]
	}
	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	// --ignore-missing: a commit the reflog names and the repository no
	// longer has (a force-push, a pruned fetch) leaves the others named.
	args := append([]string{"-C", root, "log", "--no-walk", "--ignore-missing", "--format=%H%x09%s"}, missing...)
	raw, err := exec.CommandContext(cctx, "git", args...).Output()
	if err != nil {
		return out
	}
	commitSubjectCache.Lock()
	defer commitSubjectCache.Unlock()
	for _, line := range strings.Split(string(raw), "\n") {
		h, subj, ok := strings.Cut(line, "\t")
		if ok {
			commitSubjectCache.m[h] = subj
			out[h] = subj
		}
	}
	return out
}

func isHexCommit(s string) bool {
	if len(s) < 7 || len(s) > 64 {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}

// ─── measurements: commits that added bench readings ───────────────────

// readingsCache keeps each root's reading changes per HEAD.
var readingsCache struct {
	sync.Mutex
	m map[string]readingsEntry
}

type readingsEntry struct {
	key     string
	changes []console.Change
}

// readingChanges lists the bench readings a project's commits added since
// the given time; found is false when the project records none (no
// readings directory).
func readingChanges(ctx context.Context, service, root, head string, since time.Time) ([]console.Change, bool, error) {
	if _, _, ok := gitDirs(root); !ok {
		return nil, false, nil
	}
	var dirs []string
	for _, d := range []string{"docs/bench/readings", "docs/measurements"} {
		if fi, err := os.Stat(filepath.Join(root, filepath.FromSlash(d))); err == nil && fi.IsDir() {
			dirs = append(dirs, d)
		}
	}
	if len(dirs) == 0 {
		return nil, false, nil
	}
	key := head + "|" + since.UTC().Format("2006-01-02")
	readingsCache.Lock()
	if e, ok := readingsCache.m[root]; ok && head != "" && e.key == key {
		readingsCache.Unlock()
		return e.changes, true, nil
	}
	readingsCache.Unlock()
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	args := append([]string{"-C", root, "log", "--diff-filter=A", "--name-only", "--format=" + console.ReadingsLogFormat, "--since=" + since.UTC().Format(time.RFC3339), "--"}, dirs...)
	raw, err := exec.CommandContext(cctx, "git", args...).Output()
	if err != nil {
		return nil, true, err
	}
	changes := console.ParseReadings(service, raw, func(p string) string { return readingTitle(filepath.Join(root, filepath.FromSlash(p))) })
	readingsCache.Lock()
	if readingsCache.m == nil {
		readingsCache.m = map[string]readingsEntry{}
	}
	readingsCache.m[root] = readingsEntry{key: key, changes: changes}
	readingsCache.Unlock()
	return changes, true, nil
}

// readingTitle is a reading's first "# " heading, "" when it has none.
func readingTitle(p string) string {
	f, err := os.Open(p)
	if err != nil {
		return ""
	}
	defer f.Close()
	head := make([]byte, 4096)
	n, _ := io.ReadFull(f, head)
	for _, line := range bytes.Split(head[:n], []byte{'\n'}) {
		if t := strings.TrimSpace(string(line)); strings.HasPrefix(t, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(t, "# "))
		}
	}
	return ""
}

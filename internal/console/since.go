package console

// since.go — OC-18 (vision 35 §3 Now): "since you last looked". Pushes,
// deploys, measurements and restarts, newest first, each read from a ledger
// that recorded it — never inferred:
//
//   - the last look is the newest view of the Now page in the console's own
//     page-view log (OC-15), before this one;
//   - pushes are the project's remote-tracking reflog: "update by push" is
//     a push from this clone, a fetch that moved the ref is a push landed
//     from elsewhere (observed when this clone fetched it);
//   - deploys are the first call a daemon binary answered, from the call
//     log's fathom_version stamp (the daemon writes every row itself);
//   - a restart is the serving daemon's own record (started_at, pid) when
//     it is not the deploy's start;
//   - measurements are bench readings added to the repo (docs/bench/readings,
//     docs/measurements), dated by the commit that added them.
//
// A ledger that could not be read makes the list a named lower bound.

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ChangeKind names what happened.
type ChangeKind string

const (
	ChangePush        ChangeKind = "push"
	ChangeDeploy      ChangeKind = "deploy"
	ChangeRestart     ChangeKind = "restart"
	ChangeMeasurement ChangeKind = "measurement"
)

// Change is one event since the last look.
type Change struct {
	Kind    ChangeKind `json:"kind"`
	At      string     `json:"at"`
	Service string     `json:"service,omitempty"`
	// Title is the verb a person reads first ("Pushed", "Deployed").
	Title  string `json:"title"`
	Detail string `json:"detail"`
	Ref    string `json:"ref,omitempty"`
	// New is true when the change is newer than the last look.
	New    bool   `json:"new"`
	Source string `json:"source"`
	at     time.Time
}

// SinceInputs are the changes the ledgers recorded and how each was read.
type SinceInputs struct {
	Now time.Time
	// LastLook is the previous view of the Now page; zero when none was
	// recorded (LastLookNote says why).
	LastLook     time.Time
	LastLookNote string
	Changes      []Change
	Sources      []SourceRead
	// Limit is one page's length (default 50) and Page the 1-based page
	// to return: the list is paged, never cut — every change in the window
	// is on some page (design §2: pagination over 50 rows).
	Limit int
	Page  int
}

// Since is the "since you last looked" projection.
type Since struct {
	LastLook     string `json:"last_look,omitempty"`
	LastLookNote string `json:"last_look_note"`
	From         string `json:"from"`
	// Items is this page of the changes, newest first; Total and New
	// count every change in the window, on every page.
	Items   []Change     `json:"items"`
	New     int          `json:"new"`
	Total   int          `json:"total"`
	Page    int          `json:"page"`
	Pages   int          `json:"pages"`
	PerPage int          `json:"per_page"`
	State   State        `json:"state"`
	Reason  string       `json:"reason,omitempty"`
	Sources []SourceRead `json:"sources"`
}

// SinceWindow is how far back the list reaches when the last look is
// recent; SinceMaxWindow bounds it when the last look is old.
const (
	SinceWindow    = 7 * 24 * time.Hour
	SinceMaxWindow = 30 * 24 * time.Hour
)

// SinceLastLook lists the recorded changes newest first: everything within
// the last week — or back to the last look, when that is older (at most 30
// days) — each marked new when it is newer than the last look.
func SinceLastLook(in SinceInputs) Since {
	now := in.Now.UTC()
	from := now.Add(-SinceWindow)
	if !in.LastLook.IsZero() && in.LastLook.Before(from) {
		from = in.LastLook
		if min := now.Add(-SinceMaxWindow); from.Before(min) {
			from = min
		}
	}
	s := Since{From: from.Format(time.RFC3339), LastLookNote: in.LastLookNote, Sources: in.Sources}
	if s.Sources == nil {
		s.Sources = []SourceRead{}
	}
	if !in.LastLook.IsZero() {
		s.LastLook = in.LastLook.UTC().Format(time.RFC3339)
	}
	var items []Change
	for _, c := range in.Changes {
		t, err := time.Parse(time.RFC3339Nano, c.At)
		if err != nil || t.Before(from) || t.After(now) {
			continue
		}
		c.at = t.UTC()
		c.At = c.at.Format(time.RFC3339)
		c.New = in.LastLook.IsZero() || c.at.After(in.LastLook)
		items = append(items, c)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if !items[i].at.Equal(items[j].at) {
			return items[i].at.After(items[j].at)
		}
		if items[i].Kind != items[j].Kind {
			return items[i].Kind < items[j].Kind
		}
		return items[i].Service+items[i].Ref < items[j].Service+items[j].Ref
	})
	s.Total = len(items)
	for _, c := range items {
		if c.New {
			s.New++
		}
	}
	s.PerPage = in.Limit
	if s.PerPage <= 0 {
		s.PerPage = 50
	}
	s.Pages = (len(items) + s.PerPage - 1) / s.PerPage
	if s.Pages == 0 {
		s.Pages = 1
	}
	s.Page = in.Page
	if s.Page < 1 {
		s.Page = 1
	}
	if s.Page > s.Pages {
		s.Page = s.Pages
	}
	lo := (s.Page - 1) * s.PerPage
	hi := lo + s.PerPage
	if hi > len(items) {
		hi = len(items)
	}
	if lo < hi {
		items = items[lo:hi]
	} else {
		items = nil
	}
	s.Items = items
	if s.Items == nil {
		s.Items = []Change{}
	}
	var unread []string
	read := 0
	for _, src := range in.Sources {
		switch src.State {
		case StateError, StatePartial:
			unread = append(unread, src.Name+" ("+src.Reason+")")
		case StatePresent, StateZero:
			read++
		}
	}
	switch {
	case len(unread) > 0:
		s.State, s.Reason = StatePartial, "not read: "+strings.Join(unread, "; ")+" — the list is a lower bound"
	case read == 0:
		s.State, s.Reason = StateAbsent, "no ledger that records pushes, deploys, measurements or restarts could be read"
	case len(s.Items) == 0:
		s.State = StateZero
	default:
		s.State = StatePresent
	}
	return s
}

// LastLook is the newest view of the page with the given id in a console
// page-view log that is older than before (the view being rendered now is
// not yet logged, and a clock skew must not make it its own last look).
func LastLook(viewsLog []byte, page string, before time.Time) (time.Time, bool) {
	var last time.Time
	for _, line := range bytes.Split(viewsLog, []byte{'\n'}) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var row struct {
			At   string `json:"at"`
			Page string `json:"page"`
		}
		if json.Unmarshal(line, &row) != nil || row.Page != page {
			continue
		}
		t, err := time.Parse(time.RFC3339, row.At)
		if err != nil || !t.Before(before) {
			continue
		}
		if t.After(last) {
			last = t
		}
	}
	return last, !last.IsZero()
}

// ReflogEntry is one line of a git ref's reflog.
type ReflogEntry struct {
	Old, New string
	At       time.Time
	Message  string
}

// ParseReflog reads a git reflog file (.git/logs/refs/…): "<old> <new>
// <name> <email> <unix-seconds> <tz>\t<message>" per line.
func ParseReflog(data []byte) []ReflogEntry {
	var out []ReflogEntry
	for _, line := range strings.Split(string(data), "\n") {
		head, msg, ok := strings.Cut(line, "\t")
		if !ok {
			continue
		}
		f := strings.Fields(head)
		if len(f) < 4 {
			continue
		}
		secs, err := strconv.ParseInt(f[len(f)-2], 10, 64)
		if err != nil {
			continue
		}
		out = append(out, ReflogEntry{Old: f[0], New: f[1], At: time.Unix(secs, 0).UTC(), Message: msg})
	}
	return out
}

// PushChanges turns a remote-tracking ref's reflog into changes: a push
// from this clone, or a push landed from elsewhere and fetched here. A
// fetch that did not move the ref, and the ref's creation, are neither.
// subject names a commit ("" when unknown).
func PushChanges(service, ref string, entries []ReflogEntry, subject func(commit string) string) []Change {
	var out []Change
	const zero = "0000000000000000000000000000000000000000"
	for _, e := range entries {
		if e.Old == e.New || e.Old == zero || e.New == zero {
			continue
		}
		title, how := "", ""
		switch {
		case strings.Contains(e.Message, "update by push"):
			title, how = "Pushed", "pushed from this clone"
		case strings.HasPrefix(e.Message, "fetch") || strings.HasPrefix(e.Message, "pull"):
			title, how = "Landed", "pushed elsewhere; fetched here"
		default:
			continue
		}
		if strings.Contains(e.Message, "forced-update") {
			how += ", forced"
		}
		detail := service + " " + ref + " → " + shortCommit(e.New)
		if subject != nil {
			if s := subject(e.New); s != "" {
				detail += " — " + s
			}
		}
		out = append(out, Change{Kind: ChangePush, At: e.At.Format(time.RFC3339), Service: service, Title: title, Detail: detail + " (" + how + ")", Ref: e.New, Source: "git reflog of " + ref})
	}
	return out
}

// VersionSeen is the first call a daemon binary answered.
type VersionSeen struct {
	Version string
	First   string // RFC3339 timestamp of the first row it wrote
}

// DeployScan follows a call log's fathom_version stamps: the first row each
// binary wrote. It can be fed a log in pieces (a growing file read from its
// last offset), so it scans each byte once.
type DeployScan struct {
	seen    map[string]bool
	Order   []VersionSeen
	pending []byte
}

// Feed scans more of the log. Rows a CLI process wrote itself (surface
// "cli") carry no daemon binary and are skipped.
func (d *DeployScan) Feed(data []byte) {
	if d.seen == nil {
		d.seen = map[string]bool{}
	}
	buf := append(d.pending, data...)
	last := bytes.LastIndexByte(buf, '\n')
	if last < 0 {
		d.pending = buf
		return
	}
	d.pending = append([]byte(nil), buf[last+1:]...)
	sc := bufio.NewScanner(bytes.NewReader(buf[:last]))
	sc.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		v := jsonStringField(line, "fathom_version")
		if v == "" || d.seen[v] || jsonStringField(line, "surface") == "cli" {
			continue
		}
		ts := jsonStringField(line, "ts")
		if ts == "" {
			continue
		}
		d.seen[v] = true
		d.Order = append(d.Order, VersionSeen{Version: v, First: ts})
	}
}

// jsonStringField extracts a top-level string field from one JSON line
// without decoding the whole row (a call log is tens of megabytes). It
// accepts only the field's exact key followed by a plain string.
func jsonStringField(line []byte, key string) string {
	needle := []byte(`"` + key + `":"`)
	i := bytes.Index(line, needle)
	if i < 0 {
		return ""
	}
	rest := line[i+len(needle):]
	j := bytes.IndexByte(rest, '"')
	if j < 0 || bytes.IndexByte(rest[:j], '\\') >= 0 {
		return ""
	}
	return string(rest[:j])
}

// DeployChanges are the binaries a daemon began serving: every version
// after the first the log ever recorded (the log's first binary was
// already running when it started, which is not a deploy).
func DeployChanges(scan DeployScan, source string) []Change {
	var out []Change
	for i, v := range scan.Order {
		if i == 0 {
			continue
		}
		prev := scan.Order[i-1].Version
		out = append(out, Change{Kind: ChangeDeploy, At: v.First, Title: "Deployed", Ref: v.Version,
			Detail: "the daemon began answering with binary " + shortCommit(v.Version) + " (was " + shortCommit(prev) + ")", Source: source})
	}
	return out
}

// DaemonStart is the serving daemon's start as a change: a restart, unless
// its binary's deploy began within ten minutes of it (then it is that
// deploy's start, and the deploy is dated by it instead).
func DaemonStart(rec DaemonRecord, deploys []Change, source string) (restart *Change, updated []Change) {
	started, err := time.Parse(time.RFC3339, rec.StartedAt)
	if err != nil {
		return nil, deploys
	}
	updated = append([]Change{}, deploys...)
	commit := rec.BinaryCommit
	for i, d := range updated {
		if commit == "" || !(strings.HasPrefix(commit, d.Ref) || strings.HasPrefix(d.Ref, commit)) {
			continue
		}
		t, err := time.Parse(time.RFC3339Nano, d.At)
		if err != nil {
			continue
		}
		if diff := t.Sub(started); diff >= -10*time.Minute && diff <= 10*time.Minute {
			updated[i].At = started.UTC().Format(time.RFC3339)
			updated[i].Detail += fmt.Sprintf("; serve-http pid %d on %s", rec.PID, rec.Addr)
			return nil, updated
		}
	}
	c := Change{Kind: ChangeRestart, At: started.UTC().Format(time.RFC3339), Title: "Restarted", Ref: commit, Source: source,
		Detail: fmt.Sprintf("serve-http pid %d on %s", rec.PID, rec.Addr)}
	if commit != "" {
		c.Detail += ", binary " + shortCommit(commit)
	}
	return &c, updated
}

// ReadingsLogFormat is the `git log` format ParseReadings reads: a NUL, the
// commit, a tab and its committer date, then the files it added.
const ReadingsLogFormat = "%x00%H%x09%cI"

// ParseReadings turns `git log --diff-filter=A --name-only
// --format=ReadingsLogFormat` output into measurement changes: each reading
// file (docs/bench/readings/*.md, docs/measurements/*.md; not READMEs)
// dated by the commit that added it. title names a reading from its path.
func ParseReadings(service string, gitLog []byte, title func(path string) string) []Change {
	var out []Change
	for _, block := range strings.Split(string(gitLog), "\x00") {
		lines := strings.Split(strings.TrimSpace(block), "\n")
		if len(lines) < 2 {
			continue
		}
		commit, date, ok := strings.Cut(lines[0], "\t")
		if !ok {
			continue
		}
		t, err := time.Parse(time.RFC3339, strings.TrimSpace(date))
		if err != nil {
			continue
		}
		for _, f := range lines[1:] {
			f = strings.TrimSpace(f)
			dir := path.Dir(f)
			if (dir != "docs/bench/readings" && dir != "docs/measurements") || path.Ext(f) != ".md" || strings.EqualFold(path.Base(f), "README.md") {
				continue
			}
			name := strings.TrimSuffix(path.Base(f), ".md")
			if title != nil {
				if s := title(f); s != "" {
					name = s
				}
			}
			out = append(out, Change{Kind: ChangeMeasurement, At: t.UTC().Format(time.RFC3339), Service: service, Title: "Measured",
				Detail: service + " · " + name, Ref: f, Source: "git log of " + dir + " (commit " + shortCommit(commit) + ")"})
		}
	}
	return out
}

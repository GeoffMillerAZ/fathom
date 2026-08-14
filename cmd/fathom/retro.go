package main

// retro.go — IR-2 (docs/design/intervention-outcomes.md): the mechanical
// session retro. Joins a session's intervention firings (the beat log)
// against what is on disk afterward, so "did the firing help" becomes an
// answerable question instead of a count. No LLM, no daemon dependency;
// the priced judgment lane is IR-3 (`assess`), not this verb.
//
// Honesty rules, binding: firings without a session/file stamp are counted
// and NAMED as unjoinable, never dropped; an OFF-arm session says its
// channels were silent by design; the retro never claims causality — the
// arm carries that (law 12).

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/custodian"
)

type retroBeat struct {
	Channel string            `json:"channel"`
	Kind    string            `json:"kind"`
	At      time.Time         `json:"at"`
	Meta    map[string]string `json:"meta"`
}

type retroFiringOutcome struct {
	At      string `json:"at"`
	Channel string `json:"channel"` // guard | obligations | custodian
	File    string `json:"file,omitempty"`
	Outcome string `json:"outcome"` // committed | modified-uncommitted | untouched | (custodian) remedied | unremedied
}

type retroModel struct {
	Session      string               `json:"session"`
	ProjectRoot  string               `json:"project_root"`
	WindowStart  string               `json:"window_start"`
	WindowEnd    string               `json:"window_end"`
	Arm          string               `json:"arm,omitempty"`
	Firings      []retroFiringOutcome `json:"firings"`
	GuardFires   int                  `json:"guard_fires"`
	Injections   int                  `json:"obligation_injections"`
	SilentClean  int                  `json:"answered_silent"` // answered with emitted=false — the zero-output-on-clean discipline working
	Unjoinable   int                  `json:"unjoinable"`      // firings in the window without session/file stamps (pre-IR-1)
	Discovery    int                  `json:"discovery_beats"`
	ViolatedNow  int                  `json:"concerns_violated_now"`
	AttentionHit bool                 `json:"custodian_attention_fired"`
}

func cmdRetro(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("retro", stderr)
	session := fs.String("session", "", "session id (default: the most recent stamped session for --project)")
	project := fs.String("project", ".", "project root")
	asJSON := fs.Bool("json", false, "emit JSON instead of text")
	write := fs.Bool("write", false, "also write the retro under workspace/artifacts/retros/")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	root, err := filepath.Abs(*project)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	beats, err := readRetroBeats(beatLogPath())
	if err != nil {
		fmt.Fprintf(stderr, "error: reading beat log: %v\n", err)
		return 1
	}
	var projBeats []retroBeat
	for _, b := range beats {
		if b.Meta["project_dir"] == root {
			projBeats = append(projBeats, b)
		}
	}
	if len(projBeats) == 0 {
		fmt.Fprintf(stdout, "no firings recorded for %s — nothing to retro (the beat log has no rows for this project; hooks may not be wired here, or the log rotated past the window).\n", root)
		return 0
	}

	sid := *session
	if sid == "" {
		for i := len(projBeats) - 1; i >= 0; i-- {
			if s := projBeats[i].Meta["session"]; s != "" {
				sid = s
				break
			}
		}
		if sid == "" {
			fmt.Fprintf(stdout, "no session-stamped firings for %s — %d beat(s) exist but all predate per-session stamping (IR-1); re-run after a session on the current binary.\n", root, len(projBeats))
			return 0
		}
	}

	var inSession []retroBeat
	for _, b := range projBeats {
		if b.Meta["session"] == sid {
			inSession = append(inSession, b)
		}
	}
	if len(inSession) == 0 {
		fmt.Fprintf(stdout, "no firings stamped with session %s for %s.\n", sid, root)
		return 0
	}
	sort.Slice(inSession, func(i, j int) bool { return inSession[i].At.Before(inSession[j].At) })
	start, end := inSession[0].At, inSession[len(inSession)-1].At

	m := retroModel{
		Session: sid, ProjectRoot: root,
		WindowStart: start.UTC().Format(time.RFC3339),
		WindowEnd:   end.UTC().Format(time.RFC3339),
		Arm:         ledgerArmFor(sid),
	}

	committed, modified := gitAftermath(root, start)

	for _, b := range projBeats {
		inWindow := !b.At.Before(start) && !b.At.After(end.Add(time.Minute))
		switch {
		case b.Meta["session"] == sid && b.Channel == "edit-hook" && b.Kind == "answered":
			if b.Meta["emitted"] != "true" {
				m.SilentClean++
				continue
			}
			ch := "obligations"
			if b.Meta["phase"] == "pre" {
				ch = "guard"
				m.GuardFires++
			} else {
				m.Injections++
			}
			out := retroFiringOutcome{At: b.At.UTC().Format(time.RFC3339), Channel: ch}
			if f := b.Meta["file"]; f != "" {
				rel := f
				if r, err := filepath.Rel(root, f); err == nil && !strings.HasPrefix(r, "..") {
					rel = filepath.ToSlash(r)
				}
				out.File = rel
				switch {
				case committed[rel]:
					out.Outcome = "committed"
				case modified[rel]:
					out.Outcome = "modified-uncommitted"
				default:
					out.Outcome = "untouched"
				}
			} else {
				out.Outcome = "unjoinable (no file stamp)"
				m.Unjoinable++
			}
			m.Firings = append(m.Firings, out)
		case b.Meta["session"] == sid && b.Channel == "custodian" && b.Kind == "attention":
			m.AttentionHit = true
		case b.Meta["session"] == sid && b.Channel == "discovery":
			m.Discovery++
		case inWindow && b.Meta["session"] == "" && b.Channel == "edit-hook" && b.Kind == "answered" && b.Meta["emitted"] == "true":
			// A firing inside this session's window that carries no session
			// stamp (pre-IR-1 binary): counted, named, never dropped.
			m.Unjoinable++
		}
	}

	// Custodian now-state: an attention line that fired is only half the
	// story — has anyone (the hand) remedied it since?
	concerns, loadErrs := loadConcernsBoth(root)
	if len(loadErrs) == 0 && len(concerns) > 0 {
		for _, f := range custodian.Audit(root, concerns) {
			if f.Status == custodian.StatusViolated {
				m.ViolatedNow++
			}
		}
		if m.AttentionHit {
			outcome := "remedied since"
			if m.ViolatedNow > 0 {
				outcome = fmt.Sprintf("unremedied — %d violation(s) still standing", m.ViolatedNow)
			}
			m.Firings = append(m.Firings, retroFiringOutcome{Channel: "custodian", Outcome: outcome})
		}
	}

	if *asJSON {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(m); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
	} else {
		renderRetro(stdout, m)
	}
	if *write {
		if p, err := writeRetroArtifact(root, m); err != nil {
			fmt.Fprintf(stderr, "warning: could not write retro artifact: %v\n", err)
		} else {
			fmt.Fprintf(stdout, "\nretro written: %s\n", p)
		}
	}
	return 0
}

func renderRetro(w io.Writer, m retroModel) {
	fmt.Fprintf(w, "retro for session %s in %s (%s → %s)\n", m.Session, m.ProjectRoot, m.WindowStart, m.WindowEnd)
	if m.Arm == "off" {
		fmt.Fprintf(w, "  OFF arm: channels were silent by design this session — absent firings are the experiment, not adoption failure.\n")
	}
	fmt.Fprintf(w, "  %d guard fire(s), %d obligation injection(s), %d silent-on-clean answer(s), %d discovery beat(s)\n",
		m.GuardFires, m.Injections, m.SilentClean, m.Discovery)
	for _, f := range m.Firings {
		if f.File != "" {
			fmt.Fprintf(w, "  %-11s %-40s → %s\n", f.Channel, f.File, f.Outcome)
		} else {
			fmt.Fprintf(w, "  %-11s %s\n", f.Channel, f.Outcome)
		}
	}
	if m.Unjoinable > 0 {
		fmt.Fprintf(w, "  %d firing(s) unjoinable (no session/file stamp — recorded before IR-1); counted, never dropped.\n", m.Unjoinable)
	}
	fmt.Fprintf(w, "\nOutcome-shaped evidence, not causality: the arm carries causality (law 12); `fathom assess` (IR-3) judges compliance quality.\n")
}

// readRetroBeats tolerates an absent log (a wired-but-quiet machine) and
// skips malformed lines rather than failing the whole read — one bad row
// must not hide every good one.
func readRetroBeats(path string) ([]retroBeat, error) {
	if path == "" {
		return nil, nil
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []retroBeat
	for _, ln := range strings.Split(string(data), "\n") {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		var b retroBeat
		if json.Unmarshal([]byte(ln), &b) == nil && b.Channel != "" {
			out = append(out, b)
		}
	}
	return out, nil
}

// gitAftermath returns the repo-relative files committed since start and
// the files currently modified-but-uncommitted. Both empty on any git
// failure — the retro then reports "untouched", which is the conservative
// direction (it under-claims impact, never over-claims).
func gitAftermath(root string, start time.Time) (committed, modified map[string]bool) {
	committed, modified = map[string]bool{}, map[string]bool{}
	log := exec.Command("git", "-C", root, "log", "--since="+start.UTC().Format(time.RFC3339), "--name-only", "--pretty=format:")
	if raw, err := log.Output(); err == nil {
		for _, ln := range strings.Split(string(raw), "\n") {
			if ln = strings.TrimSpace(ln); ln != "" {
				committed[filepath.ToSlash(ln)] = true
			}
		}
	}
	st := exec.Command("git", "-C", root, "status", "--porcelain")
	if raw, err := st.Output(); err == nil {
		for _, ln := range strings.Split(string(raw), "\n") {
			if len(ln) > 3 {
				modified[filepath.ToSlash(strings.TrimSpace(ln[3:]))] = true
			}
		}
	}
	return committed, modified
}

// ledgerArmFor returns the arm label of the session's last ledger row, or
// "" when the ledger has none — absence, not a guess.
func ledgerArmFor(sid string) string {
	arm := ""
	for _, e := range dedupedLedgerRows() {
		if e.SessionID == sid {
			arm = e.Arm
		}
	}
	return arm
}

func writeRetroArtifact(root string, m retroModel) (string, error) {
	dir := filepath.Join(root, "workspace", "artifacts", "retros")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := m.Session
	if len(name) > 8 {
		name = name[:8]
	}
	p := filepath.Join(dir, name+".json")
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return "", err
	}
	return p, os.WriteFile(p, data, 0o644)
}

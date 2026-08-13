package main

// purgecmd.go — SP-7/SP-8/SP-9: `fathom purge`, the IRREVERSIBLE half of
// docs/design/scope-and-purge.md.
//
// The verb runs §3.3's four phases and refuses to shortcut any of them:
//
//	1. resolve   → expand the pattern against EACH LOCATION'S OWN path
//	               column, show the file list and per-table counts, require
//	               confirmation (--yes, or --dry-run to stop here)
//	2. execute   → delete across every location, on a secure_delete
//	               connection (store.PurgePaths), plus the derived stores —
//	               including <repo>/.fathom/memories.jsonl when the
//	               service's root is known (AM-12)
//	3. verify    → a scan re-derived from the path columns PLUS a raw-bytes
//	               probe of the store file; verifyPurge maps the tri-state
//	               verdict onto exit codes — and "SQL zero but bytes
//	               unchecked" is its own named, NON-ZERO state (V1), never
//	               printed as VERIFIED
//	4. report    → per-location before/after, the retentions, the
//	               verification result, appended to the purge log
//
// And it writes an INGEST-time exclusion, not only a query-layer
// suppression (§3.5) — but SERVICE-SCOPED (V2/V4): the pattern lands in
// ~/.fathom/ignore.d/<service>, which Discover unions only for THAT
// service. The first build appended to the shared ~/.fathom/ignore, where
// `--all`'s bare `**` unioned into every repository and destroyed the
// whole fleet index on the next refresh. connector.AppendServiceIgnore
// refuses a fleet-wide write outright, and the report DISCLOSES the
// exclusion's scope in words.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/filter"
	"github.com/geoffmilleraz/fathom/internal/memory"
	"github.com/geoffmilleraz/fathom/internal/scope"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// purgesFileName is the append-only run log's basename (§3.3 phase 4);
// $FATHOM_PURGES overrides the full path (the FATHOM_SESSION_LEDGER
// pattern, so tests never touch the real ~/.fathom).
const purgesFileName = "purges.jsonl"

func purgesPath() (string, error) {
	if p := os.Getenv("FATHOM_PURGES"); p != "" {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", fmt.Errorf("cannot resolve $HOME")
	}
	return filepath.Join(home, ".fathom", purgesFileName), nil
}

// purgeBetweenExecuteAndVerify is the test-only seam for SP-7's
// artificially-surviving row; nil in production, and deliberately not
// armed by any init.
var purgeBetweenExecuteAndVerify func()

// purgeRecord is one line of the purge log — "what has been purged, when,
// and whether verified". Verdict is the tri-state; there is no boolean to
// flatten "bytes unchecked" into success.
type purgeRecord struct {
	At          string            `json:"at"`
	Service     string            `json:"service"`
	Patterns    []string          `json:"patterns"`
	Reason      string            `json:"reason"`
	Files       int               `json:"files"`
	Verdict     string            `json:"verdict"`
	VerdictNote string            `json:"verdict_note"`
	Before      map[string]int    `json:"before"`
	After       map[string]int    `json:"after"`
	Retained    map[string]int    `json:"retained,omitempty"`
	Derived     map[string]string `json:"derived,omitempty"`
	HistoryNote string            `json:"history_note,omitempty"`
	Note        string            `json:"note"`
}

func cmdPurge(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "history" {
		return cmdPurgeHistory(args[1:], stdout, stderr)
	}

	operands, rest := splitLeadingOperands(args)
	fs := newFlagSet("purge", stderr)
	service := fs.String("service", "", "service ID to purge from (REQUIRED)")
	storePath := fs.String("store", "workspace/db/fathom.sqlite", "store path (.sqlite/.db)")
	reason := fs.String("reason", "", "why this content is being destroyed (REQUIRED)")
	dryRun := fs.Bool("dry-run", false, "resolve and report only; touch nothing")
	yes := fs.Bool("yes", false, "confirm: this deletes data and cannot be undone")
	all := fs.Bool("all", false, "purge EVERY path in the service (equivalent to the glob '**')")
	includeHistory := fs.Bool("include-history", false, "also delete claim_events for the affected claims (breaks claim-history integrity)")
	force := fs.Bool("force", false, "second key required by --include-history")
	asJSON := fs.Bool("json", false, "emit JSON instead of a report")
	var probes stringList
	fs.Var(&probes, "probe", "extra literal string to hunt for in the raw store bytes (repeatable) — e.g. the leaked credential itself")
	if err := fs.Parse(rest); err != nil {
		return 1
	}

	patterns := append(operands, fs.Args()...)
	if *all {
		patterns = append(patterns, "**")
	}
	if len(patterns) == 0 {
		fmt.Fprintln(stderr, "usage: fathom purge '<glob>' --service S --reason '<why>' [--dry-run] [--yes]")
		fmt.Fprintln(stderr, "       fathom purge --service S --all --reason '<why>' --yes")
		fmt.Fprintln(stderr, "       fathom purge history")
		return 1
	}
	if *service == "" {
		fmt.Fprintln(stderr, "error: --service is required (a fleet-wide purge is an invitation to an accident)")
		return 1
	}
	if strings.TrimSpace(*reason) == "" {
		fmt.Fprintln(stderr, "error: --reason is required — this destroys data and the record has to say why")
		return 1
	}
	for _, p := range patterns {
		if strings.HasPrefix(strings.TrimSpace(p), "!") {
			fmt.Fprintf(stderr, "error: %q is a re-include pattern; purge takes exclusions only\n", p)
			return 1
		}
	}
	if *includeHistory && !*force {
		fmt.Fprintln(stderr, "error: --include-history deletes the append-only claim_events log for the affected claims, "+
			"breaking claim-history integrity; pass --force as well if that is what you mean (§3.4)")
		return 1
	}

	matcher := filter.New(patterns)

	// ---- phase 1: resolve, and stop here unless confirmed ----
	if *dryRun || !*yes {
		return purgeResolveOnly(*storePath, *service, patterns, matcher, *dryRun, *asJSON, stdout, stderr)
	}

	raw, err := store.BackendForPath(*storePath)
	if err != nil {
		fmt.Fprintf(stderr, "error: open store %q: %v\n", *storePath, err)
		return 1
	}
	defer raw.Close()
	sq, ok := raw.(*store.SQLiteStore)
	if !ok {
		fmt.Fprintf(stderr, "error: path-scoped purge needs the SQLite backend (got %T)\n", raw)
		return 1
	}

	// The memories location (AM-12): reachable only when the service's
	// repo root is on record; otherwise the report names the absence.
	var memPurger store.MemoryPurger
	if meta, found, err := sq.GetServiceMeta(context.Background(), *service); err == nil && found && meta.Path != "" {
		memPurger = memoriesAdapter{fs: memory.NewFileStore(memory.Path(meta.Path))}
	}

	// ---- phase 2 + 3: execute and verify ----
	rep, err := sq.PurgePaths(context.Background(), *service, matcher, store.PurgeOptions{
		IncludeHistory: *includeHistory,
		Force:          *force,
		Probes:         probes,
		Memories:       memPurger,
		// Cache and Vectors are deliberately left nil here: this process
		// holds neither, and the report says so by name rather than
		// reporting a comforting zero (law 3). The daemon's own purge
		// path passes its live instances.
		BetweenExecuteAndVerify: purgeBetweenExecuteAndVerify,
	})
	if err != nil {
		fmt.Fprintf(stderr, "error: purge: %v\n", err)
		return 1
	}

	// ---- phase 3b: the ingest-time exclusion (§3.5), SERVICE-SCOPED ----
	wrote, ignorePath, err := connector.AppendServiceIgnore(*service, patterns, "purged: "+*reason)
	if err != nil {
		fmt.Fprintf(stderr, "error: writing the ingest-time exclusion: %v\n", err)
		fmt.Fprintln(stderr, "       the rows are deleted, but the next ingest will re-index them — fix this before trusting the purge")
		return 1
	}

	// A suppression too, so the content is hidden the instant the purge
	// runs even from a process that opened its store before the delete.
	if p := scope.ResolvePath(); p != "" {
		for _, pat := range patterns {
			if _, err := scope.AppendSuppression(p, scope.Suppression{
				Service:   *service,
				Pattern:   pat,
				Reason:    "purged: " + *reason,
				CreatedAt: time.Now().UTC(),
				CreatedBy: scope.OriginPurge,
			}); err != nil {
				fmt.Fprintf(stderr, "warning: could not record the read-side suppression: %v\n", err)
			}
		}
	}

	// ---- phase 4: report ----
	if err := recordPurge(rep, patterns, *reason); err != nil {
		fmt.Fprintf(stderr, "warning: could not append to the purge log: %v\n", err)
	}
	if *asJSON {
		if rc := emitScopeJSON(stdout, stderr, rep); rc != 0 {
			return rc
		}
	} else {
		renderPurgeReport(stdout, rep, patterns, wrote, ignorePath)
	}
	return verifyPurge(rep, stdout, stderr)
}

// memoriesAdapter satisfies store.MemoryPurger over internal/memory's
// FileStore.
type memoriesAdapter struct{ fs *memory.FileStore }

func (a memoriesAdapter) PurgeMemories(m store.PathMatcher) (int, error) {
	return a.fs.PurgeMatching(m)
}

// verifyPurge turns the store's tri-state verdict into the process's exit
// code. A partial purge that reports success is worse than a failed one,
// because the user stops looking (§3.3) — so anything short of fully
// verified is a loud, non-zero exit. The V1 rework's core: "SQL zero,
// bytes unchecked" is its own named state with its own exit code (2), and
// this function has no way to print VERIFIED for it, because the verdict
// type carries the probe's outcome.
func verifyPurge(rep *store.PurgeReport, stdout, stderr io.Writer) int {
	switch rep.Verdict {
	case store.PurgeVerified:
		fmt.Fprintf(stdout, "\nVERIFIED: %s\n", rep.VerdictNote)
		return 0
	case store.PurgeBytesUnchecked:
		fmt.Fprintf(stderr, "\nPURGE NOT VERIFIED — BYTES UNCHECKED: %s\n", rep.VerdictNote)
		for _, sk := range rep.Physical.Probe.Skipped {
			fmt.Fprintf(stderr, "  not probed: %s — %s\n", sk.Needle, sk.Reason)
		}
		fmt.Fprintln(stderr, "  the SQL rows are gone, but no raw-bytes probe confirmed the store file; re-run with --probe '<literal>' to check the content itself")
		return 2
	default:
		fmt.Fprintf(stderr, "\nPURGE FAILED VERIFICATION: %s\n", rep.VerdictNote)
		for _, sv := range rep.Survivors {
			fmt.Fprintf(stderr, "  survivor: %s (%d row(s))\n", sv.Location, sv.Rows)
		}
		for _, h := range rep.Physical.Probe.Hits {
			fmt.Fprintf(stderr, "  raw bytes: %q still present in %s at offset %d\n", h.Needle, h.File, h.Offset)
		}
		fmt.Fprintln(stderr, "  the purge is NOT complete; do not treat this content as removed")
		return 1
	}
}

// purgeResolveOnly is phase 1 standing alone: the exact file list and the
// per-table row counts, with nothing touched. Without --yes it is also
// what an unconfirmed run does, and it exits non-zero — a purge that
// executed because the operator forgot a flag is the accident this whole
// design is shaped around.
func purgeResolveOnly(storePath, service string, patterns []string, m store.PathMatcher, dryRun, asJSON bool, stdout, stderr io.Writer) int {
	raw, err := store.BackendForPath(storePath)
	if err != nil {
		fmt.Fprintf(stderr, "error: open store %q: %v\n", storePath, err)
		return 1
	}
	defer raw.Close()
	sq, ok := raw.(*store.SQLiteStore)
	if !ok {
		fmt.Fprintf(stderr, "error: path-scoped purge needs the SQLite backend (got %T)\n", raw)
		return 1
	}

	plan, err := sq.ResolvePurge(context.Background(), service, m)
	if err != nil {
		fmt.Fprintf(stderr, "error: resolve: %v\n", err)
		return 1
	}
	counts := map[string]int{}
	for _, c := range plan.Counts {
		counts[c.Location] = c.Rows
	}
	if asJSON {
		if rc := emitScopeJSON(stdout, stderr, struct {
			Service  string         `json:"service"`
			Patterns []string       `json:"patterns"`
			Files    []string       `json:"files"`
			Counts   map[string]int `json:"counts"`
			DryRun   bool           `json:"dry_run"`
		}{service, patterns, plan.Paths, counts, dryRun}); rc != 0 {
			return rc
		}
	} else {
		fmt.Fprintf(stdout, "purge plan for %s: %v\n", service, patterns)
		fmt.Fprintf(stdout, "  %d file(s) resolved, from every location's own path column:\n", len(plan.Paths))
		for _, p := range plan.Paths {
			fmt.Fprintf(stdout, "    %s\n", p)
		}
		fmt.Fprintln(stdout, "  rows that would be deleted:")
		for _, c := range plan.Counts {
			fmt.Fprintf(stdout, "    %-26s %d\n", c.Location, c.Rows)
		}
	}
	if dryRun {
		return 0
	}
	fmt.Fprintln(stderr, "\nerror: refusing to purge without confirmation — re-run with --yes once the file list above is what you meant.")
	fmt.Fprintln(stderr, "       This DELETES data. It cannot be undone; `fathom scope suppress` is the reversible option.")
	return 1
}

func renderPurgeReport(stdout io.Writer, rep *store.PurgeReport, patterns []string, wroteIgnore bool, ignorePath string) {
	fmt.Fprintf(stdout, "purged %v from %s\n", patterns, rep.ServiceID)
	fmt.Fprintf(stdout, "  %d file(s) resolved\n\n", len(rep.Resolved))
	before := map[string]int{}
	for _, c := range rep.Before {
		before[c.Location] = c.Rows
	}
	fmt.Fprintf(stdout, "  %-26s %8s %8s\n", "location", "before", "after")
	for _, c := range rep.After {
		fmt.Fprintf(stdout, "  %-26s %8d %8d\n", c.Location, before[c.Location], c.Rows)
	}
	for _, r := range rep.Retained {
		fmt.Fprintf(stdout, "\n  retained: %s (%d row(s)) — %s\n", r.Location, r.Rows, r.Reason)
	}
	if rep.HistoryNote != "" {
		fmt.Fprintf(stdout, "\n  %s\n", rep.HistoryNote)
	}
	for _, d := range rep.Derived {
		state := "cleared"
		if !d.Applied {
			state = "ABSENT"
		}
		if d.Failed {
			state = "FAILED"
		}
		fmt.Fprintf(stdout, "\n  %s: %s — %s", d.Location, state, d.Reason)
	}
	fmt.Fprintln(stdout)
	physical := "applied"
	if !rep.Physical.Applied {
		physical = "NOT applied: " + rep.Physical.Reason
	}
	fmt.Fprintf(stdout, "\n  physical: %s (secure_delete=%v checkpoint=%v vacuum=%v, %d -> %d bytes)\n",
		physical, rep.Physical.SecureDelete, rep.Physical.Checkpointed, rep.Physical.Vacuumed,
		rep.Physical.BytesBefore, rep.Physical.BytesAfter)
	if rep.Physical.Probe.Ran {
		fmt.Fprintf(stdout, "  raw-bytes probe: %d needle(s) across %s — %d hit(s)\n",
			len(rep.Physical.Probe.Needles), strings.Join(rep.Physical.Probe.Files, ", "), len(rep.Physical.Probe.Hits))
	} else {
		fmt.Fprintf(stdout, "  raw-bytes probe: NOT RUN — %s\n", rep.Physical.Probe.Reason)
	}
	for _, sk := range rep.Physical.Probe.Skipped {
		fmt.Fprintf(stdout, "    not probed: %s — %s\n", sk.Needle, sk.Reason)
	}
	// V4: the exclusion's scope is DISCLOSED in words, not implied. The
	// first build wrote the pattern fleet-wide and said nothing; a reader
	// of this report must know exactly how far the exclusion reaches.
	if wroteIgnore {
		fmt.Fprintf(stdout, "\n  ingest-time exclusion: %v recorded in %s —\n", patterns, ignorePath)
	} else {
		fmt.Fprintf(stdout, "\n  ingest-time exclusion: already present in %s —\n", ignorePath)
	}
	fmt.Fprintf(stdout, "  future ingests of service %q will skip these paths; the exclusion is scoped to this service only, and every other service is unaffected\n", rep.ServiceID)
	fmt.Fprintf(stdout, "  note: %s\n", rep.Note)
}

func recordPurge(rep *store.PurgeReport, patterns []string, reason string) error {
	path, err := purgesPath()
	if err != nil {
		return err
	}
	rec := purgeRecord{
		At:          time.Now().UTC().Format(time.RFC3339),
		Service:     rep.ServiceID,
		Patterns:    patterns,
		Reason:      reason,
		Files:       len(rep.Resolved),
		Verdict:     string(rep.Verdict),
		VerdictNote: rep.VerdictNote,
		Before:      map[string]int{},
		After:       map[string]int{},
		HistoryNote: rep.HistoryNote,
		Note:        rep.Note,
	}
	for _, c := range rep.Before {
		rec.Before[c.Location] = c.Rows
	}
	for _, c := range rep.After {
		rec.After[c.Location] = c.Rows
	}
	if len(rep.Retained) > 0 {
		rec.Retained = map[string]int{}
		for _, r := range rep.Retained {
			rec.Retained[string(r.Location)] = r.Rows
		}
	}
	if len(rep.Derived) > 0 {
		rec.Derived = map[string]string{}
		for _, d := range rep.Derived {
			state := "cleared"
			if !d.Applied {
				state = "absent: " + d.Reason
			}
			if d.Failed {
				state = "failed: " + d.Reason
			}
			rec.Derived[string(d.Location)] = state
		}
	}
	line, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(line, '\n'))
	return err
}

func cmdPurgeHistory(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("purge history", stderr)
	asJSON := fs.Bool("json", false, "emit JSON instead of a table")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	path, err := purgesPath()
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			if *asJSON {
				return emitScopeJSON(stdout, stderr, []purgeRecord{})
			}
			fmt.Fprintln(stdout, "no purges have been run on this machine")
			return 0
		}
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	var recs []purgeRecord
	for i, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var r purgeRecord
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			fmt.Fprintf(stderr, "error: %s:%d: %v\n", path, i+1, err)
			return 1
		}
		recs = append(recs, r)
	}
	sort.SliceStable(recs, func(i, j int) bool { return recs[i].At < recs[j].At })
	if *asJSON {
		if recs == nil {
			recs = []purgeRecord{}
		}
		return emitScopeJSON(stdout, stderr, recs)
	}
	fmt.Fprintf(stdout, "%d purge(s) recorded in %s\n\n", len(recs), path)
	for _, r := range recs {
		mark := strings.ToUpper(r.Verdict)
		if r.Verdict == string(store.PurgeVerified) {
			mark = "VERIFIED"
		}
		fmt.Fprintf(stdout, "  %s  %-12s %v  %d file(s)  [%s]\n", r.At, r.Service, r.Patterns, r.Files, mark)
		fmt.Fprintf(stdout, "      %q\n", r.Reason)
		if r.Verdict != string(store.PurgeVerified) {
			fmt.Fprintf(stdout, "      %s\n", r.VerdictNote)
		}
	}
	return 0
}

// stringList is a repeatable string flag.
type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ",") }

func (s *stringList) Set(v string) error {
	*s = append(*s, v)
	return nil
}

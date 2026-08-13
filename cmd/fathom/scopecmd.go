package main

// scopecmd.go — SP-4: `fathom scope list|suppress|unsuppress|preview`, the
// REVERSIBLE half of docs/design/scope-and-purge.md.
//
// preview exists because a glob that matches more than you think is the main
// way this feature hurts someone (§2.4). Its count is not an estimate: it is
// computed by store.ResolvePurge — the same per-location resolution purge
// plans with, normalized by the same scope.NormalizeRef the read filter
// matches through — so "preview then suppress" hides exactly what preview
// promised, and it reads raw SQL, past any suppression already in force (a
// preview computed through the filter under-reports precisely the paths
// already hidden).
//
// State lives where scope.ResolvePath says: $FATHOM_SUPPRESSIONS when set
// (the hermetic/test channel, and what the store's own read gate consults),
// else ~/.fathom/suppressions.jsonl.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/filter"
	"github.com/geoffmilleraz/fathom/internal/scope"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func cmdScope(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return cmdScopeList(nil, stdout, stderr)
	}
	switch args[0] {
	case "list":
		return cmdScopeList(args[1:], stdout, stderr)
	case "suppress":
		return cmdScopeSuppress(args[1:], stdout, stderr)
	case "unsuppress":
		return cmdScopeUnsuppress(args[1:], stdout, stderr)
	case "preview":
		return cmdScopePreview(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "error: unknown scope subcommand %q\n", args[0])
		fmt.Fprintln(stderr, "usage: fathom scope list|suppress <glob> --reason R|unsuppress <id>|preview <glob>")
		return 1
	}
}

// suppressionsPath resolves the suppression file, refusing loudly when
// neither $FATHOM_SUPPRESSIONS nor $HOME resolves — a suppression written
// nowhere is worse than one refused.
func suppressionsPath(stderr io.Writer) (string, bool) {
	p := scope.ResolvePath()
	if p == "" {
		fmt.Fprintln(stderr, "error: cannot resolve a suppressions file (set FATHOM_SUPPRESSIONS or $HOME) — suppressions are user-scope by design (law 13)")
		return "", false
	}
	return p, true
}

type scopeListEntry struct {
	ID        string `json:"id"`
	Service   string `json:"service,omitempty"`
	Pattern   string `json:"pattern"`
	Reason    string `json:"reason"`
	State     string `json:"state"`
	CreatedAt string `json:"created_at"`
	CreatedBy string `json:"created_by"`
	ExpiresAt string `json:"expires_at,omitempty"`
	RemovedAt string `json:"removed_at,omitempty"`
}

func cmdScopeList(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("scope list", stderr)
	service := fs.String("service", "", "only entries scoped to this service (or fleet-wide)")
	asJSON := fs.Bool("json", false, "emit JSON instead of a table")
	all := fs.Bool("all", false, "include expired and removed entries (they are retained as history)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	path, ok := suppressionsPath(stderr)
	if !ok {
		return 1
	}
	sups, err := scope.LoadSuppressions(path)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	now := time.Now()

	entries := make([]scopeListEntry, 0, len(sups))
	for _, s := range sups {
		state := scope.SuppressionState(s, now)
		if !*all && state != scope.StateActive {
			continue
		}
		if *service != "" && s.Service != "" && s.Service != *service {
			continue
		}
		e := scopeListEntry{
			ID: s.ID, Service: s.Service, Pattern: s.Pattern, Reason: s.Reason,
			State: string(state), CreatedAt: s.CreatedAt.UTC().Format(time.RFC3339),
			CreatedBy: s.CreatedBy,
		}
		if s.ExpiresAt != nil {
			e.ExpiresAt = s.ExpiresAt.UTC().Format(time.RFC3339)
		}
		if s.RemovedAt != nil {
			e.RemovedAt = s.RemovedAt.UTC().Format(time.RFC3339)
		}
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].ID < entries[j].ID })

	if *asJSON {
		return emitScopeJSON(stdout, stderr, entries)
	}
	if len(entries) == 0 {
		fmt.Fprintln(stdout, "no active suppressions (nothing is hidden from answers on this machine)")
		return 0
	}
	fmt.Fprintf(stdout, "%d suppression(s) in %s\n\n", len(entries), path)
	for _, e := range entries {
		svc := e.Service
		if svc == "" {
			svc = "(fleet-wide)"
		}
		when := "no expiry"
		if e.ExpiresAt != "" {
			when = "expires " + e.ExpiresAt
		}
		fmt.Fprintf(stdout, "  %s  %-24s %-14s %s [%s]\n", e.ID, e.Pattern, svc, when, e.State)
		fmt.Fprintf(stdout, "             %q (%s)\n", e.Reason, e.CreatedBy)
	}
	return 0
}

func cmdScopeSuppress(args []string, stdout, stderr io.Writer) int {
	operands, rest := splitLeadingOperands(args)
	fs := newFlagSet("scope suppress", stderr)
	service := fs.String("service", "", "service ID to scope the suppression to (default: fleet-wide)")
	reason := fs.String("reason", "", "why this is hidden (REQUIRED)")
	until := fs.String("until", "", "time-box the suppression, e.g. 7d, 36h (default: until removed)")
	if err := fs.Parse(rest); err != nil {
		return 1
	}
	operands = append(operands, fs.Args()...)
	if len(operands) != 1 {
		fmt.Fprintln(stderr, "usage: fathom scope suppress '<glob>' --reason '<why>' [--service S] [--until 7d]")
		return 1
	}
	pattern := operands[0]
	if strings.TrimSpace(*reason) == "" {
		fmt.Fprintln(stderr, "error: --reason is required — a suppression with no reason is a mystery in three weeks")
		return 1
	}

	now := time.Now().UTC()
	s := scope.Suppression{
		Service:   *service,
		Pattern:   pattern,
		Reason:    *reason,
		CreatedAt: now,
		CreatedBy: scope.OriginCLI,
	}
	if *until != "" {
		d, err := parseScopeDuration(*until)
		if err != nil {
			fmt.Fprintf(stderr, "error: --until %q: %v\n", *until, err)
			return 1
		}
		exp := now.Add(d)
		s.ExpiresAt = &exp
	}

	path, ok := suppressionsPath(stderr)
	if !ok {
		return 1
	}
	id, err := scope.AppendSuppression(path, s)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "suppressed %s (id %s)\n", pattern, id)
	if s.ExpiresAt != nil {
		fmt.Fprintf(stdout, "  expires %s — evaluated at read time, so no sweep can miss it\n", s.ExpiresAt.Format(time.RFC3339))
	}
	fmt.Fprintln(stdout, "  this HIDES, it does not delete: `fathom scope unsuppress "+id+"` reverses it instantly.")
	return 0
}

func cmdScopeUnsuppress(args []string, stdout, stderr io.Writer) int {
	operands, rest := splitLeadingOperands(args)
	fs := newFlagSet("scope unsuppress", stderr)
	if err := fs.Parse(rest); err != nil {
		return 1
	}
	operands = append(operands, fs.Args()...)
	if len(operands) != 1 {
		fmt.Fprintln(stderr, "usage: fathom scope unsuppress <id>")
		return 1
	}
	path, ok := suppressionsPath(stderr)
	if !ok {
		return 1
	}
	if err := scope.RemoveSuppression(path, operands[0], time.Now().UTC()); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "unsuppressed %s (the record is retained as history)\n", operands[0])
	return 0
}

type scopePreview struct {
	Pattern string   `json:"pattern"`
	Service string   `json:"service,omitempty"`
	Files   []string `json:"files"`
	Count   int      `json:"count"`
	Symbols int      `json:"symbols"`
}

func cmdScopePreview(args []string, stdout, stderr io.Writer) int {
	operands, rest := splitLeadingOperands(args)
	fs := newFlagSet("scope preview", stderr)
	service := fs.String("service", "", "service ID (default: every service in the store)")
	storePath := fs.String("store", "workspace/db/fathom.sqlite", "store path (.sqlite/.db)")
	asJSON := fs.Bool("json", false, "emit JSON instead of a summary")
	sample := fs.Int("sample", 10, "how many matching files to print (0 = all)")
	if err := fs.Parse(rest); err != nil {
		return 1
	}
	operands = append(operands, fs.Args()...)
	if len(operands) != 1 {
		fmt.Fprintln(stderr, "usage: fathom scope preview '<glob>' [--service S] [--store P]")
		return 1
	}
	pattern := operands[0]

	st, err := store.BackendForPath(*storePath)
	if err != nil {
		fmt.Fprintf(stderr, "error: open store %q: %v\n", *storePath, err)
		return 1
	}
	defer st.Close()
	sq, ok := st.(*store.SQLiteStore)
	if !ok {
		fmt.Fprintf(stderr, "error: preview needs the SQLite backend (got %T)\n", st)
		return 1
	}

	services, err := previewServices(st, *service)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	m := filter.New([]string{pattern})
	fileSet := map[string]bool{}
	symbols := 0
	ctx := context.Background()
	for _, svc := range services {
		// ResolvePurge reads each location's own path column with raw SQL —
		// deliberately past the read filter, because preview answers "what
		// WOULD be hidden", including what already is.
		plan, err := sq.ResolvePurge(ctx, svc, m)
		if err != nil {
			fmt.Fprintf(stderr, "error: resolve %q: %v\n", svc, err)
			return 1
		}
		for _, p := range plan.Paths {
			fileSet[p] = true
		}
		for _, c := range plan.Counts {
			if c.Location == string(store.LocObservations) {
				symbols += c.Rows
			}
		}
	}

	files := make([]string, 0, len(fileSet))
	for f := range fileSet {
		files = append(files, f)
	}
	sort.Strings(files)

	p := scopePreview{Pattern: pattern, Service: *service, Files: files, Count: len(files), Symbols: symbols}
	if *asJSON {
		return emitScopeJSON(stdout, stderr, p)
	}
	fmt.Fprintf(stdout, "preview %q: %d file(s), %d symbol(s) would be hidden\n", pattern, p.Count, p.Symbols)
	shown := files
	if *sample > 0 && len(shown) > *sample {
		shown = shown[:*sample]
	}
	for _, f := range shown {
		fmt.Fprintf(stdout, "  %s\n", f)
	}
	if len(shown) < len(files) {
		fmt.Fprintf(stdout, "  … and %d more\n", len(files)-len(shown))
	}
	return 0
}

// previewServices returns the service ids a preview should scan: the one
// named, or every service that has content.
func previewServices(st store.Store, service string) ([]string, error) {
	if service != "" {
		return []string{service}, nil
	}
	ctx := context.Background()
	ids, err := st.ListServiceIDs(ctx)
	if err != nil {
		return nil, err
	}
	claimIDs, err := st.ClaimServiceIDs(ctx)
	if err != nil {
		return nil, err
	}
	set := map[string]bool{}
	for _, id := range append(ids, claimIDs...) {
		set[id] = true
	}
	out := make([]string, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	sort.Strings(out)
	return out, nil
}

// parseScopeDuration accepts Go durations plus a "d" (day) suffix, because
// "--until 7d" is what a human writes and time.ParseDuration refuses it.
func parseScopeDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if strings.HasSuffix(s, "d") {
		n, err := strconv.Atoi(strings.TrimSuffix(s, "d"))
		if err != nil {
			return 0, fmt.Errorf("not a number of days")
		}
		if n <= 0 {
			return 0, fmt.Errorf("must be positive")
		}
		return time.Duration(n) * 24 * time.Hour, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, err
	}
	if d <= 0 {
		return 0, fmt.Errorf("must be positive")
	}
	return d, nil
}

func emitScopeJSON(stdout, stderr io.Writer, v any) int {
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	return 0
}

// splitLeadingOperands separates the operands written BEFORE the first flag
// from the rest of the argument list.
//
// The design writes these verbs glob-first — `fathom purge '<glob>'
// --service S --reason '<why>'`, `fathom scope suppress '<glob>' --reason
// '<why>'` — which is how a human types them, and exactly what Go's flag
// package cannot parse: it stops at the first non-flag token and leaves
// every later flag unparsed, so `--service` would silently never be seen.
// Hoisting the leading operands out first makes both orders work, with no
// flag-value ambiguity: everything from the first "-" onward goes to
// flag.Parse untouched.
func splitLeadingOperands(args []string) (operands, rest []string) {
	i := 0
	for ; i < len(args); i++ {
		if strings.HasPrefix(args[i], "-") {
			break
		}
	}
	return args[:i], args[i:]
}

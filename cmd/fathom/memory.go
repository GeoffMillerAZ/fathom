package main

// memory.go — `fathom memory add|list|review|show` (docs/design/
// anchored-memory.md, AM-1..AM-5/AM-7 of docs/plans/ambient-fathom.md).
//
// Every verb opens the code store READ-ONLY (store.MountReadOnly, the same
// discipline cmd/fathom/findings.go and metrics_*.go use — a CLI query must
// never contend with the daemon's own writes) and keeps memories in the
// project-scoped `<root>/.fathom/memories.jsonl` (law 13: only project scope
// governs, and nothing leaves loopback).
//
// The interesting behaviour lives in internal/memory: the refusals, the
// structural tier derivation, and the read-time drift evaluation. This file
// is argument parsing and rendering, deliberately.

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/memory"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func cmdMemory(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "error: memory requires a verb: add | list | review | show")
		printMemoryUsage(stderr)
		return 1
	}
	switch args[0] {
	case "add":
		return cmdMemoryAdd(args[1:], stdout, stderr)
	case "list":
		return cmdMemoryList(args[1:], stdout, stderr)
	case "review":
		return cmdMemoryReview(args[1:], stdout, stderr)
	case "show":
		return cmdMemoryShow(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "error: unknown memory verb %q (want add | list | review | show)\n", args[0])
		printMemoryUsage(stderr)
		return 1
	}
}

func printMemoryUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  fathom memory add --store PATH --service NAME --claim TEXT --anchor KIND:REF [--anchor …] --evidence KIND:REF [--evidence …] [--tier T] [--confidence N] [--by ACTOR] [--root DIR] [--json]")
	fmt.Fprintln(w, "  fathom memory list --store PATH --service NAME [--state fresh|suspect|orphaned|retired] [--root DIR] [--json]")
	fmt.Fprintln(w, "  fathom memory review --store PATH --service NAME [--id ID --resolve still-true|no-longer-true|rewrite --by ACTOR [--claim TEXT] [--note TEXT]] [--root DIR] [--json]")
	fmt.Fprintln(w, "  fathom memory show --id ID [--root DIR] [--json]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Anchor kinds (sharpest first): symbol | file | commit | claim | path")
	fmt.Fprintln(w, "Evidence kinds → tier:")
	for _, line := range memory.EvidenceHelp() {
		fmt.Fprintf(w, "  %s\n", line)
	}
	fmt.Fprintln(w, "Actors: human | model | machine (optionally kind:name) — who confirms decides how far the tier can rise.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "A memory with no anchor is refused: "+memory.UnanchoredRemedy)
}

// memoryEnv is everything the verbs share: the memory store, the anchor
// index over the read-only code store, and the service's current watermark.
type memoryEnv struct {
	memories *memory.FileStore
	index    memory.Index
	commit   string
	close    func()
}

func openMemoryEnv(storePath, root, service string) (*memoryEnv, error) {
	mounted, err := store.MountReadOnly(storePath, store.LevelL0)
	if err != nil {
		return nil, fmt.Errorf("mount store %s: %w", storePath, err)
	}

	commit := ""
	if service != "" {
		if meta, found, merr := mounted.GetServiceMeta(context.Background(), service); merr == nil && found {
			commit = meta.IndexedCommit
		}
	}
	return &memoryEnv{
		memories: memory.NewFileStore(memory.Path(root)),
		index:    memory.NewStoreIndex(mounted),
		commit:   commit,
		close:    func() { _ = mounted.Close() },
	}, nil
}

// ─── memory add ─────────────────────────────────────────────────────────────

func cmdMemoryAdd(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("memory add", stderr)
	storePath := fs.String("store", "", "local store path (required)")
	service := fs.String("service", "", "service ID (required)")
	root := fs.String("root", ".", "project root — memories live in <root>/.fathom/memories.jsonl")
	claim := fs.String("claim", "", "the claim: one sentence, present tense, falsifiable (required)")
	var anchors stringSliceFlag
	fs.Var(&anchors, "anchor", "anchor as KIND:REF, repeatable (at least one is required)")
	var evidence stringSliceFlag
	fs.Var(&evidence, "evidence", "evidence as KIND:REF, repeatable (at least one is required)")
	tier := fs.String("tier", "", "the tier you CLAIM (recorded for audit; the stored tier is derived from the evidence)")
	confidence := fs.Int("confidence", 0, "the confidence you CLAIM, 0-5 (capped by the derived tier)")
	by := fs.String("by", "human", "author as KIND[:NAME] — human | model | machine")
	commit := fs.String("commit", "", "corpus watermark to record (default: the service's indexed commit)")
	jsonOut := fs.Bool("json", false, "emit JSON instead of a human-readable summary")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *storePath == "" || *service == "" {
		fmt.Fprintln(stderr, "error: --store and --service are required")
		fs.Usage()
		return 1
	}

	draft := memory.Draft{Service: *service, Claim: *claim, ClaimedConfidence: *confidence}
	if *tier != "" {
		claimed := core.EvidenceTier(*tier)
		if !claimed.Valid() {
			fmt.Fprintf(stderr, "error: unknown tier %q (want %s)\n", *tier, strings.Join(memory.TierOrder(), " | "))
			return 1
		}
		draft.ClaimedTier = claimed
	}
	for _, raw := range anchors {
		a, err := memory.ParseAnchor(raw)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		draft.Anchors = append(draft.Anchors, a)
	}
	for _, raw := range evidence {
		e, err := memory.ParseEvidence(raw)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
		draft.Evidence = append(draft.Evidence, e)
	}
	author, err := memory.ParseActor(*by)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	draft.Author = author

	env, err := openMemoryEnv(*storePath, *root, *service)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	defer env.close()
	draft.Commit = *commit
	if draft.Commit == "" {
		draft.Commit = env.commit
	}

	m, err := memory.WriteMemory(context.Background(), env.memories, env.index, time.Now, draft)
	if err != nil {
		// A refusal is the feature, so it is reported as a refusal — named,
		// with its remedy — rather than as an anonymous failure.
		if re, ok := memory.Refused(err); ok {
			fmt.Fprintf(stderr, "refused (%s): %s\n", re.Reason, re.Detail)
			return 1
		}
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	if *jsonOut {
		jsonPrint(stdout, m)
		return 0
	}
	fmt.Fprintf(stdout, "remembered %s\n", m.ID)
	fmt.Fprintf(stdout, "  claim:    %s\n", m.Claim)
	fmt.Fprintf(stdout, "  anchors:  %s\n", joinAnchors(m.Anchors))
	fmt.Fprintf(stdout, "  evidence: %s\n", joinEvidence(m.Evidence))
	fmt.Fprintf(stdout, "  tier:     %s (confidence %d, derived from the evidence)\n", m.Tier, m.Confidence)
	if m.Capped {
		fmt.Fprintf(stdout, "  capped:   %s\n", m.CapReason)
	}
	for _, w := range m.Warnings {
		fmt.Fprintf(stdout, "  warning:  %s\n", w)
	}
	return 0
}

// ─── memory list ────────────────────────────────────────────────────────────

// memoryListReport is the shape both the table and --json render.
type memoryListReport struct {
	Service string           `json:"service"`
	Filter  string           `json:"state_filter,omitempty"`
	Total   int              `json:"total"`
	Entries []memoryListItem `json:"entries"`
	Note    string           `json:"note,omitempty"`
}

type memoryListItem struct {
	ID              string   `json:"id"`
	Claim           string   `json:"claim"`
	State           string   `json:"state"`
	Tier            string   `json:"tier"`
	BaseTier        string   `json:"base_tier"`
	Confidence      int      `json:"confidence"`
	Surfaces        bool     `json:"surfaces"`
	Anchors         []string `json:"anchors"`
	Line            string   `json:"line"`
	Drift           []string `json:"drift,omitempty"`
	LastConfirmedAt string   `json:"last_confirmed_at,omitempty"`
}

func cmdMemoryList(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("memory list", stderr)
	storePath := fs.String("store", "", "local store path (required)")
	service := fs.String("service", "", "service ID (required)")
	root := fs.String("root", ".", "project root — memories live in <root>/.fathom/memories.jsonl")
	state := fs.String("state", "", "only memories in this state: fresh | suspect | orphaned | retired")
	jsonOut := fs.Bool("json", false, "emit JSON instead of a human-readable table")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *storePath == "" || *service == "" {
		fmt.Fprintln(stderr, "error: --store and --service are required")
		fs.Usage()
		return 1
	}

	env, err := openMemoryEnv(*storePath, *root, *service)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	defer env.close()

	ctx := context.Background()
	all, err := env.memories.List(ctx, *service)
	if err != nil {
		fmt.Fprintf(stderr, "error: read memories: %v\n", err)
		return 1
	}
	evs, err := memory.EvaluateAll(ctx, env.index, memory.NewDriftCache(), time.Now, all)
	if err != nil {
		fmt.Fprintf(stderr, "error: evaluate memories: %v\n", err)
		return 1
	}

	report := memoryListReport{Service: *service, Filter: *state, Total: len(evs)}
	for _, e := range evs {
		if *state != "" && string(e.State) != *state {
			continue
		}
		report.Entries = append(report.Entries, listItem(e))
	}
	if len(report.Entries) == 0 {
		report.Note = "no memories" + stateSuffix(*state)
	}

	if *jsonOut {
		jsonPrint(stdout, report)
		return 0
	}
	fmt.Fprintf(stdout, "memories for %s (%d total)\n", *service, report.Total)
	if report.Note != "" {
		fmt.Fprintf(stdout, "  %s\n", report.Note)
		return 0
	}
	for _, e := range report.Entries {
		fmt.Fprintf(stdout, "  %s  %s\n", e.ID, e.Line)
		fmt.Fprintf(stdout, "    anchors: %s\n", strings.Join(e.Anchors, ", "))
	}
	return 0
}

func stateSuffix(state string) string {
	if state == "" {
		return ""
	}
	return " in state " + state
}

func listItem(e memory.Evaluation) memoryListItem {
	item := memoryListItem{
		ID: e.ID, Claim: e.Claim, State: string(e.State),
		Tier: string(e.Tier), BaseTier: string(e.BaseTier),
		Confidence: e.Confidence, Surfaces: e.Surfaces, Line: e.Line(),
	}
	if !e.LastConfirmedAt.IsZero() {
		item.LastConfirmedAt = e.LastConfirmedAt.UTC().Format(time.RFC3339)
	}
	for _, d := range e.Drift {
		item.Anchors = append(item.Anchors, d.Anchor.Key())
		if d.Detail != "" {
			item.Drift = append(item.Drift, d.Detail)
		}
	}
	return item
}

// ─── memory review ──────────────────────────────────────────────────────────

// memoryReviewReport is the shape both the queue listing and a resolution
// render.
type memoryReviewReport struct {
	Service  string           `json:"service"`
	Queue    []memoryListItem `json:"queue,omitempty"`
	Resolved *memoryResolved  `json:"resolved,omitempty"`
	Note     string           `json:"note,omitempty"`
}

type memoryResolved struct {
	ID              string `json:"id"`
	Resolution      string `json:"resolution"`
	By              string `json:"by"`
	Commit          string `json:"commit,omitempty"`
	State           string `json:"state"`
	Tier            string `json:"tier"`
	Confidence      int    `json:"confidence"`
	Capped          bool   `json:"capped"`
	CapReason       string `json:"cap_reason,omitempty"`
	LastConfirmedAt string `json:"last_confirmed_at"`
	Claim           string `json:"claim"`
}

// cmdMemoryReview lists the suspect/orphaned queue, and with --id/--resolve
// applies one of the three resolutions from §3.2.
//
// The tier consequence of WHO resolved is not a flag: internal/memory.Resolve
// derives it from the actor, so a model confirming a memory lands it strictly
// below the same memory confirmed by a human without the caller (or this
// file) getting a say.
func cmdMemoryReview(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("memory review", stderr)
	storePath := fs.String("store", "", "local store path (required)")
	service := fs.String("service", "", "service ID (required)")
	root := fs.String("root", ".", "project root — memories live in <root>/.fathom/memories.jsonl")
	id := fs.String("id", "", "memory to resolve (omit to list the queue)")
	resolve := fs.String("resolve", "", "still-true | no-longer-true | rewrite")
	by := fs.String("by", "", "who is resolving: human | model | machine (optionally kind:name) — required with --resolve")
	claim := fs.String("claim", "", "new claim text (required for --resolve rewrite)")
	note := fs.String("note", "", "free-text note recorded with the resolution")
	commit := fs.String("commit", "", "corpus watermark to record (default: the service's indexed commit)")
	jsonOut := fs.Bool("json", false, "emit JSON instead of a human-readable summary")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *storePath == "" || *service == "" {
		fmt.Fprintln(stderr, "error: --store and --service are required")
		fs.Usage()
		return 1
	}
	if (*id == "") != (*resolve == "") {
		fmt.Fprintln(stderr, "error: --id and --resolve go together (omit both to list the queue)")
		fs.Usage()
		return 1
	}

	env, err := openMemoryEnv(*storePath, *root, *service)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	defer env.close()
	ctx := context.Background()

	if *id == "" {
		items, qerr := memory.ReviewQueue(ctx, env.memories, env.index, memory.NewDriftCache(), time.Now, *service)
		if qerr != nil {
			fmt.Fprintf(stderr, "error: build review queue: %v\n", qerr)
			return 1
		}
		report := memoryReviewReport{Service: *service}
		for _, it := range items {
			report.Queue = append(report.Queue, listItem(it.Evaluation))
		}
		if len(report.Queue) == 0 {
			report.Note = "the review queue is empty — every memory's anchors still match what they were when it was confirmed"
		}
		if *jsonOut {
			jsonPrint(stdout, report)
			return 0
		}
		fmt.Fprintf(stdout, "review queue for %s (%d)\n", *service, len(report.Queue))
		if report.Note != "" {
			fmt.Fprintf(stdout, "  %s\n", report.Note)
			return 0
		}
		// Evaluation.Line() already carries the drift detail; repeating it
		// per anchor here just made the queue unreadable.
		for _, it := range items {
			fmt.Fprintf(stdout, "  %s  %s\n", it.Memory.ID, it.Evaluation.Line())
			fmt.Fprintf(stdout, "    %s\n", it.Reason)
		}
		fmt.Fprintf(stdout, "  resolve with: fathom memory review --store … --service %s --id <ID> --resolve still-true|no-longer-true|rewrite --by human\n", *service)
		return 0
	}

	if *by == "" {
		fmt.Fprintln(stderr, "error: --by is required with --resolve: a re-confirmation records WHO confirmed, and that decides the tier")
		return 1
	}
	actor, aerr := memory.ParseActor(*by)
	if aerr != nil {
		fmt.Fprintf(stderr, "error: %v\n", aerr)
		return 1
	}
	watermark := *commit
	if watermark == "" {
		watermark = env.commit
	}

	m, rerr := memory.Resolve(ctx, env.memories, env.index, time.Now, *id,
		memory.Resolution(*resolve), actor, watermark, *claim, *note)
	if rerr != nil {
		if re, ok := memory.Refused(rerr); ok {
			fmt.Fprintf(stderr, "refused (%s): %s\n", re.Reason, re.Detail)
			return 1
		}
		fmt.Fprintf(stderr, "error: %v\n", rerr)
		return 1
	}

	ev, eerr := memory.EvaluateState(ctx, env.index, memory.NewDriftCache(), time.Now, m)
	if eerr != nil {
		fmt.Fprintf(stderr, "error: evaluate resolved memory: %v\n", eerr)
		return 1
	}

	report := memoryReviewReport{Service: *service, Resolved: &memoryResolved{
		ID: m.ID, Resolution: *resolve, By: actor.String(), Commit: watermark,
		State: string(ev.State), Tier: string(ev.Tier), Confidence: ev.Confidence,
		Capped: m.Capped, CapReason: m.CapReason,
		LastConfirmedAt: m.LastConfirmedAt.UTC().Format(time.RFC3339),
		Claim:           m.Claim,
	}}
	if *jsonOut {
		jsonPrint(stdout, report)
		return 0
	}
	fmt.Fprintf(stdout, "%s resolved %s as %s\n", actor.String(), m.ID, *resolve)
	fmt.Fprintf(stdout, "  %s\n", ev.Line())
	fmt.Fprintf(stdout, "  confirmed %s against %s\n", report.Resolved.LastConfirmedAt, commitLabel(watermark))
	if m.Capped {
		fmt.Fprintf(stdout, "  capped:   %s\n", m.CapReason)
	}
	return 0
}

func commitLabel(commit string) string {
	if commit == "" {
		return "an unnamed commit (the service records no indexed commit)"
	}
	return memory.ShortWatermark(commit)
}

// ─── memory show ────────────────────────────────────────────────────────────

func cmdMemoryShow(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("memory show", stderr)
	root := fs.String("root", ".", "project root — memories live in <root>/.fathom/memories.jsonl")
	id := fs.String("id", "", "memory id (required)")
	jsonOut := fs.Bool("json", false, "emit JSON instead of a human-readable summary")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *id == "" {
		fmt.Fprintln(stderr, "error: --id is required")
		fs.Usage()
		return 1
	}

	// show reads the memory file alone: a RETIRED memory must stay
	// retrievable by id even when its anchors are long gone, so this verb
	// deliberately does not need the index.
	ms := memory.NewFileStore(memory.Path(*root))
	m, found, err := ms.Get(context.Background(), *id)
	if err != nil {
		fmt.Fprintf(stderr, "error: read memories: %v\n", err)
		return 1
	}
	if !found {
		fmt.Fprintf(stderr, "error: no memory with id %q in %s\n", *id, memory.Path(*root))
		return 1
	}

	if *jsonOut {
		jsonPrint(stdout, m)
		return 0
	}
	fmt.Fprintf(stdout, "%s\n", m.ID)
	fmt.Fprintf(stdout, "  service:   %s\n", m.Service)
	fmt.Fprintf(stdout, "  claim:     %s\n", m.Claim)
	fmt.Fprintf(stdout, "  anchors:   %s\n", joinAnchors(m.Anchors))
	fmt.Fprintf(stdout, "  evidence:  %s\n", joinEvidence(m.Evidence))
	fmt.Fprintf(stdout, "  tier:      %s (confidence %d)\n", m.Tier, m.Confidence)
	if m.Capped {
		fmt.Fprintf(stdout, "  capped:    %s\n", m.CapReason)
	}
	fmt.Fprintf(stdout, "  author:    %s\n", m.Author)
	fmt.Fprintf(stdout, "  confirmed: %s by %s against %s\n",
		m.LastConfirmedAt.UTC().Format(time.RFC3339), m.ConfirmedBy, commitLabel(m.ConfirmedAtCommit))
	if m.Retired() {
		fmt.Fprintf(stdout, "  retired:   %s — %s (never surfaces again; still retrievable by id)\n",
			m.RetiredAt.UTC().Format(time.RFC3339), m.RetiredReason)
	}
	for _, h := range m.History {
		fmt.Fprintf(stdout, "  history:   %s %s by %s %s\n",
			h.At.UTC().Format(time.RFC3339), h.Kind, h.By, h.Note)
	}
	return 0
}

func joinAnchors(as []memory.Anchor) string {
	out := make([]string, 0, len(as))
	for _, a := range as {
		out = append(out, a.Key())
	}
	return strings.Join(out, ", ")
}

func joinEvidence(es []memory.Evidence) string {
	out := make([]string, 0, len(es))
	for _, e := range es {
		out = append(out, e.Key())
	}
	return strings.Join(out, ", ")
}

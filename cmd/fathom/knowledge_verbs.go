package main

// knowledge_verbs.go — the Knowledge verbs the console's Knowledge pages
// render (vision 35 §3, design §1 routes, P10): `fathom layers strata`,
// `fathom layers coverage`, `fathom layers show <name> --json`,
// `fathom claim show <id>` and `fathom report packs`. Each prints one
// internal/console projection; the pages at /layers, /layers/{layer},
// /claims/{id} and /packs serve the same projection as their JSON section,
// and TestConsolePagesRenderTheirVerbs holds them equal.
//
// They read the store the daemon reads — never a store relative to the
// working directory. The store is resolved once (resolveKnowledgeStore):
// --store when given, else the store the daemon at --url recorded when its
// listener bound (~/.fathom/daemons/<host>-<port>.json, OC-2's record),
// else [daemon].store, else the workspace default. It is mounted genuinely
// read-only (store.MountReadOnly): a verb that explains the index never
// writes to it.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// knowledgeStore names the store a Knowledge verb reads and the rule that
// chose it.
type knowledgeStore struct {
	Path string
	Rule string // flag | daemon | config | default
	Note string
}

func (k knowledgeStore) String() string {
	return k.Path + " (rule " + k.Rule + ": " + k.Note + ")"
}

// resolveKnowledgeStore picks the store: --store, the daemon's recorded
// store, [daemon].store, then the workspace default.
func resolveKnowledgeStore(flag, baseURL string) knowledgeStore {
	if flag != "" {
		return knowledgeStore{Path: flag, Rule: "flag", Note: "--store"}
	}
	base := baseURL
	if base == "" {
		base = defaultFathomURL()
	}
	if rec, ok := readDaemonRecordFor(base); ok && rec.Store != "" {
		return knowledgeStore{Path: rec.Store, Rule: "daemon", Note: "the store " + rec.Note()}
	}
	if loaded, err := config.Load(config.DefaultUserPath(), config.DefaultProjectPath()); err == nil && loaded.Config.Daemon.Store != "" {
		return knowledgeStore{Path: loaded.Config.Daemon.Store, Rule: "config", Note: "[daemon].store in config.toml"}
	}
	return knowledgeStore{Path: defaultStorePath(), Rule: "default", Note: "no daemon record for " + base + " and no [daemon].store; the workspace default"}
}

// openKnowledgeStore mounts ks read-only. A missing store is refused with
// the remedy: store.Open would create an empty file there and every figure
// would read as a measured zero.
func openKnowledgeStore(ks knowledgeStore) (store.Store, error) {
	if _, err := os.Stat(ks.Path); err != nil {
		return nil, fmt.Errorf("no store at %s — %v; pass --store, run against the daemon that records its store (--url), or set [daemon].store; nothing was read", ks, err)
	}
	st, err := store.MountReadOnly(ks.Path, store.LevelWriter)
	if err != nil {
		return nil, fmt.Errorf("open store %s: %w", ks, err)
	}
	return st, nil
}

// knowledgeFlags are the store flags every Knowledge verb takes.
type knowledgeFlags struct {
	store, url *string
	json       *bool
}

func addKnowledgeFlags(fs interface {
	String(name, value, usage string) *string
	Bool(name string, value bool, usage string) *bool
}) knowledgeFlags {
	return knowledgeFlags{
		store: fs.String("store", "", "store path, .sqlite/.db (default: the store the daemon at --url recorded, else [daemon].store, else the workspace default)"),
		url:   fs.String("url", defaultFathomURL(), "fathom serve-http base URL whose recorded store to read"),
		json:  fs.Bool("json", false, "emit the console projection as JSON (the page's JSON section)"),
	}
}

// withKnowledgeSource resolves and mounts the store, runs fn, and closes it.
func withKnowledgeSource(kf knowledgeFlags, stderr io.Writer, fn func(console.KnowledgeSource, knowledgeStore) error) int {
	ks := resolveKnowledgeStore(*kf.store, *kf.url)
	st, err := openKnowledgeStore(ks)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	defer st.Close()
	if err := fn(console.KnowledgeSource{Store: st}, ks); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	return 0
}

// writeKnowledgeJSON writes v indented, the shape every page section carries.
func writeKnowledgeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// valueWords renders a projected count for a terminal: the number, "≥ n"
// for a lower bound, or the named absence — never a bare 0 for something
// unmeasured.
func valueWords(v console.Value[int]) string { return valuePlain(v) }

// dollarsWords renders a projected dollar figure the same way.
func dollarsWords(v console.Value[float64]) string {
	switch v.State {
	case console.StateAbsent:
		return "not measured (" + v.Reason + ")"
	case console.StateError:
		return "unreadable (" + v.Reason + ")"
	case console.StatePartial:
		return fmt.Sprintf("≥ $%.2f (%s)", v.V, v.Reason)
	}
	return fmt.Sprintf("$%.2f", v.V)
}

// cmdLayersStrata implements `fathom layers strata`.
func cmdLayersStrata(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("layers strata", stderr)
	kf := addKnowledgeFlags(fs)
	if err := fs.Parse(args); err != nil {
		return 1
	}
	return withKnowledgeSource(kf, stderr, func(src console.KnowledgeSource, ks knowledgeStore) error {
		s, err := console.Strata(context.Background(), src)
		if err != nil {
			return err
		}
		if *kf.json {
			return writeKnowledgeJSON(stdout, s)
		}
		fmt.Fprintf(stdout, "store: %s\n", ks)
		for _, b := range s.Bands {
			fmt.Fprintf(stdout, "\n%s — %s\n", strings.ToUpper(b.Stratum), b.Summary)
			if b.Reason != "" {
				fmt.Fprintf(stdout, "  %s\n", b.Reason)
			}
			for _, d := range b.Dialects {
				line := fmt.Sprintf("  %-7s %s", d.Level, strings.Join(d.Languages, ", "))
				if len(d.Producers) > 0 {
					line += " (delegates: " + strings.Join(d.Producers, ", ") + ")"
				}
				fmt.Fprintln(stdout, line)
			}
			for _, l := range b.Layers {
				fmt.Fprintf(stdout, "  %-24s %-8s claims %s · runs %s · spent %s\n", l.Name, l.Source, valueWords(l.Claims), valueWords(l.Runs), dollarsWords(l.CostUSD))
			}
		}
		t := s.Totals
		fmt.Fprintf(stdout, "\ntotals: %s current claims · %s real runs · spent %s · %s emitted · %s rejected · %s refused before starting\n",
			valueWords(t.Claims), valueWords(t.Runs), dollarsWords(t.CostUSD), valueWords(t.Emitted), valueWords(t.Rejected), valueWords(t.Refused))
		var evs []string
		for _, v := range []string{"re-anchored", "contradicted", "corroborated", "refuted", "superseded", "annotated"} {
			evs = append(evs, v+" "+valueWords(s.Lifecycle.Events[v]))
		}
		fmt.Fprintf(stdout, "lifecycle: %s\n", strings.Join(evs, " · "))
		for _, n := range s.Notes {
			fmt.Fprintf(stdout, "note: %s\n", n)
		}
		return nil
	})
}

// cmdLayersCoverage implements `fathom layers coverage`.
func cmdLayersCoverage(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("layers coverage", stderr)
	kf := addKnowledgeFlags(fs)
	if err := fs.Parse(args); err != nil {
		return 1
	}
	return withKnowledgeSource(kf, stderr, func(src console.KnowledgeSource, ks knowledgeStore) error {
		m, err := console.LayerCoverage(context.Background(), src)
		if err != nil {
			return err
		}
		if *kf.json {
			return writeKnowledgeJSON(stdout, m)
		}
		fmt.Fprintf(stdout, "store: %s\n\n", ks)
		header := fmt.Sprintf("%-24s", "PROJECT")
		for _, l := range m.Layers {
			header += fmt.Sprintf(" %14s", strings.ToUpper(l))
		}
		fmt.Fprintln(stdout, header)
		for _, r := range m.Rows {
			line := fmt.Sprintf("%-24s", r.Service)
			for _, c := range r.Cells {
				cell := valueCellWord(c.Claims)
				line += fmt.Sprintf(" %14s", cell)
			}
			fmt.Fprintln(stdout, line)
		}
		fmt.Fprintf(stdout, "\ncovered: %s projects with a current claim (%s)\n", valueWords(m.Covered), m.Covered.Denominator)
		for _, a := range m.NeverCompiled {
			where := ""
			if len(a.InstalledIn) > 0 {
				where = " — installed in " + strings.Join(a.InstalledIn, ", ")
			}
			fmt.Fprintf(stdout, "never compiled: %s (%s): %s%s\n", a.Layer, a.Source, a.Reason, where)
		}
		return nil
	})
}

// valueCellWord is a coverage cell for a terminal table: the count, or a
// word for a cell without one.
func valueCellWord(v console.Value[int]) string {
	switch v.State {
	case console.StateAbsent:
		if strings.HasPrefix(v.Reason, "not installed") {
			return "not installed"
		}
		return "not compiled"
	case console.StateError:
		return "unreadable"
	case console.StatePartial:
		return "≥ " + fmtThousands(v.V)
	}
	return fmtThousands(v.V)
}

// cmdLayersShowJSON is `fathom layers show <name> --json`: the layer page's
// projection (definition, compile runs, rejections, uncompiled scopes).
func cmdLayersShowJSON(name string, kf knowledgeFlags, stdout, stderr io.Writer) int {
	return withKnowledgeSource(kf, stderr, func(src console.KnowledgeSource, _ knowledgeStore) error {
		d, err := console.LayerDetail(context.Background(), src, name)
		if err != nil {
			return err
		}
		if !d.Known {
			return fmt.Errorf("no layer %q: no pack defines it and the store holds no claim or compile run for it (fathom layers strata lists them)", name)
		}
		return writeKnowledgeJSON(stdout, d)
	})
}

// cmdClaimShow implements `fathom claim show <id>`.
func cmdClaimShow(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("claim show", stderr)
	kf := addKnowledgeFlags(fs)
	service := fs.String("service", "", "service ID (needed only when two services hold the same claim id)")
	if err := fs.Parse(reorderFlagsFirst(fs, args)); err != nil {
		return 1
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(stderr, "error: claim show requires exactly one claim id")
		printClaimUsage(stderr)
		return 1
	}
	id := fs.Arg(0)
	return withKnowledgeSource(kf, stderr, func(src console.KnowledgeSource, ks knowledgeStore) error {
		d, err := console.ClaimDetail(context.Background(), src, id, *service)
		if err != nil {
			return err
		}
		if !d.Found && len(d.Matches) == 0 {
			return fmt.Errorf("no claim %s in store %s", id, ks)
		}
		if *kf.json {
			return writeKnowledgeJSON(stdout, d)
		}
		if len(d.Matches) > 0 {
			fmt.Fprintf(stdout, "claim %s is held by %d services — name one with --service:\n", id, len(d.Matches))
			for _, m := range d.Matches {
				fmt.Fprintf(stdout, "  %s (%s, %s)\n", m.Service, m.Layer, m.Status)
			}
			return nil
		}
		fmt.Fprintf(stdout, "claim:     %s\nservice:   %s\nlayer:     %s\nstatus:    %s (served: %v)\ntier:      %s\nstatement: %s\nscope:     %s\n",
			d.ClaimID, d.Service, d.Layer, d.Status, d.Served, d.Tier, d.Statement, strings.Join(d.Scope, ", "))
		for _, c := range d.Citations {
			fmt.Fprintf(stdout, "cites:     %s\n", c.Ref)
		}
		switch ex := d.Excerpt; ex.State {
		case console.StatePresent:
			fmt.Fprintf(stdout, "\n%s (%s)\n", ex.V.Path, ex.V.From)
			if ex.V.Note != "" {
				fmt.Fprintf(stdout, "  %s\n", ex.V.Note)
			}
			for _, l := range ex.V.Lines {
				mark := " "
				if l.Cited {
					mark = ">"
				}
				fmt.Fprintf(stdout, "%s %5d  %s\n", mark, l.Number, l.Text)
			}
		default:
			fmt.Fprintf(stdout, "\nexcerpt: %s — %s\n", ex.State, ex.Reason)
		}
		fmt.Fprintf(stdout, "\nevents: %s\n", valueWords(d.EventCount))
		for _, e := range d.Events {
			fmt.Fprintf(stdout, "  %s  %-12s %s %s\n", e.At, e.Verdict, e.Author, e.Note)
		}
		return nil
	})
}

// cmdReportPacks implements `fathom report packs`.
func cmdReportPacks(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("report packs", stderr)
	kf := addKnowledgeFlags(fs)
	if err := fs.Parse(args); err != nil {
		return 1
	}
	return withKnowledgeSource(kf, stderr, func(src console.KnowledgeSource, ks knowledgeStore) error {
		m, err := console.PackMatrix(context.Background(), src)
		if err != nil {
			return err
		}
		if *kf.json {
			return writeKnowledgeJSON(stdout, m)
		}
		fmt.Fprintf(stdout, "store: %s\n\nDIALECT PACKS\n", ks)
		fmt.Fprintf(stdout, "%-18s %-28s %-8s %8s %8s %-10s %-24s %s\n", "LANGUAGE", "ENGINE", "EDGES", "FILES", "PROJECTS", "INTEGRITY", "INVENTORY", "PRECISION/RECALL")
		for _, d := range m.Dialects {
			pr := "unmeasured"
			if d.Precision.Known() {
				pr = fmt.Sprintf("%.2f / %.2f (%s tier)", d.Precision.V, d.Recall.V, strings.SplitN(d.Reading, " ", 2)[0])
			}
			inv := strings.Join(d.Inventory, ",")
			if inv == "" {
				inv = "—"
			}
			fmt.Fprintf(stdout, "%-18s %-28s %-8s %8s %8s %-10s %-24s %s\n", d.Name, d.Engine, d.TypedEdges, valueWords(d.Files), valueWords(d.Projects), d.Integrity, inv, pr)
		}
		fmt.Fprintln(stdout, "\nLAYER PACKS")
		for _, l := range m.Layers {
			active := "none"
			if len(l.ActiveIn) > 0 {
				active = strings.Join(l.ActiveIn, ", ")
			}
			fmt.Fprintf(stdout, "%-24s %-8s %-8s claims %s · active in %s\n", l.Name, l.Source, l.Stratum, valueWords(l.Contributes), active)
		}
		fmt.Fprintln(stdout, "\nreadings:")
		for _, r := range m.Readings {
			fmt.Fprintf(stdout, "  %s %s tier: precision %.2f, recall %.2f — %s (%s, %s)\n", console.LanguageName(r.Language), r.Tier, r.Precision, r.Recall, r.Measure, r.Source, r.Date)
		}
		return nil
	})
}

// knowledgeServiceKnown reports whether the store indexes or holds claims
// for svc, and the known services for the refusal.
func knowledgeServiceKnown(ctx context.Context, st store.Store, svc string) (bool, []string, error) {
	ids, err := st.ListServiceIDs(ctx)
	if err != nil {
		return false, nil, err
	}
	claimIDs, err := st.ClaimServiceIDs(ctx)
	if err != nil {
		return false, nil, err
	}
	seen := map[string]bool{}
	var all []string
	for _, id := range append(ids, claimIDs...) {
		if !seen[id] {
			seen[id] = true
			all = append(all, id)
		}
	}
	sort.Strings(all)
	return seen[svc], all, nil
}

// knowledgeClient wraps a mounted store for the serving explainer's
// enrichments (the raw fallback and coverage read the corpus from disk).
func knowledgeClient(st store.Store) *client.Client {
	return client.New(connector.NewLocalConnector(), st)
}

package main

// brief.go — OB-7: `fathom brief` renders the observatory's impact model to
// a dated LOCAL markdown value brief. Sharing is a human verb (law 13):
// this verb writes a file and stops — a person decides where it goes. The
// pitch is the discipline: real numbers with denominators, version stamps,
// and an explicit not-measured section.
//
// BR-1 (owner ask, 2026-08-08): the brief is read by someone who has never
// heard of fathom. Every figure therefore arrives with (a) a plain-English
// statement of what the instrument counts, (b) its denominator, and (c) a
// reading — whether it is evidence fathom helped, evidence it did not, or
// evidence of nothing yet. Large numbers carry thousands separators. The
// closing section is not a disclaimer but a work order: what is switched
// off, what turning it on would let the brief say, and the exact command.

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

func cmdBrief(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("brief", stderr)
	url := fs.String("url", defaultFathomURL(), "fathom serve-http base URL")
	outDir := fs.String("out", "", "output directory (default ~/.fathom/briefs)")
	printOnly := fs.Bool("print", false, "print the brief to stdout instead of writing a file")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	now := time.Now()
	m := buildObservatoryModel(*url, now)
	md := renderBriefMD(m)

	if *printOnly {
		fmt.Fprint(stdout, md)
		return 0
	}

	dir := *outDir
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(stderr, "error: cannot resolve home dir for the default output location: %v (use --out)\n", err)
			return 1
		}
		dir = filepath.Join(home, ".fathom", "briefs")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	path := filepath.Join(dir, "brief-"+now.UTC().Format("2006-01-02")+".md")
	if err := os.WriteFile(path, []byte(md), 0o644); err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "brief written: %s\nsharing is a human verb — nothing was sent anywhere.\n", path)
	return 0
}

// sentence capitalizes an instrument note so it can be appended after a
// full stop without reading like a fragment.
func sentence(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	if r[0] >= 'a' && r[0] <= 'z' {
		r[0] = r[0] - 'a' + 'A'
	}
	return string(r)
}

// briefFleet is the fold the narrative needs: totals plus the grade
// distribution that drives the "configure for more" section.
type briefFleet struct {
	Services   int
	Symbols    int
	Calls      int
	AgentCalls int
	Wired      int
	GradeA     int
	BelowA     []obsServiceRow // graded below A, busiest first
	Freshness  []obsServiceRow // carrying a re-ingest remedy
	Wiring     []obsServiceRow // carrying a wiring remedy
}

func foldBriefFleet(m observatoryModel) briefFleet {
	f := briefFleet{}
	for _, s := range m.Services {
		f.Services++
		f.Symbols += s.Symbols
		f.Calls += s.CallsTotal
		f.AgentCalls += s.CallsAgent
		if s.WiringRemedy == "" {
			f.Wired++
		} else {
			f.Wiring = append(f.Wiring, s)
		}
		if s.FreshnessRemedy != "" {
			f.Freshness = append(f.Freshness, s)
		}
		if s.Grade == "A" {
			f.GradeA++
		} else if s.Grade != "" {
			f.BelowA = append(f.BelowA, s)
		}
	}
	sort.SliceStable(f.BelowA, func(i, j int) bool {
		return f.BelowA[i].CallsTotal > f.BelowA[j].CallsTotal
	})
	return f
}

// briefAction is one entry in the closing work order: what is switched off,
// what switching it on would let a future brief say, and the command.
type briefAction struct {
	What   string
	Unlock string
	Cmd    string // empty when there is nothing to run — see Note
	Note   string // what happens instead, when Cmd is empty
}

// briefTuning derives the "it could help more if configured differently"
// section from live state only. Every entry is a real remedy the model
// already carries — nothing here is advice invented for the page.
func briefTuning(m observatoryModel, f briefFleet) []briefAction {
	var acts []briefAction

	if n := len(f.BelowA); n > 0 {
		names := make([]string, 0, n)
		for _, s := range f.BelowA {
			names = append(names, s.Service+" ("+s.Grade+")")
		}
		shown := names
		if len(shown) > 6 {
			shown = append(append([]string{}, names[:6]...), "…")
		}
		acts = append(acts, briefAction{
			What: fmt.Sprintf("%d of %d repositories are graded below A — each is indexed and answers queries, but each is missing at least one piece of the wiring that lets fathom act *before* an edit rather than only answer when asked: %s",
				n, f.Services, strings.Join(shown, ", ")),
			Unlock: "obligations and guards start firing in those repos, so their sessions contribute to the prevention numbers above instead of sitting outside the denominator",
			Cmd:    "fathom fleet reconcile --apply",
		})
	}
	if n := len(f.Freshness); n > 0 {
		names := make([]string, 0, n)
		for _, s := range f.Freshness {
			names = append(names, s.Service)
		}
		acts = append(acts, briefAction{
			What:   fmt.Sprintf("%d repositor%s answering from an index older than the repo's current commit: %s", n, map[bool]string{true: "y is", false: "ies are"}[n == 1], strings.Join(names, ", ")),
			Unlock: "answers for those repos stop lagging the working tree — stale recall is the one failure that costs an agent more than no recall",
			Cmd:    f.Freshness[0].FreshnessRemedy,
		})
	}

	if m.Impact.TokensOffSessions == 0 {
		acts = append(acts, briefAction{
			What:   "no control group has ever been recorded, so the token question above has no answer at all — not a small answer, no answer",
			Unlock: "the brief could state a measured token range for sessions with fathom on versus off; this is the only instrument that can license a savings claim (law 12)",
			Cmd:    "fathom arm off --sessions 20",
		})
	}
	if m.Impact.LedgerSessions > 0 && m.Impact.VerifyCatchesTotal != nil {
		if ran := verifyRanSessions(m.Impact.VerifyCatchesNote); ran >= 0 && ran*4 < m.Impact.LedgerSessions {
			acts = append(acts, briefAction{
				What:   fmt.Sprintf("the plan gate ran in only %s of %s sessions, so \"claims caught before they were believed\" is measured over a sliver of the work", fmtThousands(ran), fmtThousands(m.Impact.LedgerSessions)),
				Unlock: "a catch count over most sessions instead of a handful — the clearest single number for what fathom stopped",
				Cmd:    "fathom verify --service <svc>",
				Note:   "wire that command into the pre-push hook (lefthook.yml) so it runs on every push rather than by hand",
			})
		}
	}
	if !m.Narrate.Active {
		acts = append(acts, briefAction{
			What:   "the narration dial is off, so no session records *what happened* alongside the counts",
			Unlock: "concrete before/after stories to sit beside the numbers — labeled as a lens, never quoted as proof (law 14)",
			Cmd:    "fathom narrate on --for 7d",
		})
	}
	if m.Impact.ExpSessions == 0 {
		acts = append(acts, briefAction{
			What:   "no session yet carries the exploration ledger, so the cost of answering a question by grepping versus by index is unmeasured here",
			Unlock: "a per-question cost comparison (observational, not causal) between raw file exploration and indexed lookup",
			Note:   "nothing to run — sessions started after this build fill it in on their own",
		})
	}
	if m.Impact.FirstEditMedian == nil {
		acts = append(acts, briefAction{
			What:   "no session yet records how many turns passed before the first edit, so warm-up cost is unknown",
			Unlock: "a compounding signal: whether sessions in a well-indexed repo start editing sooner than sessions in a cold one",
			Note:   "nothing to run — sessions started after this build fill it in on their own",
		})
	}
	return acts
}

// verifyRanSessions pulls the "ran in N of M" numerator out of the verify
// note so the tuning section can compare it to the session count. Returns
// -1 when the note is not in that shape — the caller then says nothing
// rather than guessing.
func verifyRanSessions(note string) int {
	_, rest, ok := strings.Cut(note, "ran in ")
	if !ok {
		return -1
	}
	num, _, ok := strings.Cut(rest, " ")
	if !ok {
		return -1
	}
	n, err := strconv.Atoi(num)
	if err != nil {
		return -1
	}
	return n
}

// renderBriefMD is the pure rendering core (OB-7's test surface).
func renderBriefMD(m observatoryModel) string {
	imp := m.Impact
	f := foldBriefFleet(m)
	var b strings.Builder

	fmt.Fprintf(&b, "# fathom value brief — %s\n\n", m.GeneratedAt)
	fmt.Fprintf(&b, "> Generated by fathom %s from live instruments. Every figure below names the instrument that produced it and the denominator it is out of. Where something was not measured, this brief says so instead of printing a zero. There are no estimates and no projections anywhere in it.\n\n", m.FathomVersion)

	// ---- The short version: the whole story in five sentences. ----
	b.WriteString("## The short version\n\n")
	b.WriteString("**What fathom is:** a shared index of your source code that AI coding agents query — \"where is this defined\", \"who calls it\", \"what rules govern this file\" — instead of reading and searching files one at a time. It also injects governing rules into the agent's context immediately before it edits a file.\n\n")

	if m.DegradedNote != "" {
		b.WriteString("**Scale:** " + m.DegradedNote + "\n\n")
	} else {
		fmt.Fprintf(&b, "**Scale:** %s repositories indexed, %s symbols, %s recorded tool calls (%s of them through the curated 16-tool agent surface).\n\n",
			fmtThousands(f.Services), fmtThousands(f.Symbols), fmtThousands(f.Calls), fmtThousands(f.AgentCalls))
	}

	fmt.Fprintf(&b, "**What it demonstrably did** over %s coding sessions", fmtThousands(imp.LedgerSessions))
	if imp.DateRange != "" {
		fmt.Fprintf(&b, " (%s)", imp.DateRange)
	}
	b.WriteString(": ")
	var did []string
	if imp.ObligationCalls > 0 {
		did = append(did, fmt.Sprintf("injected governing rules before an edit %s times", fmtThousands(imp.ObligationCalls)))
	}
	if imp.DiscoveryTotal != nil && *imp.DiscoveryTotal > 0 {
		did = append(did, fmt.Sprintf("answered %s discovery actions that would otherwise have been file-by-file searching", fmtThousands(*imp.DiscoveryTotal)))
	}
	if len(did) == 0 {
		b.WriteString("no prevention instrument reported a non-zero count — see the sections below for which ones are absent versus genuinely zero.")
	} else {
		b.WriteString(strings.Join(did, "; ") + ".")
	}
	b.WriteString("\n\n")

	b.WriteString("**What it has NOT shown:** that it saves tokens or money. ")
	if imp.TokensOffSessions == 0 {
		b.WriteString("Every session on record ran with fathom switched on, so there is nothing to compare against. That is a missing experiment, not a negative result — see *Did it save tokens?* below.\n\n")
	} else {
		fmt.Fprintf(&b, "A comparison exists (%s sessions on, %s off) and is reported below.\n\n", fmtThousands(imp.TokensOnSessions), fmtThousands(imp.TokensOffSessions))
	}

	acts := briefTuning(m, f)
	if len(acts) > 0 {
		if acts[0].Cmd != "" {
			fmt.Fprintf(&b, "**The one thing most worth changing:** %s → `%s`\n", acts[0].What, acts[0].Cmd)
		} else {
			fmt.Fprintf(&b, "**The one thing most worth changing:** %s\n", acts[0].What)
		}
	} else {
		b.WriteString("**Configuration:** every instrument this brief knows how to read is switched on.\n")
	}

	// ---- 1. Usage ----
	b.WriteString("\n## 1 — How much is fathom being used?\n\n")
	if m.DegradedNote != "" {
		b.WriteString("- " + m.DegradedNote + "\n")
	} else {
		fmt.Fprintf(&b, "- **Repositories indexed:** %s, holding %s symbols (functions, types, and the edges between them)\n", fmtThousands(f.Services), fmtThousands(f.Symbols))
		fmt.Fprintf(&b, "- **Wiring grade:** %s of %s repositories are grade A (indexed, hooks installed, rules present). A repository below A still answers queries — it just cannot act before an edit.\n", fmtThousands(f.GradeA), fmtThousands(f.Services))
		fmt.Fprintf(&b, "- **Tool calls recorded:** %s in the current call log; %s of those came through the curated agent surface\n", fmtThousands(f.Calls), fmtThousands(f.AgentCalls))
		if len(m.Services) > 0 {
			var top []string
			for i, s := range m.Services {
				if i == 5 || s.CallsTotal == 0 {
					break
				}
				top = append(top, fmt.Sprintf("%s %s", s.Service, fmtThousands(s.CallsTotal)))
			}
			if len(top) > 0 {
				b.WriteString("- **Busiest repositories** (calls): " + strings.Join(top, " · ") + "\n")
			}
		}
		if m.AgentSurfaceTools > 0 {
			fmt.Fprintf(&b, "- **Standing cost:** the agent surface is %s tools, %s bytes of tool schema that every connected session carries before doing any work. This is fathom's fixed overhead and it is counted here honestly.\n",
				fmtThousands(m.AgentSurfaceTools), fmtThousands(m.AgentSchemaBytes))
		}
		b.WriteString("\n**Reading:** usage is real and concentrated. Traffic is not evidence of value on its own — the sections below are.\n")
	}

	// ---- 2. Prevention ----
	b.WriteString("\n## 2 — What did fathom prevent?\n\n")
	fmt.Fprintf(&b, "Measured over %s recorded coding sessions", fmtThousands(imp.LedgerSessions))
	if imp.LedgerStops > imp.LedgerSessions {
		fmt.Fprintf(&b, " (from %s recorded stops — the harness fires its end-of-turn hook many times per session, and each session is counted once)", fmtThousands(imp.LedgerStops))
	}
	if imp.DateRange != "" {
		fmt.Fprintf(&b, " (%s)", imp.DateRange)
	}
	b.WriteString(".\n\n")

	if imp.VerifyCatchesTotal != nil {
		fmt.Fprintf(&b, "- **Plan claims caught before they were believed: %s.** The gate re-checks every \"done\" box in a plan against the code and fails when the code does not back the claim. %s.\n",
			fmtThousands(*imp.VerifyCatchesTotal), sentence(imp.VerifyCatchesNote))
		if ran := verifyRanSessions(imp.VerifyCatchesNote); ran == 0 {
			b.WriteString("  A zero here means the gate never ran — not that nothing was wrong.\n")
		} else if ran > 0 && *imp.VerifyCatchesTotal == 0 {
			b.WriteString("  A zero over that few sessions means the gate found nothing where it looked, not that nothing was wrong elsewhere.\n")
		}
	}
	if imp.DiscoveryTotal != nil {
		fmt.Fprintf(&b, "- **Discovery actions served: %s.** These are the \"where is it / who calls it / show me the source\" questions the agent asked the index instead of opening and reading files. %s.\n",
			fmtThousands(*imp.DiscoveryTotal), sentence(imp.DiscoveryNote))
	}
	fmt.Fprintf(&b, "- **Rule injections before an edit: %s.** Each one is a moment where the rules governing a file were placed in front of the agent *before* it wrote, rather than discovered afterward in review.\n", fmtThousands(imp.ObligationCalls))

	b.WriteString("\n**Reading:** these are counts of things fathom did, and they are large. They are *not* counts of mistakes avoided — nobody re-ran the same work without fathom to see what would have gone wrong. Treat them as exposure, not as outcomes.\n")

	// ---- 3. Tokens ----
	b.WriteString("\n## 3 — Did it save tokens? (the honest answer)\n\n")
	b.WriteString("A saving is a difference between two worlds: the same work done with fathom, and done without it. One number on its own cannot be a saving, so this brief refuses to print one until both worlds have been recorded (law 12).\n\n")
	b.WriteString("A unit is a session fully inside one arm, or the arm-covered window of a longer one (§5 amendment, 2026-08-12) — so working in long resumed sessions still produces control data.\n\n")
	fmt.Fprintf(&b, "- **Units with fathom on: %s**", fmtThousands(imp.TokensOnSessions))
	if imp.TokensOnMedian != nil {
		fmt.Fprintf(&b, " — median %s real tokens per turn (input + output; cache reads excluded, because counting them inflates the figure)", fmtThousands(*imp.TokensOnMedian))
	}
	b.WriteString("\n")
	fmt.Fprintf(&b, "- **Units with fathom off: %s**", fmtThousands(imp.TokensOffSessions))
	if imp.TokensOffMedian != nil {
		fmt.Fprintf(&b, " — median %s real tokens per turn", fmtThousands(*imp.TokensOffMedian))
	}
	b.WriteString("\n")
	fmt.Fprintf(&b, "- **Experiment state: %s** — %s\n", imp.PilotState, imp.PilotDetail)

	if imp.ExpSessions > 0 {
		fmt.Fprintf(&b, "\nSeparately, %s recorded what each *question* cost: %s calls of raw file exploration (about %s tokens) against %s indexed lookups (about %s tokens).",
			plural(imp.ExpSessions, "session"), fmtThousands(imp.ExpRawCalls), fmtThousands(imp.ExpRawTokens), fmtThousands(imp.ExpFathomCalls), fmtThousands(imp.ExpFathomTokens))
		if imp.ExpRatioMedianX10 != nil {
			fmt.Fprintf(&b, " Median cost ratio: %s× cheaper per question.", strconv.FormatFloat(float64(*imp.ExpRatioMedianX10)/10, 'f', 1, 64))
		}
		b.WriteString(" " + imp.ExpNote + "\n")
	}

	if imp.RatedOn > 0 || imp.RatedOff > 0 {
		fmt.Fprintf(&b, "- **Self-rated outcome:** %s ON session(s) rated", fmtThousands(imp.RatedOn))
		if imp.RatingOnMed != nil {
			fmt.Fprintf(&b, " (median %d/5)", *imp.RatingOnMed)
		}
		fmt.Fprintf(&b, ", %s OFF", fmtThousands(imp.RatedOff))
		if imp.RatingOffMed != nil {
			fmt.Fprintf(&b, " (median %d/5)", *imp.RatingOffMed)
		}
		b.WriteString("\n")
	}
	if imp.CriterionNote != "" {
		fmt.Fprintf(&b, "- **Registered criterion:** %s\n", imp.CriterionNote)
	}
	if imp.UncleanArmRows > 0 {
		fmt.Fprintf(&b, "- **Excluded from the comparison: %s** — %s\n", fmtThousands(imp.UncleanArmRows), imp.UncleanArmNote)
	}

	switch {
	case imp.TokensOffSessions == 0:
		b.WriteString("\n**Reading:** unknown, and deliberately left unknown. Anyone quoting a fathom savings percentage today would be quoting an estimate, not a measurement.\n")
	case imp.TokensOnMedian == nil || imp.TokensOffMedian == nil:
		// Sessions on both sides is not the same as tokens on both sides;
		// promising a comparison that is not on the page is the small lie
		// that makes a reader distrust the honest figures beside it.
		b.WriteString("\n**Reading:** still unknown. Both arms now have sessions, but at least one side has no usable token total yet, so there is no difference to state.\n")
	default:
		b.WriteString("\n**Reading:** compare the two medians above; the difference is the only savings figure this brief will stand behind.\n")
	}

	// ---- 4. Weaknesses ----
	b.WriteString("\n## 4 — Where does the agent still flounder?\n\n")
	fmt.Fprintf(&b, "- **Open defects across the fleet: %s.** Each is a written bug report with a reproduction, filed by an agent the moment a fathom tool answered worse than expected.\n", fmtThousands(briefOpenObservations(m)))
	if len(m.Misses) > 0 {
		var noisy []string
		for i, mr := range m.Misses {
			if i == 3 {
				break
			}
			noisy = append(noisy, fmt.Sprintf("%s %s calls", mr.Tool, fmtThousands(mr.Calls)))
		}
		b.WriteString("- **Most-called tools:** " + strings.Join(noisy, " · ") + " — per-tool hit and miss rates are on `/observatory/impact` (the query text itself is never logged)\n")
	}
	b.WriteString("\n**Reading:** a growing defect list is a sign the instrument is being used honestly, not a sign it is failing. The number to watch is how long each one stays open.\n")

	// ---- 5. Compounding ----
	b.WriteString("\n## 5 — Is the knowledge compounding?\n\n")
	b.WriteString("The question behind this one: does a repository get *cheaper* to work in the longer fathom has been running there?\n\n")
	if imp.FirstEditMedian != nil {
		fmt.Fprintf(&b, "- **Warm-up: %s turns before the first edit** (median). %s\n", fmtThousands(*imp.FirstEditMedian), sentence(imp.FirstEditNote))
	} else {
		fmt.Fprintf(&b, "- **Warm-up: not yet measured.** %s\n", sentence(imp.FirstEditNote))
	}
	fmt.Fprintf(&b, "- **Rules served: %s** — the proxy for accumulated, reusable knowledge about this codebase\n", fmtThousands(imp.ObligationCalls))

	// ---- Continuity (vision 21) ----
	bet := threadBetCheck(dedupedLedgerRows())
	b.WriteString("\n## 6 — Is continuity making sessions disposable?\n\n")
	b.WriteString("Threads carry open work across `/clear` and compaction, so a session can end without its intent dying. The registered bet: a fresh session opened with a thread brief warms up as fast as a resumed one, at lower all-in cost.\n\n")
	fmt.Fprintf(&b, "- **Fresh-with-brief units: %s** of the %s the registered floor requires; **resumed baseline units: %s**\n",
		fmtThousands(bet.FreshUnits), fmtThousands(threadBetMinFreshUnits), fmtThousands(bet.ResumedUnits))
	if bet.FreshWarmupMed != nil && bet.ResumedWarmupMed != nil {
		fmt.Fprintf(&b, "- **Warm-up:** %s vs %s median turns to first edit (fresh vs resumed)\n",
			fmtThousands(*bet.FreshWarmupMed), fmtThousands(*bet.ResumedWarmupMed))
	}
	if bet.FreshAllInMed != nil && bet.ResumedAllInMed != nil {
		fmt.Fprintf(&b, "- **All-in cost:** %s vs %s tokens per turn (in + out + cache reads weighted at the 10%% pricing assumption)\n",
			fmtThousands(*bet.FreshAllInMed), fmtThousands(*bet.ResumedAllInMed))
	}
	fmt.Fprintf(&b, "- **What continuity itself has spent:** %s brief bytes across all sessions — a ledger sum, not an estimate\n", fmtThousands(bet.BriefBytesTotal))
	fmt.Fprintf(&b, "- **Verdict: %s** — %s\n", bet.Verdict, bet.Detail)

	// ---- Work order ----
	b.WriteString("\n## Could fathom help more if it were configured differently?\n\n")
	if len(acts) == 0 {
		b.WriteString("No. Every instrument this brief reads is switched on and reporting.\n")
	} else {
		b.WriteString("Yes — here is exactly what is switched off, what switching it on would let a future brief say, and the command. Ordered by how much each one changes the picture.\n\n")
		for i, a := range acts {
			fmt.Fprintf(&b, "**%d. %s**\n", i+1, a.What)
			fmt.Fprintf(&b, "- Would let this brief say: %s\n", a.Unlock)
			if a.Cmd != "" {
				fmt.Fprintf(&b, "- Command: `%s`\n", a.Cmd)
			}
			if a.Note != "" {
				fmt.Fprintf(&b, "- Note: %s\n", a.Note)
			}
			b.WriteString("\n")
		}
	}

	// ---- Absence ----
	b.WriteString("## Not measured — named rather than shown as zero\n\n")
	absent := 0
	if imp.LedgerNote != "" {
		b.WriteString("- " + imp.LedgerNote + "\n")
		absent++
	}
	if imp.VerifyCatchesTotal == nil {
		b.WriteString("- " + imp.VerifyCatchesNote + "\n")
		absent++
	}
	if imp.DiscoveryTotal == nil {
		b.WriteString("- " + imp.DiscoveryNote + "\n")
		absent++
	}
	if imp.TokensOnMedian == nil && imp.LedgerSessions > 0 {
		b.WriteString("- token totals for the on-arm sessions are absent from the ledger, so no median can be shown\n")
		absent++
	}
	if imp.FirstEditMedian == nil {
		b.WriteString("- " + imp.FirstEditNote + "\n")
		absent++
	}
	if absent == 0 {
		b.WriteString("- every instrument above reported a real value\n")
	}

	b.WriteString("\n---\n\n")
	b.WriteString("Narrative story rows, when captured, are a lens over mechanism-labeled events. They are deliberately not quoted as evidence in this brief (law 14). Nothing in this file was sent anywhere — fathom wrote it locally and stopped.\n")
	return b.String()
}

// briefOpenObservations mirrors fleetOpenObservations for the brief.
func briefOpenObservations(m observatoryModel) int {
	total := 0
	for _, s := range m.Services {
		if s.OpenObservations != nil {
			total += *s.OpenObservations
		}
	}
	return total
}

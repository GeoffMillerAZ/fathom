package console

// learn.go — OC-46 (vision 35 §4.1, design §3 Learn): "How fathom works",
// the pipeline drawn with live counts for one chosen project. Source →
// extract (dialect packs) → index (symbols, typed edges) → compile (layer
// packs → claims) → serve (rules at the edit) → gate (verify, conserve) →
// evidence (ledgers). Each stage carries its live counts with their truth
// states, the switches (`fathom capabilities` rows) that deliver it here,
// and the pages that show it in full.
//
// PipelineView is pure: the caller reads the store, the call log, the
// ledgers and the capability probe once and hands the facts in, so the
// stage table is testable without a daemon and the same facts can never be
// counted two ways.

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
)

// PipelineStageID names one stage of the pipeline.
type PipelineStageID string

const (
	StageSource   PipelineStageID = "source"
	StageExtract  PipelineStageID = "extract"
	StageIndex    PipelineStageID = "index"
	StageCompile  PipelineStageID = "compile"
	StageServe    PipelineStageID = "serve"
	StageGate     PipelineStageID = "gate"
	StageEvidence PipelineStageID = "evidence"
)

// StageStatus is whether a stage is working for the project.
type StageStatus string

const (
	// StageOperating: every switch the stage needs is on (for the source
	// stage: the index is fresh at HEAD).
	StageOperating StageStatus = "operating"
	// StageWorking: the stage's own headline shows it at work here while a
	// switch it can use is off — absent (its inputs do not exist here, and
	// the stage works without them) or broken (Remedy fixes it). A status
	// never contradicts the counts drawn beside it.
	StageWorking StageStatus = "working"
	// StageOff: a switch the stage needs is off here and the stage's own
	// headline shows no work; Reason and Remedy say which and what turns it
	// on.
	StageOff StageStatus = "off"
	// StageStale: the index is behind HEAD — the source stage, and the
	// stages that read the index (extract, index) run on the older copy.
	StageStale StageStatus = "stale"
	// StageUnknown: nothing reported whether the stage works here — never
	// read as operating.
	StageUnknown StageStatus = "unknown"
)

// CapabilityState is one `fathom capabilities` row for the project: whether
// a switch that delivers a stage is operating here, and if not, why not and
// what turns it on.
type CapabilityState struct {
	ID        string `json:"id"`
	Operating bool   `json:"operating"`
	// Broken separates the two ways a switch is off (internal/capability):
	// absent — its inputs do not exist here, normal and often deliberate —
	// and broken — its inputs exist and something fails to deliver them.
	Broken bool   `json:"broken,omitempty"`
	Detail string `json:"detail,omitempty"`
	Reason string `json:"reason,omitempty"`
	Remedy string `json:"remedy,omitempty"`
	// Required is set by PipelineView: a required switch that is off
	// decides its stage's status (working, off or stale, against the
	// stage's own counts); an optional one (guards, modules) is listed but
	// never does.
	Required bool `json:"required"`
}

// LanguageFiles is one language the index holds files for, with the
// typed-edge mechanism fathom's dialect pack provides for it ("native",
// "scip:<tool>", "file", "syntax"; "" when no pack is registered).
type LanguageFiles struct {
	Lang     string `json:"lang"`
	Files    int    `json:"files"`
	Producer string `json:"producer"`
}

// PipelineFacts is what the caller read for one project. Every count is a
// Value: a source that was not read is absent with its reason, never 0.
type PipelineFacts struct {
	Service string
	// Freshness is the fleet report's verdict: "fresh", "stale: …" or
	// "unknown: …" ("" when nothing was read).
	Freshness string
	// FreshnessRemedy is the command that refreshes a stale index (the
	// fleet row's own remedy); "" when none applies.
	FreshnessRemedy string
	IndexedCommit   string
	RepoHead        string

	Files     Value[int] // files the index holds (store source refs)
	Languages []LanguageFiles
	// UnclassifiedFiles are indexed files no dialect pack reads (held for
	// their fingerprint, never parsed into symbols).
	UnclassifiedFiles int
	Symbols           Value[int]

	Edges      Value[int] // typed-edge rows
	TypedEdges Value[int] // of those, resolved (type-checked)

	ClaimsCurrent Value[int] // served and fresh
	ClaimsFlagged Value[int] // served but stale or contradicted
	ClaimsRetired Value[int] // superseded or refuted, never served
	Layers        []string
	CompileRuns   Value[int]

	RulesServed Value[int] // obligations calls in the traffic window
	WindowCalls Value[int] // every tool call in the traffic window

	GateRuns     Value[int] // verify-ledger rows for the project
	GatePlans    Value[int] // distinct plans those rows name
	GateVerified Value[int] // claimed rows verified at each plan's latest run
	GateClaimed  Value[int] // claimed rows at each plan's latest run

	Sessions Value[int] // distinct sessions (the fleet report's session_count)
	Calls    Value[int] // every logged call for the project (fleet report)

	// Capabilities are the project's `fathom capabilities` rows;
	// CapabilitiesNote says why there are none (the probe did not run).
	Capabilities     []CapabilityState
	CapabilitiesNote string
}

// PipelineCount is one labelled count on a stage.
type PipelineCount struct {
	Label string     `json:"label"`
	Value Value[int] `json:"value"`
}

// PipelineStage is one stage for one project.
type PipelineStage struct {
	ID       PipelineStageID `json:"id"`
	Step     int             `json:"step"`
	Title    string          `json:"title"`
	Pieces   string          `json:"pieces"`   // what does the work ("dialect packs")
	Does     string          `json:"does"`     // what happens here, one sentence
	Question string          `json:"question"` // what the stage's panel answers
	// Headline is the stage's one number, drawn on the pipeline; Counts
	// are the rest, drawn on the stage's panel.
	Headline PipelineCount   `json:"headline"`
	Counts   []PipelineCount `json:"counts,omitempty"`
	Notes    []string        `json:"notes,omitempty"`
	// Capabilities are the switches that deliver this stage here.
	Capabilities []CapabilityState `json:"capabilities,omitempty"`
	Status       StageStatus       `json:"status"`
	StatusReason string            `json:"status_reason,omitempty"`
	Remedy       string            `json:"remedy,omitempty"`
	// Pages are the console routes that show this stage in full, most
	// specific first, the project filled in; the renderer links the first
	// one the console has registered.
	Pages []string `json:"pages"`
	// Live is the live-freshness source the stage's numbers move with.
	Live string `json:"live"`
}

// PipelineEdge is one hand-off between stages.
type PipelineEdge struct {
	From PipelineStageID `json:"from"`
	To   PipelineStageID `json:"to"`
}

// Pipeline is the whole map for one project.
type Pipeline struct {
	Service   string          `json:"service"`
	Stages    []PipelineStage `json:"stages"`
	Edges     []PipelineEdge  `json:"edges"`
	Languages []LanguageFiles `json:"languages,omitempty"`
}

// pipelineStageDef is a stage's fixed description.
type pipelineStageDef struct {
	id                 PipelineStageID
	title, pieces      string
	does, question     string
	required, optional []string
	pages              []string // "{svc}" is the project
	live               string   // "{svc}" is the project
}

// pipelineStages is the pipeline in order (vision 35 §4.1). The switches
// are `fathom capabilities` IDs (internal/capability.Default); the pages
// are the design's routes (design §1), most specific first — the project
// workspace's tab (/projects/{svc}/index, console wave E) or a page
// filtered to the project (/plans?svc=, /layers?svc=, the workspace's own
// governance and plans tabs), then the workspace itself where its overview
// shows the stage's facts (symbols, freshness, layers, rules served). A
// stage whose routes no page serves has no link, and says so.
var pipelineStages = []pipelineStageDef{
	{
		id: StageSource, title: "Source", pieces: "your repository at a commit",
		does:     "fathom reads the repository's code and its governing documents at a commit, and records which commit it read.",
		question: "What does fathom read here, and is its copy current?",
		optional: []string{"intent-corpus"},
		pages:    []string{"/projects/{svc}"},
		live:     "index:{svc}",
	},
	{
		id: StageExtract, title: "Extract", pieces: "dialect packs",
		does:     "One dialect pack per language turns each file into symbols and call edges; the pack decides how exact those edges can be.",
		question: "Which dialect packs turn this project's files into symbols?",
		required: []string{"index"},
		pages:    []string{"/packs", "/projects/{svc}/index", "/projects/{svc}"},
		live:     "index:{svc}",
	},
	{
		id: StageIndex, title: "Index", pieces: "symbols and typed edges",
		does:     "The symbols and edges land in the store, keyed by the commit; typed edges are the ones a type checker or a SCIP index resolved, not guessed from names.",
		question: "How much of the call graph is typed rather than guessed?",
		required: []string{"index"},
		pages:    []string{"/projects/{svc}/index"},
		live:     "index:{svc}",
	},
	{
		id: StageCompile, title: "Compile", pieces: "layer packs → claims",
		does:     "Layer packs compile governing documents into claims: each restates a rule, cites its lines and names the code it governs, and is swept stale when either moves.",
		question: "Which governing claims have layer packs compiled here?",
		required: []string{"layers"},
		optional: []string{"modules"},
		pages:    []string{"/layers", "/layers?svc={svc}", "/projects/{svc}"},
		live:     "layers:{svc}",
	},
	{
		id: StageServe, title: "Serve", pieces: "rules at the edit",
		does:     "Before an agent edits a file, the rules that govern it are put in front of it; the agent surface answers where/who-calls/what-breaks from the index.",
		question: "Are rules and answers reaching the agent before it edits?",
		required: []string{"obligations", "agent-surface"},
		optional: []string{"guards"},
		pages:    []string{"/explain?svc={svc}", "/layers?svc={svc}", "/projects/{svc}"},
		live:     "sessions:{svc}",
	},
	{
		id: StageGate, title: "Gate", pieces: "verify and conserve",
		does:     "Before a push, every box claimed done in a plan is re-checked against the code; a refactor is checked for what it had to keep.",
		question: "Is the gate checking claimed work against the code?",
		required: []string{"verify-gate"},
		pages:    []string{"/plans?svc={svc}", "/plans"},
		live:     "plans:{svc}",
	},
	{
		id: StageEvidence, title: "Evidence", pieces: "ledgers and receipts",
		does:     "Sessions, gate runs and tool calls are appended to ledgers, so what fathom did here is measured, not asserted.",
		question: "What have the ledgers recorded for this project?",
		required: []string{"session-ledger"},
		pages:    []string{"/impact", "/sessions"},
		live:     "sessions:{svc}",
	},
}

// PipelineStageIDs is the pipeline's order.
func PipelineStageIDs() []PipelineStageID {
	out := make([]PipelineStageID, 0, len(pipelineStages))
	for _, d := range pipelineStages {
		out = append(out, d.id)
	}
	return out
}

// PipelineView draws the pipeline for one project from its facts.
func PipelineView(f PipelineFacts) Pipeline {
	p := Pipeline{Service: f.Service, Languages: append([]LanguageFiles(nil), f.Languages...)}
	caps := map[string]CapabilityState{}
	for _, c := range f.Capabilities {
		caps[c.ID] = c
	}
	for i, d := range pipelineStages {
		st := PipelineStage{
			ID: d.id, Step: i + 1, Title: d.title, Pieces: d.pieces, Does: d.does, Question: d.question,
			Pages: fillService(d.pages, f.Service),
			Live:  strings.ReplaceAll(d.live, "{svc}", f.Service),
		}
		st.Headline, st.Counts, st.Notes = stageCounts(d.id, f)
		for _, id := range d.required {
			if c, ok := caps[id]; ok {
				c.Required = true
				st.Capabilities = append(st.Capabilities, c)
			}
		}
		for _, id := range d.optional {
			if c, ok := caps[id]; ok {
				c.Required = false
				st.Capabilities = append(st.Capabilities, c)
			}
		}
		st.Status, st.StatusReason, st.Remedy = stageStatus(d, f, caps)
		p.Stages = append(p.Stages, st)
		if i > 0 {
			p.Edges = append(p.Edges, PipelineEdge{From: pipelineStages[i-1].id, To: d.id})
		}
	}
	return p
}

// stageCounts is each stage's headline, counts and notes.
func stageCounts(id PipelineStageID, f PipelineFacts) (PipelineCount, []PipelineCount, []string) {
	switch id {
	case StageSource:
		var notes []string
		if f.IndexedCommit != "" {
			notes = append(notes, "indexed at "+shortRev(f.IndexedCommit))
		}
		if f.RepoHead != "" {
			notes = append(notes, "HEAD "+shortRev(f.RepoHead))
		}
		langs := Absent[int]("the index's files were not read")
		if f.Files.Known() {
			langs = Measured(len(f.Languages), f.Files.Source).WithDenominator("languages among the indexed files")
		} else if f.Files.State == StateError {
			langs = Errored[int](f.Files.Reason)
		}
		return PipelineCount{"files indexed", f.Files}, []PipelineCount{{"languages", langs}}, notes
	case StageExtract:
		var notes []string
		typed := 0
		for _, l := range f.Languages {
			producer := l.Producer
			if producer == "" {
				producer = "no dialect pack registered"
			}
			notes = append(notes, fmt.Sprintf("%s · %s · %s", l.Lang, countNoun(l.Files, "file"), producerWords(producer)))
			if l.Producer != "" && l.Producer != "syntax" {
				typed++
			}
		}
		if f.UnclassifiedFiles > 0 {
			notes = append(notes, countNoun(f.UnclassifiedFiles, "file")+" no dialect pack reads (held for their fingerprint only)")
		}
		withTyped := Absent[int]("the index's files were not read")
		if f.Files.Known() {
			withTyped = Measured(typed, "connector.TypedEdgeProducers").WithDenominator(fmt.Sprintf("of %s", countNoun(len(f.Languages), "language")))
		} else if f.Files.State == StateError {
			withTyped = Errored[int](f.Files.Reason)
		}
		return PipelineCount{"symbols", f.Symbols}, []PipelineCount{{"languages with a typed-edge pack", withTyped}}, notes
	case StageIndex:
		var notes []string
		if f.Edges.Known() && f.TypedEdges.Known() && f.Edges.V > 0 {
			notes = append(notes, fmt.Sprintf("%.1f%% of edges resolved by a type checker or SCIP index", 100*float64(f.TypedEdges.V)/float64(f.Edges.V)))
		}
		return PipelineCount{"typed edges", f.Edges}, []PipelineCount{{"resolved (type-checked) edges", f.TypedEdges}}, notes
	case StageCompile:
		var notes []string
		if len(f.Layers) > 0 {
			layers := append([]string(nil), f.Layers...)
			sort.Strings(layers)
			notes = append(notes, "layers: "+strings.Join(layers, ", "))
		}
		return PipelineCount{"current claims", f.ClaimsCurrent}, []PipelineCount{
			{"served but flagged (stale or contradicted)", f.ClaimsFlagged},
			{"retired (superseded or refuted)", f.ClaimsRetired},
			{"compile runs", f.CompileRuns},
		}, notes
	case StageServe:
		return PipelineCount{"rules served", f.RulesServed}, []PipelineCount{{"tool calls answered", f.WindowCalls}},
			[]string{"both counts cover the last 30 days of the call log; the console's own reads are not counted"}
	case StageGate:
		var notes []string
		if f.GateVerified.Known() && f.GateClaimed.Known() && f.GateClaimed.V > 0 {
			notes = append(notes, fmt.Sprintf("%s of %s claimed rows verified at each plan's latest gate run", thousands(f.GateVerified.V), thousands(f.GateClaimed.V)))
		}
		return PipelineCount{"gate runs", f.GateRuns}, []PipelineCount{
			{"plans gated", f.GatePlans},
			{"claimed rows verified (latest run per plan)", f.GateVerified},
			{"claimed rows (latest run per plan)", f.GateClaimed},
		}, notes
	case StageEvidence:
		return PipelineCount{"sessions", f.Sessions}, []PipelineCount{{"tool calls logged, all time", f.Calls}}, nil
	}
	return PipelineCount{}, nil, nil
}

// stageStatus decides whether a stage works here. The source stage reads
// freshness; every other stage reads its required switches against its own
// headline, so the status never contradicts the counts beside it: a stale
// index makes the stages that read it stale (with the daemon-routed
// refresh); a switch off on a stage whose headline shows work is a working
// stage with the switch named (and its remedy when it is broken); a switch
// off on a stage that shows no work turns it off. No report is unknown —
// never operating.
func stageStatus(d pipelineStageDef, f PipelineFacts, caps map[string]CapabilityState) (StageStatus, string, string) {
	if d.id == StageSource {
		switch {
		case f.Freshness == "fresh":
			return StageOperating, "", ""
		case strings.HasPrefix(f.Freshness, "stale"):
			return StageStale, "the index is behind HEAD (" + strings.TrimSpace(strings.TrimPrefix(f.Freshness, "stale:")) + ")", f.FreshnessRemedy
		case f.Freshness == "":
			return StageUnknown, "freshness was not read for this project", ""
		}
		return StageUnknown, f.Freshness, ""
	}
	if len(f.Capabilities) == 0 {
		reason := f.CapabilitiesNote
		if reason == "" {
			reason = "no capability report for this project"
		}
		return StageUnknown, reason, ""
	}
	headline, _, _ := stageCounts(d.id, f)
	working := headline.Value.Known() && headline.Value.V > 0
	var missing []string
	for _, id := range d.required {
		c, ok := caps[id]
		if !ok {
			missing = append(missing, id+" (not reported)")
			continue
		}
		if c.Operating {
			continue
		}
		reason := c.Reason
		if reason == "" {
			reason = "off"
		}
		switch {
		case id == "index" && c.Broken && strings.HasPrefix(f.Freshness, "stale"):
			remedy := f.FreshnessRemedy
			if remedy == "" {
				remedy = c.Remedy
			}
			return StageStale, "the index is behind HEAD (" + strings.TrimSpace(strings.TrimPrefix(f.Freshness, "stale:")) + ")", remedy
		case working && c.Broken:
			return StageWorking, id + " is broken here: " + reason + " — the stage works, but not fully", c.Remedy
		case working:
			return StageWorking, id + " is off here: " + reason + " — this stage's own counts show it at work without it", ""
		}
		return StageOff, id + ": " + reason, c.Remedy
	}
	if len(missing) > 0 {
		return StageUnknown, "the capability report does not name " + strings.Join(missing, ", "), ""
	}
	return StageOperating, "", ""
}

// producerWords says what a typed-edge producer means for a reader.
func producerWords(p string) string {
	switch {
	case p == "native":
		return "typed edges from the native type checker"
	case strings.HasPrefix(p, "scip:"):
		return "typed edges when a " + strings.TrimPrefix(p, "scip:") + " SCIP index is imported"
	case p == "file":
		return "typed edges from a provided edge file"
	case p == "syntax":
		return "syntax edges only (names, not types)"
	}
	return p
}

// fillService fills "{svc}" in each route with the project, path-escaped
// in a path and query-escaped in a query.
func fillService(routes []string, svc string) []string {
	out := make([]string, 0, len(routes))
	for _, r := range routes {
		path, query, hasQuery := strings.Cut(r, "?")
		path = strings.ReplaceAll(path, "{svc}", url.PathEscape(svc))
		if hasQuery {
			query = strings.ReplaceAll(query, "{svc}", url.QueryEscape(svc))
			path += "?" + query
		}
		out = append(out, path)
	}
	return out
}

// shortRev shortens a commit to 12 characters; a manifest watermark keeps
// its "manifest:" prefix.
func shortRev(rev string) string {
	if strings.HasPrefix(rev, "manifest:") {
		body := strings.TrimPrefix(rev, "manifest:")
		if len(body) > 12 {
			body = body[:12]
		}
		return "manifest:" + body
	}
	if len(rev) > 12 {
		return rev[:12]
	}
	return rev
}

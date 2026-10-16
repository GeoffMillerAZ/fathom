package harness

// eventcatalog.go — what each harness OFFERS versus what fathom wires
// (HP-32). The catalogs below are the primary documentation's own event
// lists, transcribed on the date in each descriptor's `verified_against`,
// with one reason per unwired row.
//
// Two rules keep this honest and cheap to maintain:
//
//   - `Wired` is never hand-maintained truth: TestEventCatalogMatchesBindings
//     proves every Wired row has a binding with that harness event name and
//     every binding's event has a Wired row, so a new binding whose catalog
//     row was forgotten fails the gate rather than lying in the table.
//   - Every unwired row carries a REASON. "Not wired" with no reason is the
//     shape of an oversight; a reason is a decision an adopter can argue
//     with. The commonest reasons, named once here rather than retyped:
//     the event carries no signal fathom has anything true to say about,
//     the event can only block (and §5.3 makes fathom inject-only), or the
//     event's cost would land on every session for a payload fathom would
//     have to invent.
const (
	reasonNoSignal  = "fathom knows nothing about this event that the session does not already know — wiring it would cost a process launch per firing for no payload"
	reasonBlockOnly = "the event's only control is a block, and fathom is inject-only by decision — a fathom entry here could only refuse, never advise"
	reasonCovered   = "already covered by a wired event: "
)

// claudeEventCatalog is the 33-event surface of Claude Code 2.1.270, in
// the reference's own order (https://code.claude.com/docs/en/hooks,
// fetched 2026-09-12).
var claudeEventCatalog = []EventCatalogEntry{
	{Event: "SessionStart", Wired: true},
	{Event: "Setup", Wired: true},
	{Event: "InstructionsLoaded", Wired: true},
	// Wired 2026-09-14 (O57 items 4/5): this row read "nothing true to add
	// per prompt" until the guards pilot's second reading found the
	// opposite — a prompt naming a concrete path, package or symbol has
	// something true and specific to add (the rules governing exactly
	// what it named), a materially different payload from the generic
	// per-prompt injection vision 21 declined. fathom prompt-hook.
	{Event: "UserPromptSubmit", Wired: true},
	{Event: "UserPromptExpansion", Wired: false, Reason: reasonNoSignal},
	{Event: "PreToolUse", Wired: true},
	{Event: "PermissionRequest", Wired: false, Reason: reasonBlockOnly},
	{Event: "PermissionDenied", Wired: false, Reason: "a denial is the user's decision; fathom neither records nor second-guesses it"},
	{Event: "PostToolUse", Wired: true},
	{Event: "PostToolUseFailure", Wired: true},
	{Event: "PostToolBatch", Wired: false, Reason: reasonCovered + "PostToolUse fires per call in the batch, and the obligations payload is per file"},
	{Event: "Notification", Wired: false, Reason: reasonNoSignal},
	{Event: "MessageDisplay", Wired: false, Reason: "rewriting what the user sees on screen is not a thing an evidence engine may do (vision 22 §3b: fathom reports, the hand decides)"},
	{Event: "SubagentStart", Wired: true},
	{Event: "SubagentStop", Wired: false, Reason: "the subagent's claims are verified where they LAND — the parent's Stop hook and the pre-push gate read the same repo; a second verify per subagent would re-run the same proof"},
	{Event: "TaskCreated", Wired: false, Reason: reasonBlockOnly},
	{Event: "TaskCompleted", Wired: false, Reason: "a task is not a plan row: `fathom verify` gates the docs/plans/*.md claim, which is the artifact that has to be true (PC-22)"},
	{Event: "Stop", Wired: true},
	{Event: "StopFailure", Wired: false, Reason: "an API error is the harness's failure, not the repo's — nothing fathom measures or can remedy"},
	{Event: "TeammateIdle", Wired: false, Reason: reasonBlockOnly},
	{Event: "ConfigChange", Wired: true},
	{Event: "CwdChanged", Wired: true},
	{Event: "DirectoryAdded", Wired: true},
	{Event: "FileChanged", Wired: true},
	{Event: "WorktreeCreate", Wired: false, Reason: "a creation hook: the harness expects a registered hook to create the worktree and print its path, and refuses any other stdout (O34) — the worktree line rides on SubagentStart instead"},
	{Event: "WorktreeRemove", Wired: false, Reason: "nothing to say: a removed worktree never re-rooted the service (the WorktreeCreate note is why), so its removal changes no index fact"},
	{Event: "PreCompact", Wired: true},
	{Event: "PostCompact", Wired: false, Reason: reasonCovered + "PreCompact deposits the thread pointer BEFORE the context is gone, which is the only moment the deposit is worth anything"},
	{Event: "PreModelSwitch", Wired: false, Reason: reasonBlockOnly},
	{Event: "PostModelSwitch", Wired: false, Reason: "re-priming on a model switch would re-charge the whole session primer for a session that kept its context — the same reason the SessionStart primer is gated to startup/clear"},
	{Event: "Elicitation", Wired: false, Reason: reasonBlockOnly},
	{Event: "ElicitationResult", Wired: false, Reason: reasonNoSignal},
	{Event: "SessionEnd", Wired: true},
}

// codexEventCatalog is Codex CLI 0.153.1/0.154's twelve events. Six of
// the events fathom wires on Claude Code have NO Codex equivalent at all
// — they are absent from the harness, which is a different fact from
// "fathom declined to wire them", and the catalog cannot express it
// (a catalog row is an event the harness offers). codexAbsentEvents
// carries that half, and the generated capability table renders it.
var codexEventCatalog = []EventCatalogEntry{
	{Event: "SessionStart", Wired: true},
	{Event: "SessionEnd", Wired: true},
	{Event: "UserPromptSubmit", Wired: false, Reason: "same reason as Claude Code: nothing true to add per prompt; `fathom prime` is the pull-tier equivalent"},
	{Event: "PreToolUse", Wired: true},
	{Event: "PermissionRequest", Wired: false, Reason: reasonBlockOnly},
	{Event: "PostToolUse", Wired: true},
	{Event: "PreCompact", Wired: true},
	{Event: "PostCompact", Wired: false, Reason: reasonCovered + "PreCompact deposits the thread pointer while the context still exists"},
	{Event: "SubagentStart", Wired: true},
	{Event: "SubagentStop", Wired: false, Reason: "verified where the claims land (the parent's Stop hook and the pre-push gate), not once per subagent"},
	{Event: "Stop", Wired: true},
	{Event: "Interrupt", Wired: false, Reason: "an interrupted turn has no completed claim to verify and no edit to serve obligations for; the next session's thread brief carries the continuity"},
}

// codexAbsentEvents are the harness events fathom wires on Claude Code
// that Codex does not offer at all, with what the adopter loses. Rendered
// beside the capability table so a Codex adopter reads the gap as a
// property of the harness, never as a fathom omission (ADR-029 §5).
var codexAbsentEvents = []EventCatalogEntry{
	{Event: "PostToolUseFailure", Reason: "Codex documents no post-failure event — a failed fathom tool call gets no remedy line; run `fathom doctor`"},
	{Event: "InstructionsLoaded", Reason: "Codex documents no instructions-loaded event — doctor's stanza_loaded row has no harness-side evidence here and reads the file instead"},
	{Event: "ConfigChange", Reason: "Codex documents no config-change event — run `fathom config audit` by hand"},
	{Event: "FileChanged", Reason: "Codex documents no file-watch event — the custodian audit has no on-disk trigger here"},
	{Event: "WorktreeCreate", Reason: "Codex creates no worktrees of its own and documents no event — the worktree/service note is not delivered"},
	{Event: "CwdChanged", Reason: "Codex documents no cwd-change or directory-added event — a session that moves root keeps the identity line it opened with"},
	{Event: "Setup", Reason: "Codex documents no setup event — run `fathom doctor --brief` by hand after wiring"},
}

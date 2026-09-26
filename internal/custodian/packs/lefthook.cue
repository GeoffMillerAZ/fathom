// The lefthook pack (CC-12, vision 22 §7): the git-hook gate, held to
// honesty rather than syntax — the hook still runs the gate the docs
// promise, and every target and script it names still exists. Enable it
// in .fathom/concerns.cue with `packs: ["lefthook"]`; a repo whose gate
// is not `task validate` restates lefthook_runs_gate with its own command
// (same id, replaced wholesale — law 13).
//
// lefthook_pre_push_verify is what doctor's lefthook_gates row renders
// (CC-13); its pattern is lefthookInvokes' (cmd/fathom), held equal by
// TestLefthookPackPatternMatchesDoctor.
pack: {
	name:    "lefthook"
	version: 1
	about:   "the git-hook gate: the hooks run fathom verify and the repo's gate, and every task and script a hook runs exists"
}

concerns: {
	lefthook_pre_push_verify: {
		want:  "the lefthook config runs `fathom verify` (any spelling of the binary: bin/fathom, $FATHOM, an absolute path)"
		why:   "I17: a claimed-but-unproven plan row blocks a push only when the hook runs verify — and the literal-substring check this replaced told a correctly gated repo to add what it had (IO-5)"
		check: {kind: "required_pattern", where: "lefthook", pattern: #"(?i)fathom\}?"?'?\s+verify\b"#}
	}
	lefthook_runs_gate: {
		want:  "the lefthook config runs the repo's deterministic gate, `task validate`, and that target exists (override this entry with your gate's command)"
		why:   "the docs → lefthook link of the gate chain: the hook still runs the gate the docs promise — O25 was a documented command that outlived reality"
		check: {kind: "command_documented_and_exists", where: "lefthook", command: "task validate"}
	}
	lefthook_commands_resolve: {
		want:  "every `task` target and repo script a lefthook hook runs exists"
		why:   "the lefthook → Taskfile → scripts links: a hook naming a renamed target fails every push — or, written fail-open, stops gating without a sound"
		check: {kind: "commands_resolve", where: "lefthook"}
	}
}

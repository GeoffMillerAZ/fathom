// Four falsifiable checks; local config only. Notes name observed failures.
concerns: {
	current_guidance: {
		want: "agent instructions must not advertise retired conversion or fidelity policy"
		why: "INC-6/INC-11: stale guidance erased design states and denied implemented commands"
		check: {kind: "forbidden_pattern", pattern: "(?i)conversion machinery is planned, not an available command|dark: color variants (are )?dropped|palette families become roles"}
	}
	maintainer_gate: {
		want: "task check remains documented and executable as a declared task"
		why: "INC-9: a served setup command did not exist"
		check: {kind: "command_documented_and_exists", command: "task check"}
	}
	pointers_hold: {
		want: "agent guidance pointers resolve to current local documents and artifacts"
		why: "INC-1/INC-9: stale routing sent consumers to incorrect setup guidance"
		check: {kind: "pointers_resolve"}
	}
	served_stanza: {
		want: "Fathom stanza tool names are served on the actual agent surface"
		why: "O1/O4: retrieval instructions and the actual tool contract diverged"
		check: {kind: "stanza_tools_served"}
	}
}

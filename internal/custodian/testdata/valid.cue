// A positive fixture: every v1 check kind, loaded by TestValidFixtureLoads.
concerns: {
	deploy_safety: {
		want:  "no raw-cp deploy instruction anywhere in agent config"
		why:   "O25: an unsigned binary took the daemon down fleet-wide"
		check: {kind: "forbidden_pattern", pattern: "cp .*libexec/fathom", where: "agent-config"}
	}
	gate_documented: {
		want:  "the validate command is documented and exists"
		check: {kind: "command_documented_and_exists", command: "task validate"}
	}
	router_thin: {
		want:  "the root CLAUDE.md stays a router"
		check: {kind: "max_lines", file: "CLAUDE.md", lines: 120}
	}
	pointers_hold: {
		want:  "every relative path in agent config resolves"
		check: {kind: "pointers_resolve"}
	}
}

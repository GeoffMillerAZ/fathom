package main

// doctor_repoonly.go — `fathom doctor --repo-only` (OC-40): the subset of
// doctor's rows that read only the repository's own files and the user
// scope's config — hooks, gates, the stanza, skills, wiring drift and the
// .fathom files. No row here dials the daemon, spawns a harness process or
// reads a ledger, so the Observatory's project wiring tab can render it on
// a page load; the full diagnosis (41 rows, ~14 s with an unreachable
// daemon) stays `fathom doctor`. Each row is doctor's own check function,
// in doctor's own order — the tab and the verb cannot disagree.

// doctorRepoChecks runs the repository-side rows for root under harness.
func doctorRepoChecks(root, harness string) []doctorCheck {
	return []doctorCheck{
		checkHooksInstalled(root, harness),
		checkLefthookGates(root),
		checkHarnessHookFiles(root, harness),
		checkStanzaPresent(root),
		checkAgentsSkills(root, harness),
		checkWiringDrift(root, harness),
		checkConfigValid(root),
		checkBindingFile(root),
		checkGuardFile(root),
		checkGuardIncludeModulesUnbound(root),
		checkUserScopeIgnore(root),
	}
}

// doctorJSONPayload is `fathom doctor --json`'s document — printDoctorJSON
// writes it and the wiring tab carries it as the verb's section (P10).
func doctorJSONPayload(harness, svc, root string, checks []doctorCheck) map[string]any {
	pass, warn, fail, skipped := tallyDoctorChecks(checks)
	return map[string]any{
		"service": svc,
		"harness": harness,
		"root":    root,
		"checks":  checks,
		// warn is additive to the pre-existing summary shape (never
		// renamed/removed) — an older consumer reading only pass/fail/
		// skipped still sees the same numbers those fields always meant.
		"summary": map[string]any{"total": len(checks), "pass": pass, "warn": warn, "fail": fail, "skipped": skipped},
	}
}

package capability

import (
	"fmt"
	"strconv"
	"strings"
)

// count renders "1 rule" / "6 rules" with thousands separators. Detection
// evidence is read by people, so "1 rule sources" and "11839 bytes" are
// defects, not cosmetics.
func count(n int, word string) string {
	s := commaInt(n) + " " + word
	if n != 1 {
		s += "s"
	}
	return s
}

func commaInt(n int) string {
	s := strconv.Itoa(n)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	var parts []string
	for len(s) > 3 {
		parts = append([]string{s[len(s)-3:]}, parts...)
		s = s[:len(s)-3]
	}
	parts = append([]string{s}, parts...)
	out := strings.Join(parts, ",")
	if neg {
		out = "-" + out
	}
	return out
}

// Default is fathom's registry. Every entry here is something a user can
// switch off, and every entry can say why it is off.
//
// Adding one: pick the seam it belongs to (§3 of the design refuses a
// fifth), write Detect over already-observed facts only, and write Remedy
// so that a zero Probe still produces either a command or an explicit
// nothing-to-run. TestRegistryEveryEntryExplainsItsAbsence enforces the
// last part.
func Default() Registry {
	caps := []Capability{
		{
			ID:         "intent-corpus",
			Switchable: true,
			Seam:       SeamCore,
			Summary:    "Fathom reads this repository's Project Intent corpus — the router, its living documents and its decision records — so an agent can see which conditions have fired, which invariants nothing enforces, and which claims no instrument proves.",
			Default:    true,
			Scopes:     ScopeAny,
			Cost:       "no per-session cost; `fathom intent audit` pays one index probe per commit, cached after that",
			Detect: func(p Probe) State {
				switch {
				case p.IntentRouterPresent:
					detail := "router present"
					if p.IntentConditions > 0 {
						detail = fmt.Sprintf("%d condition(s) across the corpus", p.IntentConditions)
					}
					return State{Operating: true, Detail: detail}
				case p.IntentDecisionsHome != "":
					// Decision records without a router are a real tier, not
					// a half-adoption: their Revisit-when trailers are the
					// conditions worth listing, and vision 23 promised those
					// repos an audit with no router precondition.
					return State{Operating: true, Detail: "decision records only (no router) — trailers are listed"}
				default:
					return State{Reason: "this repository has no docs/intent/ router and no decision records — nothing to audit (agentic-kit's intent-init bootstraps one)"}
				}
			},
			Remedy: func(p Probe) Remedy {
				if p.IntentRouterPresent || p.IntentDecisionsHome != "" {
					return Remedy{Cmd: "fathom intent audit"}
				}
				// Nothing to run: choosing not to adopt the practice is a
				// choice, not a defect, and a remedy that scolds a repo for
				// it would be the tool overreaching.
				return Remedy{Note: "not adopting Project Intent is a choice; agentic-kit's intent-init bootstraps it if you want it"}
			},
		},
		{
			ID:                   "index",
			Switchable:           false,
			NotSwitchableBecause: "the index is not a per-repository switch — it exists or it does not; `fathom ingest` and `.fathomignore` are the real controls",
			Seam:                 SeamCore,
			Summary:              "Fathom holds a searchable index of this repository's symbols, so an agent can ask where something is defined instead of reading files to find it.",
			Default:              true,
			Scopes:               ScopeAny,
			Cost:                 "no per-session cost; disk, plus ingest time on each commit",
			Detect: func(p Probe) State {
				switch {
				case p.IndexedSymbols == 0:
					return State{Reason: "this repository has no indexed symbols — it is not a fathom service yet"}
				case p.IndexStale:
					reason := "the index is behind the repository's current commit"
					if p.IndexNote != "" {
						reason += " (" + p.IndexNote + ")"
					}
					return State{Reason: reason, Broken: true}
				case !p.IndexFresh:
					// Nobody could determine freshness. That is an absence
					// of information, not a defect — reporting it as one
					// would fail every repository whose daemon is simply
					// unreachable.
					reason := "index freshness could not be determined"
					if p.IndexNote != "" {
						reason += " — " + p.IndexNote
					}
					return State{Reason: reason}
				}
				return State{Operating: true, Detail: count(p.IndexedSymbols, "symbol") + ", fresh at HEAD"}
			},
			Remedy: func(p Probe) Remedy {
				switch {
				case p.IndexedSymbols == 0:
					return Remedy{Cmd: "fathom init"}
				case p.IndexStale:
					return Remedy{Cmd: "fathom ingest --service " + orPlaceholder(p.Service)}
				}
				return Remedy{Note: "nothing to run here — the daemon could not be asked; `fathom doctor` diagnoses reachability"}
			},
		},
		{
			ID:         "obligations",
			Switchable: true,
			Seam:       SeamLayers,
			Summary:    "Before an agent edits a file, fathom puts the rules governing that file in front of it — so a rule is applied while the code is written rather than found in review.",
			Default:    true,
			Scopes:     ScopeProjectOnly,
			Cost:       "one hook round-trip per edit (~15 ms against a local daemon)",
			// Three distinguishable failures, three different remedies:
			// nothing authored, authored but never compiled, compiled but
			// nothing delivers it. Collapsing them into one "off" is how a
			// user ends up running the wrong command twice.
			Detect: func(p Probe) State {
				switch {
				case p.ObligationRules == 0:
					return State{Reason: "no obligation rules are authored for this repository (.fathom/rules/ is empty or absent)"}
				case !p.ObligationsCompiled:
					return State{Reason: "rules are authored but the obligations layer has never been compiled here", Broken: true}
				case !p.HooksReferenced:
					reason := "obligations are compiled but no edit hook is wired to deliver them"
					if p.HooksNote != "" {
						reason += " (" + p.HooksNote + ")"
					}
					return State{Reason: reason, Broken: true}
				}
				return State{Operating: true, Detail: count(p.ObligationRules, "rule source") + " compiled, injected on edit"}
			},
			Remedy: func(p Probe) Remedy {
				switch {
				case p.ObligationRules == 0:
					return Remedy{
						Cmd:  "fathom layers install <pack>",
						Note: "or author .fathom/rules/*.cue directly; kits ship rule packs",
					}
				case !p.ObligationsCompiled:
					return Remedy{Cmd: "fathom compile --service " + orPlaceholder(p.Service)}
				}
				return Remedy{Cmd: "fathom init --hooks"}
			},
		},
		{
			ID:         "guards",
			Switchable: true,
			Seam:       SeamGuards,
			Summary:    "Extra, repository-specific guidance attached to particular paths — shown before an edit or a command touching them, for the files where getting it wrong is expensive.",
			Default:    true,
			Scopes:     ScopeProjectOnly,
			Cost:       "shares the obligations hook round-trip; no additional call",
			Detect: func(p Probe) State {
				if p.GuardProblem != "" {
					return State{Reason: p.GuardProblem, Broken: true}
				}
				if p.GuardRules == 0 {
					return State{Reason: "no guard rules are defined for this repository"}
				}
				if !p.HooksReferenced {
					return State{Reason: "guard rules exist but no edit hook is wired to deliver them", Broken: true}
				}
				return State{Operating: true, Detail: count(p.GuardRules, "guard rule") + " active"}
			},
			Remedy: func(p Probe) Remedy {
				if p.GuardProblem != "" {
					return Remedy{Cmd: "fathom doctor", Note: "repair .fathom/guard.cue before enabling guards"}
				}
				if p.GuardRules == 0 {
					return Remedy{
						Cmd:  "fathom init --guards",
						Note: "writes a commented .fathom/guard.cue scaffold; a kit pack may also ship rules (fathom layers install)",
					}
				}
				return Remedy{Cmd: "fathom init --hooks"}
			},
		},
		{
			ID:                   "modules",
			Switchable:           false,
			NotSwitchableBecause: "a module is on when a binding names it; .fathom/modules.cue is the real control",
			Seam:                 SeamModules,
			Summary:              "Published expert modules — curated domain knowledge from outside this repository — answerable from inside it.",
			Default:              true,
			Scopes:               ScopeAny,
			Cost:                 "no standing cost; tokens only when a module is queried",
			Detect: func(p Probe) State {
				if p.ModuleBindings == 0 {
					return State{Reason: "no expert module is bound to this repository"}
				}
				return State{Operating: true, Detail: count(p.ModuleBindings, "module") + " bound"}
			},
			Remedy: func(Probe) Remedy {
				return Remedy{Note: "nothing to run until a module is published that this repository should consume; bindings live in .fathom/modules.cue"}
			},
		},
		{
			ID:                   "layers",
			Switchable:           false,
			NotSwitchableBecause: "a layer pack is on when it is installed; `fathom layers install` and the layers directory are the real controls",
			Seam:                 SeamLayers,
			Summary:              "Semantic layer packs — reusable sets of claims about a technology or house style, compiled over this repository's corpus.",
			Default:              true,
			Scopes:               ScopeAny,
			Cost:                 "compile time on install; no per-session cost",
			Detect: func(p Probe) State {
				if len(p.LayerPacks) == 0 {
					return State{Reason: "no layer packs are installed in this repository"}
				}
				return State{Operating: true, Detail: count(len(p.LayerPacks), "pack") + " installed"}
			},
			Remedy: func(Probe) Remedy {
				return Remedy{Cmd: "fathom layers install <pack>", Note: "kits ship packs; `fathom layers list` shows what is available"}
			},
		},
		{
			ID:         "verify-gate",
			Switchable: true,
			Seam:       SeamCore,
			Summary:    "Before a push, fathom re-checks every \"done\" box in this repository's plans against the code, and fails when the code does not back the claim.",
			Default:    true,
			Scopes:     ScopeProjectOnly,
			Cost:       "a few seconds on pre-push",
			Detect: func(p Probe) State {
				if !p.VerifyInLefthook {
					return State{Reason: "no pre-push hook runs fathom verify, so plan claims are never re-checked"}
				}
				return State{Operating: true, Detail: "runs on pre-push"}
			},
			Remedy: func(Probe) Remedy {
				return Remedy{Cmd: "fathom init --hooks", Note: "adds only fathom's own entry; existing lefthook commands are preserved"}
			},
		},
		{
			ID:                   "agent-surface",
			Switchable:           false,
			NotSwitchableBecause: "the surface is decided by which endpoint your MCP client is registered against, not by this file",
			Seam:                 SeamProfile,
			Summary:              "The curated set of tools an AI coding agent sees. A narrower surface costs less context; a wider one can answer more.",
			Default:              true,
			Scopes:               ScopeAny,
			Cost:                 "tool schema carried by every connected session before any work",
			Detect: func(p Probe) State {
				if p.AgentSurfaceTools == 0 {
					// NOT Broken: doctor's own daemon_reachable and
					// mcp_registration rows own this diagnosis. One root
					// cause producing three red marks makes the failure
					// count meaningless, so this row defers and says
					// where the answer lives.
					return State{Reason: "no agent surface is reachable — see the daemon and MCP registration checks, which diagnose this directly"}
				}
				d := count(p.AgentSurfaceTools, "tool")
				if p.AgentSchemaBytes > 0 {
					d += ", " + count(p.AgentSchemaBytes, "byte") + " of schema per session"
				}
				return State{Operating: true, Detail: d}
			},
			Remedy: func(Probe) Remedy {
				return Remedy{Cmd: "fathom doctor", Note: "registration and daemon reachability are diagnosed there"}
			},
		},
		{
			ID:                   "session-ledger",
			Switchable:           false,
			NotSwitchableBecause: "measurement is not silently disableable from a config file (law 3); FATHOM_CHANNELS=off is the documented, doctor-visible kill switch",
			Seam:                 SeamCore,
			Summary:              "Records per-session counts — sessions, discovery actions, gate runs — so fathom can report what it did rather than assert it.",
			Default:              true,
			Scopes:               ScopeAny,
			Cost:                 "one transcript parse at session end",
			Detect: func(p Probe) State {
				if p.LedgerSessions == 0 {
					return State{Reason: "no sessions have been recorded yet"}
				}
				return State{Operating: true, Detail: count(p.LedgerSessions, "session") + " recorded"}
			},
			Remedy: func(Probe) Remedy {
				return Remedy{Note: "nothing to run — the ledger fills as sessions complete; a repository with no coding sessions is idle, not broken"}
			},
		},
		{
			ID:                   "narrate",
			Switchable:           false,
			NotSwitchableBecause: "narration is a bounded window, not a standing setting; `fathom narrate on --for 7d` is the real control and it self-expires",
			Seam:                 SeamCore,
			Summary:              "Captures short stories of what happened in a session alongside the counts. Off by default; it is a lens, never evidence (law 14).",
			Default:              false,
			Scopes:               ScopeAny,
			Cost:                 "a story row per session",
			Detect: func(p Probe) State {
				if !p.NarrateActive {
					return State{Reason: "the narration window is not armed"}
				}
				return State{Operating: true, Detail: "armed"}
			},
			Remedy: func(Probe) Remedy {
				return Remedy{Cmd: "fathom narrate on --for 7d", Note: "self-expiring by design — narration is for a question you are asking now"}
			},
		},
		{
			ID:         "assess",
			Switchable: true,
			Seam:       SeamCore,
			Summary:    "An explicitly invoked, priced second opinion: a model judges a session's on-disk work products (diffs, plan deltas, gate results), never its transcript, as typed questions. Off by default; every run reports its own token cost.",
			Default:    false,
			// Law 13: a user may default the lane on for repositories that
			// have not spoken; a repository may govern it either way.
			Scopes: ScopeAny,
			Cost:   "model tokens per explicit `fathom assess` run, reported in its output and on the assess ledger; nothing when not invoked",
			Detect: func(p Probe) State {
				if p.AssessRuns == 0 {
					return State{Reason: "no assess run has been recorded for this repository — the lane runs only when explicitly invoked"}
				}
				return State{Operating: true, Detail: count(p.AssessRuns, "assess run") + " recorded on the assess ledger"}
			},
			Remedy: func(Probe) Remedy {
				return Remedy{
					Cmd:  "fathom assess --dry-run",
					Note: "the dry run shows exactly what would be sent and its estimated tokens, and spends nothing; a live run also needs a [decide] provider",
				}
			},
		},
	}

	byID := make(map[string]Capability, len(caps))
	for _, c := range caps {
		byID[c.ID] = c
	}
	return Registry{byID: byID}
}

// orPlaceholder keeps a remedy command copy-pasteable when the service is
// not yet known: "<service>" is obviously a hole, "" silently produces a
// broken command.
func orPlaceholder(s string) string {
	if s == "" {
		return "<service>"
	}
	return s
}

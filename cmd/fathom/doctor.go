package main

// doctor.go — DI-1 (docs/plans/doctor-and-impact.md): `fathom doctor` is the
// deterministic wiring-diagnosis checklist for a fathom-adopting repo. It
// answers "is fathom actually wired here, end to end?" with one command
// instead of the manual, multi-doc "you know it works when…" checklists in
// docs/adoption/claude-code-internals.md §14 and docs/adoption/codex.md §7.
//
// Every check follows the C4-5 honesty discipline (docs/plans/review-verb.md):
// uncertain wiring and expired probes are unknown/timeout, never a pass or
// a claimed failure. Other uncheckable states (unreadable config, unresolvable
// home directory) is reported `skipped` with a reason and, where possible, a
// manual instruction — NEVER a guessed `pass`. A real, observable problem is
// `fail` with a one-command `remedy`. doctor is strictly read-only: it never
// writes a file, calls a mutating MCP tool, or restarts anything.
//
// Where a check can reuse an existing surface instead of duplicating logic,
// it does: config.Load for CUE validation, the `obligations`/`server_stats`
// MCP tools over the same HTTP plumbing verify.go/init.go already use,
// resolveCallLogPath/readCallLog from instrument_adoption.go (same package,
// no import needed), and fathomHookSpecs/fathomStanzaBegin/fathomStanzaEnd/
// skillMarker from init.go/skillsinstall.go (read-only reuse of their
// detection conventions, never their write paths).
import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	skillsdist "github.com/geoffmilleraz/fathom/dist/agents/skills"
	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/harness"
	"github.com/geoffmilleraz/fathom/internal/ledgerroots"
	"github.com/geoffmilleraz/fathom/internal/projectdir"
	"github.com/geoffmilleraz/fathom/internal/semantic"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
	heartbeat "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/telemetry/heartbeat"
	toml "github.com/pelletier/go-toml/v2"
)

// doctorStatus is one check's verdict.
type doctorStatus string

const (
	doctorPass    doctorStatus = "pass"
	doctorUnknown doctorStatus = "unknown"
	doctorTimeout doctorStatus = "timeout"
	doctorFail    doctorStatus = "fail"
	doctorSkipped doctorStatus = "skipped"
	// doctorWarn is phase O's addition (docs/design/platform-modules.md
	// §4 deliverable 4 / guarded-edits.md §2.4): a real, observable
	// condition worth a human's attention (a dead guard, a sub-servable
	// bound module) that must NEVER fail the gate — a config-quality
	// signal, not a wiring-broken verdict. doctorExitCode treats it
	// exactly like doctorPass for exit-code purposes; it is visually and
	// machine-distinguishable (statusLabel/JSON) so it is never silently
	// indistinguishable from a real pass either.
	doctorWarn doctorStatus = "warn"
)

// doctorCheck is one row of the checklist: {name, status, reason, remedy}
// per the plan's design section. Remedy is always a single command (or
// short instruction), never a paragraph — it is omitted entirely on pass.
type doctorCheck struct {
	Name     string       `json:"name"`
	Status   doctorStatus `json:"status"`
	Reason   string       `json:"reason"`
	Remedy   string       `json:"remedy,omitempty"`
	probeErr error        // underlying probe error, never inferred from elapsed context alone
}

// cmdDoctor implements the "doctor" subcommand.
//
// Exit codes:
//
//	0 — no confirmed failure (skipped/unknown/timeout do not fail the gate; they are
//	    honestly-uncheckable states, not failures)
//	1 — at least one check failed
//	2 — doctor itself could not run (bad flags, unresolvable --path/cwd) —
//	    an operational error, not a verdict on the repo's wiring
func cmdDoctor(args []string, stdout, stderr io.Writer) int {
	return cmdDoctorWithClock(args, stdout, stderr, time.Now)
}

func cmdDoctorWithClock(args []string, stdout, stderr io.Writer, now func() time.Time) int {
	fs := newFlagSet("doctor", stderr)
	path := fs.String("path", "", "repo root to diagnose (default: cwd)")
	harnessFlag := fs.String("harness", "auto", "claude | codex | auto, or a registry id (claude-code, codex, gemini-cli, agents — 'fathom harness list') — which harness's wiring to check")
	service := fs.String("service", "", "service ID (default: resolved from --path, the same identity the hooks use)")
	url := fs.String("url", defaultFathomURL(), "fathom serve-http base URL")
	jsonOut := fs.Bool("json", false, "emit JSON instead of a human table")
	brief := fs.Bool("brief", false, "FAIL, TIMEOUT and UNKNOWN rows, at most 5 findings, exit 0 always — the payload of the harness Setup event (doctor_brief.go)")
	repoOnly := fs.Bool("repo-only", false, "only the checks that read the repository's own files — hooks, gates, stanza, skills, wiring drift, .fathom config (no daemon, harness process or ledger); the Observatory's project wiring tab renders it (doctor_repoonly.go)")
	stranger := fs.Bool("stranger", false, "time install to first proven answer against the 5-minute bar via scripts/stranger-check.sh (fathom's own repo only; skipped elsewhere) — slow (up to 10 minutes), ignored with --repo-only")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "error: unexpected argument %q\n", fs.Arg(0))
		return 2
	}
	if _, ok := normalizeDoctorHarness(*harnessFlag); !ok {
		fmt.Fprintf(stderr, "error: --harness must be claude, codex, auto, or a registry id (%s) (got %q)\n", strings.Join(harness.IDs(), ", "), *harnessFlag)
		return 2
	}

	root := *path
	if root == "" {
		wd, err := os.Getwd()
		if err != nil {
			fmt.Fprintf(stderr, "error: resolving cwd: %v\n", err)
			return 2
		}
		root = wd
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		fmt.Fprintf(stderr, "error: resolving --path %q: %v\n", root, err)
		return 2
	}
	root = absRoot

	// K5's second half (EP-24): doctor is one of the two instruments a
	// session can reach for after seeing the attention line. Law 7 — an
	// instrument that leaves no trace cannot be shown to have been used.
	rpcOrigin = "doctor"
	if *brief {
		// --brief is the harness Setup hook (HP-31): one deadline, the
		// breaker, and silence under the kill switch, like every hook.
		endGuard, silent := beginHookGuard("doctor", "fathom doctor --brief", "", *url)
		if silent {
			return 0
		}
		defer endGuard()
	}
	defer emitBeat("doctor", "run", nil) // report reads the span before this invocation

	// svc here is display identity only (the table header). The
	// obligations_served check does its own candidate resolution against
	// the daemon's actual service list — the same all-candidates probing
	// the hooks perform (edithook.go's candidate loop) — so the header's
	// first-candidate guess can never silently misdirect a remedy.
	explicitSvc := strings.ToLower(*service)
	svc := explicitSvc
	if svc == "" {
		ctx, cancel := doctorProbeContext(context.Background())
		cands := serviceCandidates(root, ctx)
		cancel()
		svc = strings.ToLower(cands[0].name)
	}

	harness := resolveDoctorHarness(*harnessFlag, root)
	var checks []doctorCheck
	if *repoOnly {
		checks = doctorRepoChecks(root, harness)
	} else {
		checks = runDoctorChecks(root, explicitSvc, harness, *url)
		if *stranger {
			checks = append(checks, doctorStrangerRow(root)) // SR-53 (F-51, G16): opt-in, slow — never part of the default run
		}
	}

	if *brief {
		// Exit 0 whatever the findings: --brief runs as a hook.
		return printDoctorBrief(stdout, checks)
	}
	channels := doctorChannels(root, harness, now())
	if *jsonOut {
		printDoctorJSON(stdout, harness, svc, root, checks, channels)
	} else {
		printDoctorTable(stdout, harness, svc, root, checks)
		printDoctorChannels(stdout, channels)
	}

	return doctorExitCode(checks)
}

// doctorHarnessLabel maps a registry id to doctor's own harness label: the
// pre-seam vocabulary ("claude", "codex") stays the label every check
// switches on and every report prints; newer descriptors are labelled by
// their registry id.
func doctorHarnessLabel(id string) string {
	if id == harness.ClaudeCode {
		return "claude"
	}
	return id
}

// doctorHarnessID is the inverse: doctor's label back to the registry id.
func doctorHarnessID(label string) string {
	if label == "claude" {
		return harness.ClaudeCode
	}
	return label
}

// normalizeDoctorHarness accepts doctor's historical vocabulary (claude |
// codex | auto) plus every registry id (HP-12), returning doctor's label.
func normalizeDoctorHarness(flagVal string) (string, bool) {
	switch flagVal {
	case "claude", "codex", "auto":
		return flagVal, true
	}
	if _, ok := harness.Lookup(flagVal); ok {
		return doctorHarnessLabel(flagVal), true
	}
	return "", false
}

// resolveDoctorHarness resolves "auto" to a concrete harness through the
// registry's presence markers (harness.Detect, in registry order — the
// verified primary first); an explicit id always wins outright. Detection
// never errors — an unresolvable auto default falls back to "claude" (the
// primary, dogfooded path), same spirit as init's default service-name
// resolution never refusing to run.
func resolveDoctorHarness(flagVal, root string) string {
	if label, ok := normalizeDoctorHarness(flagVal); ok && label != "auto" {
		return label
	}
	if ids := harness.Detect(root, harnessHomeDir()); len(ids) > 0 {
		return doctorHarnessLabel(ids[0])
	}
	return "claude"
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// runDoctorChecks runs every check in a fixed, deterministic order (rough
// dependency order: registration/reachability first, then the
// injection/gate stack, then serving state) and returns every row —
// doctor never short-circuits on an early failure, so one bad check never
// hides the rest of the diagnosis.
func runDoctorChecks(root, explicitSvc, harness, baseURL string) []doctorCheck {
	// Prepare shared daemon facts once, then run independent rows concurrently.
	// Declaration order remains the report order; each unfinished probe is named.
	var (
		daemonUp      bool
		tenancyRep    resourcesReport
		daemonMetrics map[string]float64
	)
	var roots ledgerroots.Roots
	var once sync.Once
	ledgerRoots := func(ctx context.Context) ledgerroots.Roots {
		once.Do(func() { roots = doctorLedgerRoots(doctorHTTPClient(ctx), baseURL) })
		return roots
	} // PE-24/PE-25
	steps := []doctorStep{
		{name: "mcp_registration", probe: func(ctx context.Context) doctorCheck { return checkMCPRegistration(root, harness, baseURL, ctx) }},
		{name: "daemon_reachable", prepare: true, probe: func(ctx context.Context) doctorCheck {
			up, daemonCheck := checkDaemonReachable(doctorHTTPClient(ctx), baseURL)
			daemonUp = up
			if hookDeadlineSpent() {
				return daemonCheck // every later row is cut; skip their inputs too
			}
			// RT-49/RT-50 (§13.3): one resources projection feeds every
			// tenancy row — live when the daemon answers, else its ring and
			// ledger.
			tenancyRep = doctorResourcesReport(ctx, baseURL)
			if daemonUp {
				daemonMetrics, _ = fetchMetricsText(doctorHTTPClient(ctx), baseURL)
			}
			return daemonCheck
		}},
		{name: "binary_fresh", probe: func(ctx context.Context) doctorCheck { return checkBinaryFresh(root, ctx) }},
		{name: "daemon_fresh", probe: func(ctx context.Context) doctorCheck {
			return checkDaemonFresh(doctorHTTPClient(ctx), baseURL, daemonUp)
		}}, // PC-4: the daemon's stamp vs this binary (doctor_provenance.go)
		{name: "daemon_footprint", probe: func(ctx context.Context) doctorCheck { return checkDaemonFootprint(tenancyRep, daemonMetrics) }}, // RT-49: phys_footprint vs the budget, live or from the ring (doctor_footprint.go)
		{name: "disk_writes", probe: func(ctx context.Context) doctorCheck {
			return checkDaemonDiskWrites(doctorHTTPClient(ctx), baseURL, daemonUp)
		}}, // RT-162: bytes written outside explicit ingest vs 2 GiB/day (doctor_footprint.go)
		{name: "admission_mode", probe: func(ctx context.Context) doctorCheck {
			return checkAdmissionMode(doctorHTTPClient(ctx), baseURL, daemonUp)
		}}, // design §1: WARN while the daemon serves with admission off (doctor_admission.go)
		{name: "daemon_posture", probe: func(ctx context.Context) doctorCheck { return checkDaemonPosture("") }},                            // RT-15: the installed plist's launchd posture vs schema 2 (daemon_signing.go)
		{name: "daemon_lifecycle", probe: func(ctx context.Context) doctorCheck { return checkDaemonLifecycle(tenancyRep) }},                // RT-50: starts, exits, breaker, quarantine (doctor_tenancy.go)
		{name: "host_pressure", probe: func(ctx context.Context) doctorCheck { return checkHostPressure(tenancyRep) }},                      // RT-50: pressure, swap, swap-volume free
		{name: "scratch_daemons", probe: func(ctx context.Context) doctorCheck { return checkScratchDaemons(tenancyRep) }},                  // RT-50: leaked unmanaged daemons and stale records
		{name: "hook_budget", probe: func(ctx context.Context) doctorCheck { return checkHookBudgetFromFiles(time.Now()) }},                 // RT-135: hook p99 vs deadline, breaker-open minutes, unguarded edits, kills (doctor_retention.go)
		{name: "rule_outcome_writer", probe: func(ctx context.Context) doctorCheck { return checkRuleOutcomeWriterFromFiles(time.Now()) }},  // GL-24 (INC-27): a rule-outcome log silent 72 h while the hook served claims (doctor_outcomewriter.go)
		{name: "disk_retention", probe: func(ctx context.Context) doctorCheck { return checkDiskRetentionFromDisk(root) }, ownBudget: true}, // RT-135: reclaimable bytes by class, internal disk free (doctor_retention.go)
		{name: "tenancy_scope", probe: func(ctx context.Context) doctorCheck {
			return checkTenancyScope(root, config.DefaultUserPath(), os.Getenv)
		}}, // RT-50: [tenancy] is user scope only
		{name: "key_source_scope", probe: func(ctx context.Context) doctorCheck { return checkKeyScope(root, config.DefaultUserPath()) }}, // SEC-1: key sources are user scope only (doctor_keyscope.go)
		{name: "hooks_installed", probe: func(ctx context.Context) doctorCheck { return checkHooksInstalledContext(ctx, root, harness) }},
		{name: "duplicate_hook_registration", probe: func(ctx context.Context) doctorCheck { return checkDuplicateHookRegistration(root, harness, ctx) }},
		{name: "lefthook_gates", probe: func(ctx context.Context) doctorCheck { return checkLefthookGates(root) }},
		{name: "hooks_firing", probe: func(ctx context.Context) doctorCheck {
			return checkHooksFiring(doctorHTTPClient(ctx), baseURL, daemonUp, root, harness)
		}},
		{name: "gate_posture", probe: func(ctx context.Context) doctorCheck { return checkGatePosture(harness) }},
		{name: "hooks_coverage", probe: func(ctx context.Context) doctorCheck { return checkHooksCoverage(harness) }},              // HP-32: what the harness offers vs what fathom wires (advisory, never FAIL)
		{name: "harness_hooks", probe: func(ctx context.Context) doctorCheck { return checkHarnessHookFiles(root, harness, ctx) }}, // HP-18/19: doctor_harness.go, never PASS
		{name: "stanza_present", probe: func(ctx context.Context) doctorCheck { return checkStanzaPresent(root) }},
		{name: "stanza_loaded", probe: func(ctx context.Context) doctorCheck { return checkStanzaLoaded(root) }},             // PC-25: the harness-side twin of stanza_present (doctor_stanza_loaded.go)
		{name: "skills_installed", probe: func(ctx context.Context) doctorCheck { return checkAgentsSkills(root, harness) }}, // RP-3: skills_installed on the standard (doctor_skills.go)
		{name: "wiring_drift", probe: func(ctx context.Context) doctorCheck { return checkWiringDrift(root, harness) }},      // PC-29: hooks, stanzas, MCP entries, owned skills (doctor_wiring.go)
		{name: "config_valid", probe: func(ctx context.Context) doctorCheck { return checkConfigValid(root) }},
		{name: "decision_provider", probe: func(ctx context.Context) doctorCheck {
			return checkDecisionProvider(root, doctorProfileBuilder(ctx), ctx)
		}, ownBudget: true}, // DP-4 (ADR-042): is the configured [decide] provider viable? (doctor_decide.go)
		{name: "profile_bins", probe: func(ctx context.Context) doctorCheck { return checkProfileBins(root) }}, // GL-27 (G6): a profile whose bin no longer exists (doctor_compile.go)
		{name: "compile_layer", probe: func(ctx context.Context) doctorCheck {
			return checkCompileLayerEnabled(doctorHTTPClient(ctx), baseURL, daemonUp)
		}}, // GL-27 (G6): the daemon started with compile_layer disabled (doctor_compile.go)
		{name: "layers_stale", probe: func(ctx context.Context) doctorCheck {
			return checkStaleLayers(doctorHTTPClient(ctx), baseURL, root, explicitSvc, daemonUp)
		}}, // GL-27 (G6): sections compiled vs total per service, --dry-run first (doctor_compile.go)
		{name: "providers", probe: func(ctx context.Context) doctorCheck {
			return doctorProvidersRow(root, doctorProfileBuilder(ctx), ctx)
		}, ownBudget: true}, // SR-32 (surface reckoning F-71): CF-5's own auth probe, folded in (doctor_providers.go); CF-6 (smoke) never runs here
		{name: "obligations_served", probe: func(ctx context.Context) doctorCheck {
			return checkObligationsServed(doctorHTTPClient(ctx), baseURL, root, explicitSvc, daemonUp, ctx)
		}, ownBudget: true},
		{name: "surface_traffic", probe: func(ctx context.Context) doctorCheck {
			return checkSurfaceTraffic(doctorHTTPClient(ctx), baseURL, daemonUp, root)
		}},
		// Phase O (docs/design/platform-modules.md §4 / guarded-edits.md
		// §2.4): config-validity first, then the checks that need it.
		{name: "binding_file", probe: func(ctx context.Context) doctorCheck { return checkBindingFile(root) }},
		{name: "guard_file", probe: func(ctx context.Context) doctorCheck { return checkGuardFile(root) }},
		{name: "project_keys", probe: func(ctx context.Context) doctorCheck { return checkProjectKeys(root) }}, // every .fathom/*.cue key: file, status, entries (doctor_project.go)
		{name: "tech_packs", probe: func(ctx context.Context) doctorCheck { return checkTechPacks(root) }},     // FD-21: discovered tech packs, pins, served and withheld sections, exclusions (doctor_techpacks.go)
		{name: "legacy_files", probe: func(ctx context.Context) doctorCheck { return checkLegacyFiles(root) }}, // old copies of moved or shadowed files (doctor_legacy.go)
		{name: "binding_modules_mounted", probe: func(ctx context.Context) doctorCheck {
			return checkBindingModulesMounted(doctorHTTPClient(ctx), baseURL, root, daemonUp)
		}},
		{name: "binding_module_readiness", probe: func(ctx context.Context) doctorCheck {
			return checkBindingModuleReadiness(doctorHTTPClient(ctx), baseURL, root, daemonUp)
		}},
		{name: "guard_dead_scopes", probe: func(ctx context.Context) doctorCheck {
			return checkGuardDeadScopes(doctorHTTPClient(ctx), baseURL, root, explicitSvc, daemonUp, ctx)
		}},
		{name: "guard_include_modules_unbound", probe: func(ctx context.Context) doctorCheck { return checkGuardIncludeModulesUnbound(root) }},
		// Phase P (docs/design/production-tokenomics.md / scoped-config.md):
		// env-driven honesty checks first, then scoped-config disclosure.
		{name: "channels_env", probe: func(ctx context.Context) doctorCheck { return checkChannelsEnv() }},
		{name: "binary_on_path", probe: func(ctx context.Context) doctorCheck { return checkBinaryOnPath() }},
		{name: "ident_cache_env", probe: func(ctx context.Context) doctorCheck { return checkIdentCacheEnv() }},
		{name: "user_scope_ignore", probe: func(ctx context.Context) doctorCheck { return checkUserScopeIgnoreContext(ctx, root) }},
		{name: "user_scope_guard", probe: func(ctx context.Context) doctorCheck { return checkUserScopeGuard() }},
		{name: "session_ledger_tokens", probe: func(ctx context.Context) doctorCheck { return checkSessionLedgerTokens() }},
		// Phase Q (docs/design/observability-surfaces.md §8 item 3).
		{name: "metrics_tier", probe: func(ctx context.Context) doctorCheck { return checkMetricsTier() }},
		// Phase R (docs/plans/cpp-toolchain.md CT-7).
		{name: "cpp_toolchain", probe: func(ctx context.Context) doctorCheck { return checkCppToolchain(root) }},
		// BP-36 (docs/plans/bench-pyramid.md): is the graph this daemon
		// serves actually typed? (doctor_typed.go)
		{name: "typed_edges", probe: func(ctx context.Context) doctorCheck {
			return checkTypedEdges(doctorHTTPClient(ctx), baseURL, root, explicitSvc, daemonUp)
		}},
		// O69 (wave P1): does this repo have source in a language fathom has
		// no extractor for — named and counted, never silent (doctor_languages.go)
		{name: "unsupported_languages", probe: func(ctx context.Context) doctorCheck {
			return checkUnsupportedLanguages(doctorHTTPClient(ctx), baseURL, root, explicitSvc, daemonUp)
		}},
		// EW-20 (O127): a registered service whose root no longer exists,
		// named with the ingest call that re-points it (doctor_serviceroots.go)
		{name: "service_roots", probe: func(ctx context.Context) doctorCheck {
			return checkServiceRoots(doctorHTTPClient(ctx), baseURL, root, explicitSvc, daemonUp)
		}},
		// Phase T (docs/design/capability-registry.md): the one row that
		// answers "why isn't X happening here?" — last, because it reads
		// every fact the rows above diagnose individually.
		{name: "capabilities", probe: func(ctx context.Context) doctorCheck {
			return checkCapabilities(baseURL, root, explicitSvc, ctx)
		}, ownBudget: true},
		// CC-6 (docs/plans/custodian.md): the config custodian — are this
		// repo's registered config contracts still true?
		{name: "concerns", probe: func(ctx context.Context) doctorCheck { return checkConcerns(root) }},
		{name: "intent", probe: func(ctx context.Context) doctorCheck { return checkIntent(root, ledgerRoots(ctx)) }},
		{name: "readouts", probe: func(ctx context.Context) doctorCheck { return checkReadouts(root) }}, // RO-18 (ADR-071): readouts mode, refused rules, view support, another Bash rewriter (doctor_readouts.go)
		{name: "unproven_claims", probe: func(ctx context.Context) doctorCheck {
			return checkUnprovenClaims(root, baseURL, explicitSvc, ledgerRoots(ctx))
		}},
		// DP-3/DP-4 (docs/plans/custodian.md): the doctrine plane — the
		// vision promotion pipeline and goals as pre-registered bets
		// (intentdoctrine.go). Warns, never fails.
		{name: "doctrine", probe: func(ctx context.Context) doctorCheck { return checkDoctrine(root) }},
		// vision 33 / ADR-043 stage 1 (TC-14): the certifications layer's
		// own declared/live/join-rate facts, read directly from the
		// repo's local store (doctor's only store-direct row — see
		// doctor_certifications.go's own doc comment for why).
		{name: "certifications", probe: func(ctx context.Context) doctorCheck {
			return checkCertifications(certificationsStorePath(root), certificationsDoctorService(root, explicitSvc), ctx)
		}},
		// J5 (app-kit O25): instrument --adoption/research report/flywheel
		// must agree on which call log they're reading.
		{name: "instrument_identity", probe: func(ctx context.Context) doctorCheck {
			return checkInstrumentIdentity(doctorHTTPClient(ctx), baseURL, explicitSvc)
		}},
	}
	// Preserve the existing fail-open contract when doctor is entered after
	// the caller has already spent the entire invocation budget. Once started,
	// every individual unfinished check is returned by name as TIMEOUT.
	if hookDeadlineSpent() {
		return []doctorCheck{{Name: doctorDeadlineRow, Status: doctorSkipped, Reason: fmt.Sprintf("%d check(s) not run — the hook deadline was already spent before diagnosis", len(steps)), Remedy: "run `fathom doctor` for the full table"}}
	}
	return runDoctorSteps(steps)
}

// doctorDeadlineRow names the row a hook's spent deadline leaves in place
// of the checks it cut (RT-7).
const doctorDeadlineRow = "hook_deadline"

// Individual IO probes keep their own caps. Only a parent deadline permits
// abandoning a check; interactive local projections run to completion.
type doctorStep struct {
	name      string
	probe     func(context.Context) doctorCheck
	ownBudget bool
	prepare   bool // gathers immutable inputs consumed by the independent checks
}

func runDoctorSteps(steps []doctorStep) []doctorCheck {
	ctx, cancel := doctorStepContext(context.Background())
	defer cancel()
	return runDoctorStepsContext(ctx, steps)
}

// Preparation finishes before readers start. Independent checks run together,
// but results retain declaration order regardless of completion order.
func runDoctorStepsContext(parent context.Context, steps []doctorStep) []doctorCheck {
	checks := make([]doctorCheck, len(steps))
	run := func(i int) {
		step := steps[i]
		var ctx context.Context
		var cancel context.CancelFunc
		if at, ok := parent.Deadline(); ok {
			remaining := time.Until(at)
			// Leave part of the existing invocation budget for collection/rendering.
			cap := remaining * 3 / 4
			if !step.ownBudget && cap > 5*time.Second {
				cap = 5 * time.Second
			}
			ctx, cancel = context.WithTimeout(parent, cap)
		} else {
			ctx, cancel = context.WithCancel(parent)
		}
		defer cancel()
		timeout := func() doctorCheck {
			return doctorCheck{Name: step.name, Status: doctorErrorStatus(ctx.Err()), Reason: "check did not finish within its remaining deadline", Remedy: "re-run doctor for this named check"}
		}
		if step.name != "" && ctx.Err() != nil {
			checks[i] = timeout()
			return
		}
		// Preparations write shared inputs; join them before releasing readers.
		_, bounded := parent.Deadline()
		if step.prepare || step.name == "" || !bounded {
			checks[i] = runDoctorProbe(ctx, step.probe)
			return
		}
		// Let a cooperative probe return its source-specific row after its
		// context expires, while collection still respects the hook deadline.
		at, _ := parent.Deadline()
		collect, finish := context.WithDeadline(context.WithoutCancel(ctx), at)
		defer finish()
		stop := context.AfterFunc(ctx, func() { time.AfterFunc(25*time.Millisecond, finish) })
		defer stop()
		row, err := tenancy.RunUntilDone(collect, func() doctorCheck { return runDoctorProbe(ctx, step.probe) })
		if err != nil {
			checks[i] = timeout()
		} else {
			checks[i] = row
		}
	}
	for i, step := range steps {
		if step.prepare {
			run(i)
		}
	}
	var wg sync.WaitGroup
	for i, step := range steps {
		if !step.prepare {
			wg.Go(func() { run(i) })
		}
	}
	wg.Wait()
	return checks
}

// doctorExitCode implements the 0/1 half of the exit contract (2 is
// returned earlier, before any check runs, for a doctor-itself failure).
func doctorExitCode(checks []doctorCheck) int {
	for _, c := range checks {
		if c.Status == doctorFail {
			return 1
		}
	}
	return 0
}

// ─── 1. MCP registration ───────────────────────────────────────────────────

// checkMCPRegistration confirms the harness's own MCP client config points
// fathom at /rpc/agent (or its /rpc/companion alias) — never guessed: an
// unreadable or unparsable config is `skipped` with a manual fallback
// instruction, not a clean pass.
func checkMCPRegistration(root, harnessLabel, baseURL string, parents ...context.Context) doctorCheck {
	ctx, cancel := doctorContextArg(parents)
	defer cancel()
	switch harnessLabel {
	case "claude":
		if c, found := checkProjectMCPRegistration(ctx, root); found {
			return c
		}
		return checkClaudeMCPRegistration(ctx, baseURL, root)
	case "codex":
		// The TOML read is real evidence (a missing entry is a real FAIL,
		// an unreadable file is named — ADR-027), but a found entry is not
		// a verified registration: no Codex session has negotiated the
		// surface on this machine, so a PASS here would be a guess
		// (HP-12: never a guessed PASS). It reads `skipped: unverified`.
		c := checkCodexMCPRegistration(root, ctx)
		if d, ok := harness.Lookup(harness.Codex); ok && !d.Unverified() {
			// HP-34: a real headless session HAS negotiated the surface here
			// (it called list_services over /rpc/agent), so the found entry
			// is evidence of a registration that works, not of one that
			// might. The demotion below exists for the unverified case and
			// must not outlive it — a permanent "skipped" would make the
			// verified state unreachable, which is the circular gate this
			// wave broke.
			return c
		}
		if c.Status == doctorPass {
			c.Status = doctorSkipped
			c.Reason = "skipped: unverified — " + c.Reason + "; the descriptor is unverified on this machine (no Codex session has negotiated /rpc/agent here)"
		}
		return c
	default:
		const name = "mcp_registration"
		d, ok := harness.Lookup(doctorHarnessID(harnessLabel))
		if !ok {
			return doctorCheck{Name: name, Status: doctorSkipped, Reason: fmt.Sprintf("skipped: unverified — no registry descriptor for %q", harnessLabel)}
		}
		return doctorCheck{
			Name: name, Status: doctorSkipped,
			Reason: fmt.Sprintf("skipped: unverified — this build has no %s MCP-registration reader and the descriptor is unverified on this machine", d.Display),
			Remedy: d.MCPAddLine(baseURL),
		}
	}
}

// claudeUserSettingsFile is Claude Code's user-scope settings.json under the
// per-user config root (harness.ClaudeConfigRoot, which honours
// CLAUDE_CONFIG_DIR); "" when no root resolves.
func claudeUserSettingsFile() string {
	cfg := harness.ClaudeConfigRoot(wiringDriftHome())
	if cfg == "" {
		return ""
	}
	return filepath.Join(cfg, "settings.json")
}

func registeredAtAgentSurface(url string) bool {
	trimmed := strings.TrimRight(url, "/")
	return strings.HasSuffix(trimmed, "/rpc/agent") || strings.HasSuffix(trimmed, "/rpc/companion")
}

func checkClaudeMCPRegistration(ctx context.Context, baseURL string, roots ...string) doctorCheck {
	const name = "mcp_registration"
	remedy := fmt.Sprintf("claude mcp add --transport http --scope user fathom %s/rpc/agent", baseURL)

	path := harness.ClaudeUserConfigFile(wiringDriftHome())
	if path == "" {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: "cannot resolve home directory to read ~/.claude.json — check manually: claude mcp get fathom", Remedy: remedy}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: fmt.Sprintf("%s not readable (%v) — check manually: claude mcp get fathom", path, err), Remedy: remedy}
	}
	var doc struct {
		McpServers map[string]struct {
			Type    string   `json:"type"`
			URL     string   `json:"url"`
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: fmt.Sprintf("%s did not parse as JSON (%v)", path, err), Remedy: remedy}
	}
	entry, ok := doc.McpServers["fathom"]
	if !ok {
		return doctorCheck{Name: name, Status: doctorFail, Reason: fmt.Sprintf("no \"fathom\" entry in %s's mcpServers", path), Remedy: remedy}
	}
	if entry.Command != "" {
		root := ""
		if len(roots) > 0 {
			root = roots[0]
		}
		return doctorStdioRegistration(ctx, root, path, entry.Command, entry.Args)
	}
	if entry.Type != "http" || !registeredAtAgentSurface(entry.URL) {
		return doctorCheck{Name: name, Status: doctorFail, Reason: fmt.Sprintf("fathom is registered in %s but not at /rpc/agent (type=%q url=%q)", path, entry.Type, entry.URL), Remedy: remedy}
	}
	return doctorCheck{Name: name, Status: doctorPass, Reason: fmt.Sprintf("configured: registered in %s at %s", path, entry.URL)}
}

// codexMCPEntry mirrors the subset of Codex's [mcp_servers.<name>] TOML
// shape doctor needs (docs/adoption/codex.md §2): `url` selects the HTTP
// transport, `command` the stdio transport.
type codexMCPEntry struct {
	Command string   `toml:"command"`
	Args    []string `toml:"args"`
	URL     string   `toml:"url"`
}

type codexConfigDoc struct {
	McpServers map[string]codexMCPEntry `toml:"mcp_servers"`
}

func checkCodexMCPRegistration(root string, parents ...context.Context) doctorCheck {
	ctx, cancel := doctorContextArg(parents)
	defer cancel()
	const name = "mcp_registration"
	remedy := `add [mcp_servers.fathom] with url = "http://127.0.0.1:7575/rpc/agent" to ~/.codex/config.toml`

	var candidates []string
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		candidates = append(candidates, filepath.Join(home, ".codex", "config.toml"))
	}
	candidates = append(candidates, filepath.Join(root, ".codex", "config.toml"))

	var readAny bool
	var parseErr error
	var unreadable []string
	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err != nil {
			// ADR-027: a config that EXISTS and cannot be read is not the
			// same as one that is not there, and a reader told "no config
			// found" will never think to check permissions.
			if !errors.Is(err, fs.ErrNotExist) {
				unreadable = append(unreadable, fmt.Sprintf("%s (%s)", path, errnoOf(err)))
			}
			continue
		}
		readAny = true
		var doc codexConfigDoc
		if err := toml.Unmarshal(data, &doc); err != nil {
			parseErr = fmt.Errorf("%s: %w", path, err)
			continue
		}
		entry, ok := doc.McpServers["fathom"]
		if !ok {
			continue
		}
		switch {
		case entry.URL != "" && registeredAtAgentSurface(entry.URL):
			return doctorCheck{Name: name, Status: doctorPass, Reason: fmt.Sprintf("configured: registered in %s at %s", path, entry.URL)}
		case entry.URL != "":
			return doctorCheck{Name: name, Status: doctorFail, Reason: fmt.Sprintf("fathom is registered in %s but not at /rpc/agent (url=%q)", path, entry.URL), Remedy: remedy}
		case entry.Command != "":
			return doctorStdioRegistration(ctx, root, path, entry.Command, entry.Args)
		}
	}
	if !readAny {
		reason := "no ~/.codex/config.toml or .codex/config.toml — verify manually: codex mcp list"
		if len(unreadable) > 0 {
			reason = fmt.Sprintf("a Codex config EXISTS but could not be read: %s — this is a permissions problem, not a missing registration", strings.Join(unreadable, "; "))
		}
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: reason, Remedy: remedy}
	}
	if parseErr != nil {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: fmt.Sprintf("could not parse Codex config: %v", parseErr), Remedy: remedy}
	}
	failReason := "no [mcp_servers.fathom] entry found in any readable Codex config"
	if len(unreadable) > 0 {
		failReason += fmt.Sprintf("; %d further config(s) could NOT BE READ and may carry one: %s", len(unreadable), strings.Join(unreadable, "; "))
	}
	return doctorCheck{Name: name, Status: doctorFail, Reason: failReason, Remedy: remedy}
}

// ─── 2. daemon reachable ────────────────────────────────────────────────────

func checkDaemonReachable(hc *http.Client, baseURL string) (bool, doctorCheck) {
	const name = "daemon_reachable"
	// The label is RESOLVED, never assumed: this remedy named one machine's
	// service for months, and every other adopter got "Could not find service".
	remedy := daemonRestartHint("")

	resp, err := hc.Get(baseURL + "/healthz")
	if err != nil {
		return false, doctorCheck{Name: name, Status: doctorFail, Reason: doctorReachabilityReason(baseURL, err), Remedy: remedy, probeErr: err}
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false, doctorCheck{Name: name, Status: doctorFail, Reason: fmt.Sprintf("GET %s/healthz returned %d", baseURL, resp.StatusCode), Remedy: remedy}
	}
	return true, doctorCheck{Name: name, Status: doctorPass, Reason: fmt.Sprintf("%s/healthz -> ok", baseURL)}
}

// ─── 3. binary freshness ────────────────────────────────────────────────────

// fathomModulePath is fathom's own go.mod module path — the one signal
// isFathomSourceTree needs to tell "this IS the fathom checkout" apart from
// every other repo doctor might be pointed at.
const fathomModulePath = "github.com/geoffmilleraz/fathom"

// isFathomSourceTree reports whether root is the fathom checkout itself
// (its go.mod declares fathomModulePath), never a guess: it is the ONLY
// repo where comparing a fathom binary's embedded revision, or resolving a
// daemon-relative call-log path, against root is meaningful. O23
// (docs/dogfood-observations.md): doctor was written and exercised only
// inside fathom's own repo, where "the repo being doctored" and "the repo
// the binary/daemon came from" happen to be the same directory — checked
// here so binary_fresh (this section) and the call-log resolution
// (checkHooksFiring, below) never again conflate the two in an adopter
// repo. A missing or unreadable go.mod is treated as "not fathom" (the
// overwhelmingly common case for anything that is not this repo), never a
// guessed match.
func isFathomSourceTree(root string) bool {
	yes, _ := fathomSourceTree(root)
	return yes
}

// fathomSourceTree reports whether root is the fathom checkout AND, when it
// is not, WHY — because "go.mod declares another module" and "go.mod could
// not be read" are different facts and only one of them is about the repo
// (ADR-027, RC-7).
//
// The observed failure: this check reported "<root>'s go.mod does not declare
// github.com/geoffmilleraz/fathom" against the fathom checkout itself, whose
// go.mod declares exactly that on line 1. The read had failed — a wedged TCC
// verdict on the external volume — and the message stated the opposite of the
// truth, then SKIPped instead of FAILing. Degrading toward "fine" is the one
// direction an instrument may not degrade in.
func fathomSourceTree(root string) (bool, string) {
	path := filepath.Join(root, "go.mod")
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
	case errors.Is(err, fs.ErrNotExist):
		return false, "has no go.mod, so it is not a Go module checkout"
	default:
		return false, fmt.Sprintf("has a go.mod that could NOT BE READ (%s) — this is an instrument failure, not a fact about the repository; on an external volume it is usually a wedged TCC verdict", errnoOf(err))
	}
	for _, line := range strings.Split(string(data), "\n") {
		if mod, ok := strings.CutPrefix(strings.TrimSpace(line), "module "); ok {
			m := strings.TrimSpace(mod)
			if m == fathomModulePath {
				return true, ""
			}
			return false, fmt.Sprintf("declares module %s, not %s", m, fathomModulePath)
		}
	}
	return false, "has a go.mod with no module line"
}

// errnoOf returns the operating system's own wording for a failed read.
func errnoOf(err error) string {
	var pe *fs.PathError
	if errors.As(err, &pe) && pe.Err != nil {
		return pe.Err.Error()
	}
	return err.Error()
}

// doctorBinaryRevision returns the running `fathom doctor` binary's own
// embedded VCS revision and dirty flag (version.go's vcsInfo, no ldflags
// needed). A package var so tests can simulate a known build identity
// instead of depending on `go test`'s own build info.
var doctorBinaryRevision = func() (rev string, dirty, ok bool) {
	r, _, modified := vcsInfo()
	return r, modified, r != ""
}

// checkBinaryFresh compares the currently-running binary's own build
// revision against `git rev-parse HEAD` of the checked repo — the same
// one-line doctrine CLAUDE.md's "Dogfood" section and
// docs/adoption/claude-code-internals.md §13 both document by hand
// (`fathom version` vs `git rev-parse HEAD`). This is a LOCAL check: it
// says whether the binary you just ran doctor with is current — the best
// available proxy since there is no remote "ask the daemon its version"
// RPC today — and every PASS says so explicitly (the daemon's installed
// copy stays unverified). A matching stamp is additionally NOT conclusive
// in two verified-detectable cases, both annotated when present:
// vcs.modified (a dirty build's stamp names the commit, not the code) and
// enclosing-checkout stamp poisoning (a `go build` run inside a linked
// worktree NESTED under this checkout embeds THIS checkout's HEAD, not the
// worktree's — traced live: `go build -x` from such a worktree runs every
// `git` command against the enclosing checkout's root).
//
// O23 (docs/dogfood-observations.md): the binary is fathom's own build
// artifact, so "is it fresh" is a question about the FATHOM source tree's
// HEAD — meaningless anywhere else. Live failure this fixes: "binary @
// a0b74df3 does not match HEAD 061afdba", where 061afdba was an ADOPTER
// repo's HEAD, not fathom's — comparing fathom's build revision to a
// different project's commit history can never be meaningful, so this
// check now only runs when root IS the fathom checkout; every other repo
// gets an honest SKIP instead of a guaranteed-wrong FAIL.
func checkBinaryFresh(root string, parents ...context.Context) doctorCheck {
	parent, stop := doctorContextArg(parents)
	defer stop()
	ctx, cancel := doctorProbeContext(parent)
	defer cancel()
	const name = "binary_fresh"
	remedy := "task deploy (signs the rebuilt binary, installs it via temp+rename, restarts the daemon, and asserts it serves HEAD) — never a raw cp over the running executable: an unsigned copy loses the TCC grant and cp truncates a live binary (docs/adoption/claude-code.md)"

	if yes, why := fathomSourceTree(root); !yes {
		status, remedyForRow := doctorSkipped, ""
		if strings.Contains(why, "NOT BE READ") {
			// Blindness is not a reason to skip quietly: the check could
			// not determine whether it applies, which is a WARN.
			status, remedyForRow = doctorWarn, "make go.mod readable and re-run; the check cannot tell whether it applies while it cannot read it"
		}
		return doctorCheck{
			Name: name, Status: status,
			Reason: fmt.Sprintf("binary freshness is only checkable from the fathom source tree; %s %s — compare `fathom version` to the daemon's deployed commit by hand, or run doctor --path against a fathom checkout", root, why),
			Remedy: remedyForRow,
		}
	}

	rev, dirty, ok := doctorBinaryRevision()
	if !ok {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: "this binary carries no embedded VCS revision (built without -buildvcs, or outside a git checkout) — compare `fathom version` to `git rev-parse HEAD` by hand", Remedy: remedy}
	}

	head, err := doctorGitHead(root, ctx)
	if err != nil || head == "" {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: fmt.Sprintf("could not resolve git HEAD at %s: %v", root, err), Remedy: remedy, probeErr: err}
	}

	if strings.HasPrefix(head, rev) || strings.HasPrefix(rev, head) {
		reason := fmt.Sprintf("this binary @ %s matches HEAD %s of its own source tree — this row knows only the local binary; the daemon's build is daemon_fresh's row", shortSHA(rev), shortSHA(head))
		if dirty {
			reason += "; vcs.modified=true (built from a locally modified tree — the stamp names the commit, not the exact code)"
		}
		if n := nestedLinkedWorktrees(ctx, root); n > 0 {
			reason += fmt.Sprintf("; note: %d linked worktree(s) are nested under this checkout — a `go build` run inside one stamps THIS checkout's HEAD (enclosing-checkout resolution), so a matching stamp does not prove the binary was built from this tree's code", n)
		}
		return doctorCheck{Name: name, Status: doctorPass, Reason: reason}
	}

	// DG-13: a bare inequality FAIL cannot tell "binary is stale, deploy
	// catches it up" from "this checkout is behind the running binary,
	// deploying now would REBUILD FROM THE OLDER TREE and reinstall a
	// regression". Resolve the direction with `git merge-base
	// --is-ancestor` before picking the reason/remedy; only when neither
	// side is an ancestor of the other (diverged history, or one revision
	// is unknown to this repo — e.g. a shallow clone) does this fall back
	// to today's direction-less message and generic remedy.
	reason := fmt.Sprintf("this binary @ %s does not match HEAD %s of its own source tree (local binary only; the daemon's build is daemon_fresh's row)", shortSHA(rev), shortSHA(head))
	freshRemedy := remedy
	if binaryBehind, ok := gitIsAncestor(ctx, root, rev, head); ok && binaryBehind {
		// rev is an ancestor of head: the binary was built from an older
		// commit than HEAD. Deploying is the correct fix.
		reason = fmt.Sprintf("binary @ %s is behind HEAD %s", shortSHA(rev), shortSHA(head))
		if n, ok := gitCommitCount(ctx, root, rev, head); ok {
			reason = fmt.Sprintf("binary @ %s is %d commit(s) behind HEAD %s", shortSHA(rev), n, shortSHA(head))
		}
		reason += " — deploy to catch up"
	} else if checkoutBehind, ok := gitIsAncestor(ctx, root, head, rev); ok && checkoutBehind {
		// head is an ancestor of rev: this checkout has NOT pulled the
		// commits the running binary was built from. Deploying from here
		// would rebuild from the older tree and reinstall a regression
		// over the currently-running binary.
		reason = fmt.Sprintf("local checkout HEAD %s is behind the running binary @ %s", shortSHA(head), shortSHA(rev))
		if n, ok := gitCommitCount(ctx, root, head, rev); ok {
			reason = fmt.Sprintf("local checkout HEAD %s is %d commit(s) behind the running binary @ %s", shortSHA(head), n, shortSHA(rev))
		}
		reason += " — do not deploy from here (would install a regression); pull the latest commits first"
		freshRemedy = "git pull/fetch to bring this checkout up to the running binary's revision, then re-run doctor — do NOT run task deploy from here: it would rebuild from this older tree and reinstall a regression over the currently-running binary"
	}
	// Worktree-stamp caveat (this repo's own history — CLAUDE.md's
	// "Dogfood" section, docs/dogfood-observations.md O3): only asserted
	// when it can actually be verified — a linked git worktree's own .git
	// is a FILE (pointing at the main repo's gitdir), never a directory.
	// Comparing THIS binary's embedded revision against a DIFFERENT
	// worktree's HEAD is apples-to-oranges, not genuine staleness, so the
	// note is appended only when that condition is independently true —
	// never asserted speculatively.
	if isGitWorktree(root) {
		reason += " (note: this path is a linked git worktree — if the binary was built from a different worktree of the same repo, a mismatch may reflect worktree divergence rather than a stale deploy)"
	}
	return doctorCheck{Name: name, Status: doctorFail, Reason: reason, Remedy: freshRemedy}
}

// gitIsAncestor reports whether ancestor is an ancestor of (or equal to)
// descendant in root's git history, via `git merge-base --is-ancestor`.
// The second return is false when the question could not be answered at
// all (git unavailable, or a revision unknown to this repo — e.g. a
// shallow clone) — callers must treat that as "undetermined", never as a
// negative answer.
func gitIsAncestor(ctx context.Context, root, ancestor, descendant string) (bool, bool) {
	err := doctorGitCommand(ctx, root, "merge-base", "--is-ancestor", ancestor, descendant).Run()
	if err == nil {
		return true, true
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		// Exit 1 means the question was answered: not an ancestor.
		return false, true
	}
	// Any other failure (exit 128 "unknown revision", git missing, …)
	// means the question itself could not be answered.
	return false, false
}

// gitCommitCount returns the number of commits reachable from `to` but not
// from `from` (`git rev-list --count from..to`). ok is false when the
// count could not be determined; the caller then omits the number rather
// than guessing.
func gitCommitCount(ctx context.Context, root, from, to string) (int, bool) {
	out, err := doctorGitCommand(ctx, root, "rev-list", "--count", from+".."+to).Output()
	if err != nil {
		return 0, false
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, false
	}
	return n, true
}

func doctorGitHead(root string, parents ...context.Context) (string, error) {
	ctx, cancel := doctorContextArg(parents)
	defer cancel()
	out, err := doctorGitCommand(ctx, root, "rev-parse", "HEAD").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// isGitWorktree reports whether root is a linked git worktree (its .git is
// a file, "gitdir: <path>", rather than the main repo's .git directory).
func isGitWorktree(root string) bool {
	info, err := os.Stat(filepath.Join(root, ".git"))
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// nestedLinkedWorktrees counts linked worktrees whose paths sit UNDER root
// (e.g. .claude/worktrees/agent-*). These are the stamp-poisoning hazard
// checkBinaryFresh's doc comment describes: `go build -buildvcs` from
// inside such a worktree resolves the VCS root to the ENCLOSING checkout
// and embeds its HEAD, so a binary carrying root's HEAD may actually have
// been compiled from a nested worktree's different code. Best-effort: any
// git failure reports zero (the note is additive, never load-bearing).
func nestedLinkedWorktrees(ctx context.Context, root string) int {
	out, err := doctorGitCommand(ctx, root, "worktree", "list", "--porcelain").Output()
	if err != nil {
		return 0
	}
	self := root
	if r, err := filepath.EvalSymlinks(root); err == nil {
		self = r
	}
	sep := string(filepath.Separator)
	n := 0
	for _, line := range strings.Split(string(out), "\n") {
		p, ok := strings.CutPrefix(line, "worktree ")
		if !ok {
			continue
		}
		rp := p
		if r, err := filepath.EvalSymlinks(p); err == nil {
			rp = r
		}
		if rp != self && strings.HasPrefix(rp, self+sep) {
			n++
		}
	}
	return n
}

func shortSHA(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}

// ─── 4. hooks installed (Claude Code only) ─────────────────────────────────

// checkHooksInstalled looks for the fathom hooks across EVERY settings
// scope Claude Code merges — project .claude/settings.json, project
// .claude/settings.local.json, and user ~/.claude/settings.json — because
// hooks wired at user scope (a perfectly good wiring) must never read as a
// project-scope FAIL whose remedy would install duplicates (every
// PostToolUse would then fire twice).
//
// When a hook is missing from every readable scope, the contradiction
// probe asks whether that SPECIFIC hook variant has firing evidence
// somewhere doctor cannot see — using whichever log can actually
// distinguish it from its siblings. edit-hook's PostToolUse invocation and
// its two PreToolUse ("--pre") matcher groups all share one CLI and one
// call-log origin ("edit-hook"), so call-log traffic from one can never be
// treated as evidence for another — only the hooks beat log's phase/target
// dimension (edithook.go/edithook_pre.go) tells them apart. session-hook
// and stop-hook are each their own CLI with no ambiguous sibling, so their
// call-log origin IS variant-specific evidence on its own.
//
// A missing hook with no evidence anywhere is genuinely missing —
// `fathom init` (idempotent, and — since it now also reads user scope
// before writing — safe even when some OTHER hook is wired there) is the
// correct remedy. A missing hook WITH evidence is wired at a scope doctor
// cannot read (managed settings, e.g.); prescribing `fathom init` for that
// one would install a project-scope duplicate.
func checkHooksInstalled(root, harness string) doctorCheck {
	ctx, cancel := doctorProbeContext(context.Background())
	defer cancel()
	return checkHooksInstalledContext(ctx, root, harness)
}

func checkHooksInstalledContext(ctx context.Context, root, harness string) doctorCheck {
	const name = "hooks_installed"
	if harness != "claude" {
		// HP-34: a verified non-Claude descriptor gets a REAL row — its own
		// hook files, read for fathom's markers. An unverified one keeps the
		// skip, and harness_hooks reports what it found without claiming it.
		if d, ok := descriptorHooksVerifiable(harness); ok {
			return checkHarnessHooksInstalled(root, d, ctx)
		}
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: hooksNotWiredReason(harness, "Codex has no SessionStart/PostToolUse/Stop hook mechanism — nothing to check")}
	}

	type scopeFile struct{ label, path string }
	scopes := []scopeFile{
		{"project", filepath.Join(root, ".claude", "settings.json")},
		{"project-local", filepath.Join(root, ".claude", "settings.local.json")},
	}
	if p := claudeUserSettingsFile(); p != "" {
		scopes = append(scopes, scopeFile{"user", p})
	}

	specs := fathomHookSpecsAt(root)          // RO-38: an opt-in row is expected only while root's config switches it on
	wiredScopes := map[hookSpecKey][]string{} // spec key (event+matcher+marker) -> scope labels that wire it
	var scopeNotes []string
	indirect := false
	uncertain := map[hookSpecKey]bool{}
	for _, sc := range scopes {
		data, err := os.ReadFile(sc.path)
		if err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				scopeNotes = append(scopeNotes, fmt.Sprintf("%s unreadable: %v", sc.path, err))
			}
			continue // an absent scope file is normal, not a finding
		}
		var parsed map[string]any
		if err := json.Unmarshal(data, &parsed); err != nil {
			scopeNotes = append(scopeNotes, fmt.Sprintf("%s scope %s did not parse as JSON (%v) — hooks wired there are invisible to this check", sc.label, sc.path, err))
			continue
		}
		hooksObj, _ := parsed["hooks"].(map[string]any)
		for _, spec := range specs {
			ev := doctorHookEvidence(ctx, root, hooksObj, spec)
			uncertain[hookSpecKeyOf(spec)] = uncertain[hookSpecKeyOf(spec)] || (ev.unknown && ev.missingScript == "")
			if !ev.found && ev.missingScript != "" {
				scopeNotes = append(scopeNotes, "hook script not found: "+ev.missingScript)
			}
			if ev.found {
				indirect = indirect || ev.indirect
				k := hookSpecKeyOf(spec)
				wiredScopes[k] = append(wiredScopes[k], sc.label)
			}
		}
	}

	var missing []hookSpec
	labelSet := map[string]bool{}
	var labels []string
	for _, spec := range specs {
		found := wiredScopes[hookSpecKeyOf(spec)]
		if len(found) == 0 {
			missing = append(missing, spec)
			continue
		}
		for _, l := range found {
			if !labelSet[l] {
				labelSet[l] = true
				labels = append(labels, l)
			}
		}
	}

	if len(missing) == 0 {
		state := "configured"
		if indirect {
			state = "configured (indirect)"
		}
		reason := fmt.Sprintf("%s: all %d fathom hooks wired (%s scope)", state, len(specs), strings.Join(labels, " + "))
		if len(scopeNotes) > 0 {
			reason += "; " + strings.Join(scopeNotes, "; ")
		}
		return doctorCheck{Name: name, Status: doctorPass, Reason: reason}
	}

	// Uncertainty belongs to a missing hook, never to the whole row.
	var definite []hookSpec
	var unknownLabels []string
	for _, spec := range missing {
		if uncertain[hookSpecKeyOf(spec)] {
			unknownLabels = append(unknownLabels, specLabel(spec))
		} else {
			definite = append(definite, spec)
		}
	}
	if len(unknownLabels) > 0 {
		scopeNotes = append(scopeNotes, "uncertain project wrapper wiring: "+strings.Join(unknownLabels, ", "))
	}
	if len(definite) == 0 {
		return doctorCheck{Name: name, Status: doctorUnknown, Reason: "unknown: " + strings.Join(scopeNotes, "; "), Remedy: "inspect the named project wrappers manually"}
	}
	missing = definite
	checked := "checked project .claude/settings.json, .claude/settings.local.json, user ~/.claude/settings.json"

	var missingLabels []string
	var evidenced []string // "<label> (evidenced by <source>: N call(s))"
	var trulyMissing []string
	for _, spec := range missing {
		label := specLabel(spec)
		missingLabels = append(missingLabels, label)
		if n, source := hookVariantSeenEvidence(root, spec); n > 0 {
			evidenced = append(evidenced, fmt.Sprintf("%s (evidenced by %s: %d call(s))", label, source, n))
		} else {
			trulyMissing = append(trulyMissing, label)
		}
	}

	switch {
	case len(evidenced) > 0 && len(trulyMissing) == 0:
		// Every missing hook has firing evidence elsewhere — wired at a
		// scope doctor cannot read. Never prescribe init: it would install
		// project-scope duplicates for hooks that already fire.
		reason := fmt.Sprintf("no readable settings scope wires hook(s) %s (%s), yet: %s — evidently wired at a scope doctor cannot read. Running `fathom init` now would install project-scope DUPLICATES for these: every hook would fire twice.",
			strings.Join(missingLabels, ", "), checked, strings.Join(evidenced, "; "))
		if len(scopeNotes) > 0 {
			reason += " " + strings.Join(scopeNotes, "; ")
		}
		return doctorCheck{Name: name, Status: doctorFail, Reason: reason, Remedy: "find the scope that wires them (e.g. read ~/.claude/settings.json, or the harness's hook settings UI) before changing anything — do not run `fathom init` for this"}

	case len(evidenced) > 0 && len(trulyMissing) > 0:
		// Mixed: some missing hooks have contradiction evidence, others
		// have none. `fathom init` is now safe to run for the genuinely
		// missing ones — it reads user scope first and skips anything
		// already wired there — but the evidenced ones need a human to
		// find the scope that wires them (possibly one even init cannot
		// read, e.g. managed settings).
		reason := fmt.Sprintf("no readable settings scope wires hook(s) %s (%s). %s show no firing evidence anywhere doctor can see — genuinely missing. %s — evidently wired at a scope doctor cannot read.",
			strings.Join(missingLabels, ", "), checked, strings.Join(trulyMissing, ", "), strings.Join(evidenced, "; "))
		if len(scopeNotes) > 0 {
			reason += " " + strings.Join(scopeNotes, "; ")
		}
		remedy := fmt.Sprintf("fathom init --path %s installs the genuinely-missing hook(s) (%s) safely — it now skips anything already wired at user scope; if the evidenced hook(s) still aren't wired afterward, find the scope that wires them by hand (do not assume `fathom init` covers those)", root, strings.Join(trulyMissing, ", "))
		return doctorCheck{Name: name, Status: doctorFail, Reason: reason, Remedy: remedy}

	default:
		// Nothing missing has any firing evidence anywhere — genuinely
		// missing, install is the correct and safe remedy.
		reason := fmt.Sprintf("no readable settings scope wires hook(s): %s (%s)", strings.Join(missingLabels, ", "), checked)
		if len(scopeNotes) > 0 {
			reason += "; " + strings.Join(scopeNotes, "; ")
		}
		return doctorCheck{Name: name, Status: doctorFail, Reason: reason, Remedy: "fathom init --path " + root}
	}
}

// hooksNotWiredReason is the skipped reason for the hook checks on a
// harness other than Claude Code: the pre-seam Codex wording for "codex"
// (its tests and docs pin it), the descriptor's own honest line for every
// other registry id — a documented-but-unwired hook table says so, a
// harness with no hooks says so.
func hooksNotWiredReason(harnessLabel, codexReason string) string {
	d, ok := harness.Lookup(doctorHarnessID(harnessLabel))
	if ok && d.HooksWired() { // HP-18/19: the harness_hooks row (doctor_harness.go) reads its files
		return fmt.Sprintf("%s hook entries are read by the harness_hooks row (unverified on this machine) — nothing else to check here; the lefthook_gates check covers the git-native equivalent", d.Display)
	}
	if harnessLabel == "codex" {
		return codexReason
	}
	switch {
	case !ok:
		return fmt.Sprintf("no registry descriptor for %q — nothing to check", harnessLabel)
	case d.HasHooks():
		return fmt.Sprintf("%s documents %d hook binding(s) this build does not wire (hooks: not wired, wave 2; descriptor unverified) — nothing to check; the lefthook_gates check covers the git-native equivalent", d.Display, len(d.Hooks))
	default:
		return fmt.Sprintf("%s has no hook mechanism — nothing to check; the lefthook_gates check covers the git-native equivalent", d.Display)
	}
}

// ─── 5b. gate posture ─────────────────────────────────────────────────────

// checkGatePosture states, from the descriptor's fail mode, what a fathom
// guard on this harness can and cannot do (HP-15): on Claude Code the
// guards are advisory and fail-open by design (§5.3); elsewhere the row
// carries the harness's own documented posture. Informational — it never
// fails, because a posture is a fact to disclose, not a defect to fix.
func checkGatePosture(harnessLabel string) doctorCheck {
	const name = "gate_posture"
	d, ok := harness.Lookup(doctorHarnessID(harnessLabel))
	if !ok {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: fmt.Sprintf("no registry descriptor for %q", harnessLabel)}
	}
	reason := d.GatePosture
	if d.ID != harness.ClaudeCode {
		reason += " — fail mode: " + d.FailMode
	}
	return doctorCheck{Name: name, Status: doctorPass, Reason: reason}
}

// hookSpecKey identifies one hookSpec: its (event, matcher) group plus its
// own marker — the same identity ensureHookEntry (init.go) and
// hookMarkerPresent check. fathomHookSpecs has TWO entries sharing one
// marker ("edit-hook --pre", one per PreToolUse matcher group), so the
// group is in the key; and TWO entries sharing one group (PostToolUse(Bash):
// edit-hook and the test-run hook, TR-2), so the marker is too (FA-7,
// O175: keyed by the group alone, edit-hook's entry stood for the test-run
// row, and doctor, reconcile and init all read it wired).
type hookSpecKey struct{ event, matcher, marker string }

func hookSpecKeyOf(spec hookSpec) hookSpecKey {
	return hookSpecKey{event: spec.event, matcher: spec.matcher, marker: spec.marker}
}

// specLabel names one hookSpec including its matcher group, so two specs
// that share a sub (the two PreToolUse "edit-hook --pre" groups) render as
// two distinguishable entries instead of one duplicated string.
func specLabel(spec hookSpec) string {
	if spec.matcher == "" {
		return fmt.Sprintf("%s [%s]", spec.sub, spec.event)
	}
	return fmt.Sprintf("%s [%s, matcher %s]", spec.sub, spec.event, spec.matcher)
}

// hookVariantSeenEvidence reports how many times spec's SPECIFIC hook
// variant has fired, and which log the evidence came from — see
// checkHooksInstalled's doc comment for why the evidence source differs
// per hook (edit-hook's pre/post/matcher variants need the beat log's
// phase+target dimension; session-hook/stop-hook are unambiguous in the
// call log already). n is 0 with source "" for a spec this function has no
// distinguishing signal for (never a guessed pass).
func hookVariantSeenEvidence(root string, spec hookSpec) (n int, source string) {
	switch spec.sub {
	case "edit-hook":
		return editHookBeatCount("post", ""), "hooks beat log (edit-hook, phase=post)"
	case "edit-hook --pre":
		target := "file"
		if spec.matcher == "Bash" {
			target = "command"
		}
		return editHookBeatCount("pre", target), fmt.Sprintf("hooks beat log (edit-hook, phase=pre, target=%s)", target)
	case "session-hook", "stop-hook":
		return originCallCount(root, spec.sub), fmt.Sprintf("call log (origin=%s)", spec.sub)
	default:
		return 0, ""
	}
}

// editHookBeatCount counts "seen" beats on the "edit-hook" channel
// (beats.go's per-user hooks.jsonl) matching phase ("pre" or "post" — a
// beat with no phase meta predates phase-O and is treated as "post", the
// only variant that existed then) and, when target is non-empty, matching
// target too ("file" for the Edit|Write|MultiEdit PreToolUse group,
// "command" for the Bash one — edithook_pre.go). This is the ONE signal
// that can tell edit-hook's PostToolUse invocation apart from its two
// PreToolUse matcher groups; the call log cannot (all three share rpcOrigin
// "edit-hook", set unconditionally in cmdEditHook before the --pre branch).
func editHookBeatCount(phase, target string) int {
	path := beatLogPath()
	if path == "" {
		return 0
	}
	f, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer f.Close()
	beats, err := heartbeat.Load(f, true) // acceptPartial: a torn tail write must not blank the whole history
	if err != nil {
		return 0
	}
	n := 0
	for _, b := range beats {
		if b.Channel != "edit-hook" || b.Kind != "seen" {
			continue
		}
		p := b.Meta["phase"]
		if p == "" {
			p = "post"
		}
		if p != phase {
			continue
		}
		if target != "" && b.Meta["target"] != target {
			continue
		}
		n++
	}
	return n
}

// originCallCount reports how many call-log rows carry the given origin —
// checkHooksFiring's own pattern (readCallLog/resolveCallLogPath,
// instrument_adoption.go), generalized from "edit-hook" to any origin so
// session-hook/stop-hook (each unambiguous in the call log, unlike
// edit-hook's pre/post variants) can reuse it directly. This is only ever
// used for a SECONDARY signal (checkHooksInstalled's contradiction-evidence
// probe, never the primary pass/fail verdict), so — unlike
// resolveCallLogCandidates above — it deliberately does not query the
// daemon for its own "fathom" service path; it only trusts a relative
// call-log path joined against root when root actually IS the fathom
// source tree (O23: the same repo-relative assumption checkHooksFiring
// had, scoped down here rather than plumbing hc/baseURL through
// checkHooksInstalled for a check this minor). Outside the fathom tree
// this honestly returns 0 (no evidence) rather than checking the wrong
// repo and reporting a false negative as if it were conclusive.
func originCallCount(root, origin string) int {
	logPath, explicit := configuredCallLogPath()
	var resolvedPath string
	switch {
	case explicit && filepath.IsAbs(logPath):
		resolvedPath = logPath
	case explicit:
		if !isFathomSourceTree(root) {
			return 0
		}
		resolvedPath = filepath.Join(root, logPath)
	default:
		// O63 item 5: the unconfigured default is state-dir-aware
		// (callLogPathFor), never a literal workspace/ join — same
		// PC-15/O23 gate: only trust it against a root verifiably the
		// daemon's own working directory.
		if !isFathomSourceTree(root) {
			return 0
		}
		resolvedPath = callLogPathFor(root)
	}
	entries, _, read := readCallLog(resolvedPath)
	if !read {
		return 0
	}
	n := 0
	for _, e := range entries {
		if e.Origin == origin {
			n++
		}
	}
	return n
}

// hookMarkerPresent is the read-only twin of init.go's ensureHookEntry: the
// same marker-substring detection, scoped to spec's OWN (event, matcher)
// group exactly like ensureHookEntry's own write path — otherwise the two
// PreToolUse groups (Edit|Write|MultiEdit and Bash), which share the
// identical "fathom edit-hook --pre" marker, would be indistinguishable: a
// marker found in EITHER group would wrongly read as both being wired.
func hookMarkerPresent(hooksObj map[string]any, spec hookSpec) bool {
	arr, _ := hooksObj[spec.event].([]any)
	for _, g := range arr {
		gm, ok := g.(map[string]any)
		if !ok {
			continue
		}
		gMatcher, _ := gm["matcher"].(string)
		if gMatcher != spec.matcher {
			continue
		}
		hs, _ := gm["hooks"].([]any)
		for _, h := range hs {
			hm, ok := h.(map[string]any)
			if !ok {
				continue
			}
			cmdStr, _ := hm["command"].(string)
			if strings.Contains(cmdStr, spec.marker) {
				return true
			}
		}
	}
	return false
}

// ─── 5. lefthook gates (harness-agnostic — git-native) ─────────────────────

// lefthookInvokes reports whether any line of a lefthook config invokes the
// given fathom subcommand — through ANY spelling of the binary: a literal
// "fathom", an absolute/relative path ending in it, or a shell variable
// named for it ($FATHOM, "${FATHOM}"). The audit's live false negative: a
// pre-push spelled FATHOM=/path/bin/fathom; "$FATHOM" verify … was reported
// as "no fathom verify step" by the literal-substring check, telling a
// correctly-gated repo to add what it had (O5-adjacent, plan IO-5).
func lefthookInvokes(content, subcmd string) bool {
	re := regexp.MustCompile(lefthookInvokesPattern(subcmd))
	for _, line := range strings.Split(content, "\n") {
		if re.MatchString(line) {
			return true
		}
	}
	return false
}

// lefthookInvokesPattern is the per-line pattern lefthookInvokes applies —
// and the lefthook pack's lefthook_pre_push_verify entry ships verbatim
// for "verify" (TestLefthookPackPatternMatchesDoctor holds them equal).
func lefthookInvokesPattern(subcmd string) string {
	return `(?i)fathom\}?"?'?\s+` + subcmd + `\b`
}

// checkLefthookGates renders the lefthook pack's verdict (CC-13,
// doctor_packrows.go) — the derivation lives in the concern engine.

// ─── 6. hooks firing (Claude Code only) ─────────────────────────────────────

// fathomServicePath asks the daemon for the "fathom" service's recorded
// corpus root — the fleet-dogfood convention (CLAUDE.md's "Dogfood: the
// fleet index") is that a fathom daemon indexes its OWN source tree under
// exactly that service name. Combined with the launchd WorkingDirectory
// convention (docs/claude-code-integration.md, the shared HTTP server's plist: "anchors the relative
// --call-log default … without it launchd starts the daemon at / and call
// logging is disabled"), that recorded path IS the daemon's own working
// directory in the overwhelmingly common deployment. Best-effort: ("",
// false) on any error, an absent/unnamed "fathom" service, or a relative
// (pre-O10) recorded path — never a guessed location.
func fathomServicePath(hc *http.Client, baseURL string) (string, bool) {
	svcs, _ := fetchServices(hc, baseURL)
	info, ok := svcs["fathom"]
	if !ok || info.path == "" || !filepath.IsAbs(info.path) {
		return "", false
	}
	return info.path, true
}

// resolveCallLogCandidates returns the plausible absolute filesystem
// locations for the daemon's call log, in priority order, deduplicated.
// O23 (docs/dogfood-observations.md): the call log belongs to the DAEMON,
// never the repo being doctored — resolveCallLogPath (instrument_adoption.go)
// mirrors the daemon's own [daemon].call_log / literal-default resolution
// (reused here verbatim, never re-derived), but its relative fallback is
// only safe to join against a root actually known to BE the daemon's
// working directory:
//
//  1. An explicit ABSOLUTE [daemon].call_log always wins outright — it
//     names its own location unambiguously, independent of any repo.
//  2. A relative path is joined against root ONLY when root is itself the
//     fathom checkout (isFathomSourceTree) — the one case where "the repo
//     being doctored" and "the daemon's working directory" are actually
//     the same directory, by the launchd WorkingDirectory convention.
//  3. A relative path is also joined against the daemon's own recorded
//     "fathom" service path (fathomServicePath) when reachable — the
//     candidate that actually matches production in every adopter repo
//     that is not fathom itself, since the daemon indexes its own source
//     tree under that name (CLAUDE.md's fleet-dogfood convention).
//
// Zero candidates means neither is determinable — the caller reports that
// honestly (skipped), never guesses a third location.
func resolveCallLogCandidates(hc *http.Client, baseURL string, daemonUp bool, root string) []string {
	// PC-15: the unconfigured default now resolves to an absolute path
	// from the CALLER's cwd, which is exactly the wrong repo here (O23) —
	// so an unconfigured log is resolved below against roots known to be
	// the daemon's working directory: state-dir-aware (callLogPathFor,
	// O63 item 5) rather than a literal workspace/ join.
	logPath, explicit := configuredCallLogPath()
	if explicit && filepath.IsAbs(logPath) {
		return []string{logPath}
	}

	var out []string
	seen := map[string]bool{}
	add := func(base string) {
		if base == "" {
			return
		}
		p := logPath
		if explicit {
			p = filepath.Join(base, logPath)
		} else {
			p = callLogPathFor(base)
		}
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	if isFathomSourceTree(root) {
		add(root)
	}
	if daemonUp {
		if p, ok := fathomServicePath(hc, baseURL); ok {
			add(p)
		}
	}
	return out
}

// checkHooksFiring answers "has the hook stack ever actually run", not just
// "is it configured" — an EXISTENCE check over the call log (mirrors
// scripts/check-surface-traffic.sh's own existence-vs-liveness-window
// distinction), via resolveCallLogPath/readCallLog (instrument_adoption.go,
// same package — no import needed, no logic duplicated).
//
// O23 (docs/dogfood-observations.md): the call log lives with the DAEMON,
// not the doctored repo — live failure this fixes: "call log
// <adopter>/workspace/logs/mcp-calls.jsonl not found — no hook has ever
// written to it" reported FAIL in a repo where hooks were in fact firing,
// because the log was joined against the wrong root. checkHooksFiring now
// tries every candidate resolveCallLogCandidates offers (self-hosting root,
// then the daemon's own recorded "fathom" service path) before concluding
// anything is missing, and reports SKIPPED — never a guessed FAIL — when no
// candidate is determinable at all.
func checkHooksFiring(hc *http.Client, baseURL string, daemonUp bool, root, harness string) doctorCheck {
	const name = "hooks_firing"
	if harness != "claude" {
		// HP-34: the beat log's `harness` stamp is per-harness firing
		// evidence the call log cannot give (its origins name the VERB, not
		// the harness that ran it).
		if d, ok := descriptorHooksVerifiable(harness); ok {
			return checkHarnessHooksFiring(d)
		}
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: hooksNotWiredReason(harness, "Codex has no PostToolUse hook to fire; the lefthook_gates check covers its git-native equivalent")}
	}

	// Firing needs one readable log, while surface traffic must sum all
	// candidates. Try local candidates before asking the daemon only here.
	candidates := resolveCallLogCandidates(hc, baseURL, false, root)
	localReadable := false
	for _, logPath := range candidates {
		if f, err := os.Open(logPath); err == nil {
			info, statErr := f.Stat()
			_ = f.Close()
			if statErr == nil && info.Mode().IsRegular() {
				localReadable = true
				break
			}
		}
	}
	if !localReadable && daemonUp {
		candidates = resolveCallLogCandidates(hc, baseURL, true, root)
	}
	if len(candidates) == 0 {
		return doctorCheck{
			Name: name, Status: doctorSkipped,
			Reason: fmt.Sprintf("call log path %q is relative, this repo is not the fathom source tree, and the daemon does not index a service named \"fathom\" to locate its own workspace — cannot determine where the daemon's call log lives (the log belongs to the daemon, not this repo)", resolveCallLogPath()),
			Remedy: "set [daemon].call_log to an absolute path in ~/.fathom/config.toml, or check the daemon's own workspace by hand",
		}
	}

	for _, logPath := range candidates {
		entries, _, read := readCallLog(logPath)
		if !read {
			continue
		}
		editHookCalls, verifyCalls := 0, 0
		for _, e := range entries {
			switch e.Origin {
			case "edit-hook":
				editHookCalls++
			case "verify-cli":
				verifyCalls++
			}
		}
		if editHookCalls == 0 {
			return doctorCheck{
				Name: name, Status: doctorFail,
				Reason: fmt.Sprintf("%s has %d entries but none with origin edit-hook — the PostToolUse hook has never fired", logPath, len(entries)),
				Remedy: "edit a file in this repo under Claude Code, then re-run doctor; if still silent, confirm obligations is compiled: fathom compile --service <svc> --dry-run=false",
			}
		}
		return doctorCheck{Name: name, Status: doctorPass, Reason: fmt.Sprintf("firing: %s: %d edit-hook call(s), %d verify-cli call(s)", logPath, editHookCalls, verifyCalls)}
	}

	return doctorCheck{
		Name: name, Status: doctorFail,
		Reason: fmt.Sprintf("checked %d candidate call-log location(s) (%s) — none exist; no hook has ever written to any of them", len(candidates), strings.Join(candidates, ", ")),
		Remedy: "edit a file in this repo under Claude Code (fires edit-hook), then re-run doctor",
	}
}

// ─── 7. the AGENTS.md stanza ────────────────────────────────────────────────

// checkStanzaPresent renders the codex and claude-code packs' entries
// (CC-13, doctor_packrows.go) plus Claude Code's load path
// (doctor_claudeload.go, ADR-059) — the three-case detection lives in the
// concern engine (custodian's stanza_present kind).

// ─── 8. skills installed ────────────────────────────────────────────────────

// checkSkillsInstalled byte-compares every shipped skill's on-disk file
// against the embedded source (skillsdist.FS) — the same content the
// running binary would render on a fresh `fathom init --skills`. Missing,
// stale (present but byte-different), and not-fathom-owned (present but
// missing the marker, so init itself refuses to touch it) are reported as
// distinct problems, never lumped into one undifferentiated "fail".
func checkSkillsInstalled(root, harnessLabel string) doctorCheck {
	const name = "skills_installed"

	var missing, stale, notOwned []string
	current := 0

	check := func(dest, embeddedPath string) {
		want, err := skillsdist.FS.ReadFile(embeddedPath)
		if err != nil {
			return // the embedded source is fixed at compile time; should not happen
		}
		got, err := os.ReadFile(dest)
		if err != nil {
			missing = append(missing, dest)
			return
		}
		if !strings.Contains(string(got), skillMarker) {
			notOwned = append(notOwned, dest)
			return
		}
		if string(got) != string(want) {
			stale = append(stale, dest)
			return
		}
		current++
	}

	switch harnessLabel {
	case "codex":
		remedy := "fathom init --skills=codex"
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			return doctorCheck{Name: name, Status: doctorSkipped, Reason: "cannot resolve home directory to check ~/.codex/prompts/", Remedy: remedy}
		}
		for _, skillName := range skillsdist.Names {
			check(filepath.Join(home, ".codex", "prompts", skillName+".md"), skillName+"/codex-prompt.md")
		}
		return summarizeSkillsCheck(name, remedy, harnessLabel, missing, stale, notOwned, current)
	case "claude":
		remedy := "fathom init --skills=claude --path " + root
		for _, skillName := range skillsdist.Names {
			check(filepath.Join(root, ".claude", "skills", skillName, "SKILL.md"), skillName+"/SKILL.md")
		}
		return summarizeSkillsCheck(name, remedy, "claude", missing, stale, notOwned, current)
	default:
		// Every other registry descriptor discovers skills through the
		// open Agent Skills path (<root>/.agents/skills, `--skills=agents`);
		// the descriptor says so, and says it is unverified here.
		remedy := "fathom init --skills=agents --path " + root
		for _, skillName := range skillsdist.Names {
			check(filepath.Join(root, ".agents", "skills", skillName, "SKILL.md"), skillName+"/SKILL.md")
		}
		c := summarizeSkillsCheck(name, remedy, harnessLabel, missing, stale, notOwned, current)
		if d, ok := harness.Lookup(doctorHarnessID(harnessLabel)); ok && !d.Skills.Verified {
			c.Reason += " (discovery of .agents/skills by " + d.Display + " is unverified on this machine)"
		}
		return c
	}
}

func summarizeSkillsCheck(name, remedy, harness string, missing, stale, notOwned []string, current int) doctorCheck {
	switch {
	case len(missing) == len(skillsdist.Names):
		return doctorCheck{Name: name, Status: doctorFail, Reason: fmt.Sprintf("none of the %d shipped skills are installed for %s", len(skillsdist.Names), harness), Remedy: remedy}
	case len(missing) > 0 || len(stale) > 0 || len(notOwned) > 0:
		var parts []string
		if len(missing) > 0 {
			parts = append(parts, "missing: "+strings.Join(missing, ", "))
		}
		if len(stale) > 0 {
			parts = append(parts, "stale (differs from the shipped source): "+strings.Join(stale, ", "))
		}
		if len(notOwned) > 0 {
			parts = append(parts, "not fathom-owned (no marker; init refuses to touch): "+strings.Join(notOwned, ", "))
		}
		return doctorCheck{Name: name, Status: doctorFail, Reason: strings.Join(parts, "; "), Remedy: remedy}
	default:
		return doctorCheck{Name: name, Status: doctorPass, Reason: fmt.Sprintf("all %d shipped skills current for %s", current, harness)}
	}
}

// ─── 9. .fathom/config.toml CUE-valid ───────────────────────────────────────

// checkConfigValid invokes the same config.Load the daemon/CLI already use
// for every config read — CUE validation happens inside it, so this check
// never re-implements the schema.
func checkConfigValid(root string) doctorCheck {
	const name = "config_valid"
	userPath := config.DefaultUserPath()
	projPath := projectdir.Config.Path(root)

	if _, err := config.Load(userPath, projPath); err != nil {
		return doctorCheck{Name: name, Status: doctorFail, Reason: err.Error(), Remedy: "fix the reported key/file, or remove the offending config.toml and re-run `fathom init` to scaffold a fresh one"}
	}
	return doctorCheck{Name: name, Status: doctorPass, Reason: "user + project .fathom/config.toml (where present) validate against the CUE schema"}
}

// ─── 10. obligations served ─────────────────────────────────────────────────

// doctorObligationsProbeFile is the AGENTS.md fallback probe: `fathom init`
// writes it into every wired repo (ADR-059: the stanza's one file), so it
// is meaningful without repo-specific configuration; CLAUDE.md follows it
// for a repository still on the older layout. O23 (docs/dogfood-observations.md): a
// service whose compiled obligations are code-scoped (e.g. "**/*.go") can
// serve real rules while CLAUDE.md alone answers zero — live evidence: the
// same daemon served 6 rules on cmd/conjure/main.go while probing CLAUDE.md
// alone found none. checkObligationsServed now probes a real INDEXED
// source file (obligationsProbeCandidates) ahead of this fallback, so a
// code-scoped-only install reports the real PASS it deserves instead of a
// false FAIL.
const doctorObligationsProbeFile = "AGENTS.md"

// obligationsProbeCandidates returns the ordered, deduplicated list of
// repo-relative files checkObligationsServed probes obligations with: a
// real indexed source file in the service's most common extension first
// (dominantLanguageProbeFile — "" when undeterminable, e.g. list_sources
// unavailable or the service is empty), then the AGENTS.md/CLAUDE.md
// fallbacks a `fathom init`-wired repo carries (AGENTS.md first, ADR-059). Each candidate is tried
// in turn (checkObligationsServed) and the first that actually serves rules
// answers the check — never just the first one, which is what let a
// code-scoped-only install FAIL under CLAUDE.md alone (O23).
func obligationsProbeCandidates(hc *http.Client, baseURL, svc string) []string {
	var out []string
	seen := map[string]bool{}
	add := func(f string) {
		if f != "" && !seen[f] {
			seen[f] = true
			out = append(out, f)
		}
	}
	add(dominantLanguageProbeFile(hc, baseURL, svc))
	add(doctorObligationsProbeFile)
	add("CLAUDE.md")
	return out
}

// dominantLanguageProbeFile picks a real, indexed, repo-relative file to
// probe obligations with, biased toward the service's most common source
// extension — best-effort via list_sources (listSourcesForDoctor,
// doctor_phaseo.go; reused, not re-derived). Returns "" when list_sources
// has nothing to offer (RPC failure, empty service, no extensioned files),
// never a guessed path.
func dominantLanguageProbeFile(hc *http.Client, baseURL, svc string) string {
	sources, err := listSourcesForDoctor(hc, baseURL, svc)
	if err != nil || len(sources) == 0 {
		return ""
	}
	counts := map[string]int{}
	for _, s := range sources {
		if ext := strings.ToLower(filepath.Ext(s)); ext != "" {
			counts[ext]++
		}
	}
	if len(counts) == 0 {
		return ""
	}
	exts := make([]string, 0, len(counts))
	for e := range counts {
		exts = append(exts, e)
	}
	sort.Strings(exts) // deterministic tie-break
	bestExt, bestN := "", -1
	for _, e := range exts {
		if counts[e] > bestN {
			bestExt, bestN = e, counts[e]
		}
	}
	for _, s := range sources {
		if strings.ToLower(filepath.Ext(s)) == bestExt {
			return s
		}
	}
	return ""
}

// obligationsProbeResult is one probe file's obligations() answer.
type obligationsProbeResult struct {
	file  string
	rules []struct {
		Status string `json:"status"`
	}
	watermark  string
	indexStale bool
	// coverage is O54 deliverable 5's source: the obligations layer's
	// LATEST compile run completeness, when the daemon reported one
	// (layer_coverage, present whenever a compile_runs row exists for the
	// layer — see internal/mcp/semantic_tools.go's layerCoverageBackend).
	coverage   doctorLayerCoverage
	hasCovered bool
}

// doctorLayerCoverage mirrors semantic.LayerCoverage's JSON shape — a
// package-local copy rather than an import so doctor.go stays free of an
// internal/semantic dependency for one small struct. Status is O60's
// addition: the envelope now carries layer_coverage:{"status":"unknown"}
// (instead of omitting the key) when the backend has no compile_runs row
// for the service — a real, distinct field from the numeric case below,
// not just another zero value. CorpusGrew (wave O4, item 3; ADR-037
// addendum) was silently dropped by this mirror despite the wire payload
// (semantic.LayerCoverage's own corpus_grew,omitempty tag) already carrying
// it — an operator daemon reading 42/66 compiled with a real corpus_grew:14
// (14 CURRENT-corpus sections no compile run has ever even seen — distinct
// from a stale digest) got a WARN that named the 42/66 split but never the
// corpus growth, understating how much of the gap is "the corpus outgrew
// the last compile" versus "this run's own budget stopped it early".
type doctorLayerCoverage struct {
	SectionsTotal    int  `json:"sections_total"`
	SectionsCompiled int  `json:"sections_compiled"`
	BudgetStopped    bool `json:"budget_stopped"`
	CorpusGrew       int  `json:"corpus_grew"`
	// PrefilterSkipped (ADR-042 decision 2, O109) mirrors
	// semantic.LayerCoverage.PrefilterSkipped — sections the decision
	// port's pre-filter skipped before compilation, never compiled and
	// never failed. Zero when no Decider was configured for the run that
	// produced this coverage, or every section passed.
	PrefilterSkipped int    `json:"prefilter_skipped"`
	Status           string `json:"status"`
	// UnreadCorpus (RT-143, I38) mirrors semantic.LayerCoverage's
	// unread_corpus: guides the live count could not read (over the read
	// cap, unreadable), which make every count above a lower bound.
	UnreadCorpus []string `json:"unread_corpus"`
}

// fetchObligationsProbe calls the obligations MCP tool for one
// (service, file) pair via callMCPTool (doctor_phaseo.go; reused, not
// re-derived).
func fetchObligationsProbe(hc *http.Client, baseURL, svc, file string) (obligationsProbeResult, error) {
	inner, err := callMCPTool(hc, baseURL, "obligations", map[string]any{"service": svc, "file": file})
	if err != nil {
		return obligationsProbeResult{}, err
	}
	var payload struct {
		Rules []struct {
			Status string `json:"status"`
		} `json:"rules"`
		Watermark     string               `json:"compiled_at_watermark"`
		IndexStale    bool                 `json:"index_stale"`
		LayerCoverage *doctorLayerCoverage `json:"layer_coverage"`
	}
	if err := json.Unmarshal(inner, &payload); err != nil {
		return obligationsProbeResult{}, err
	}
	res := obligationsProbeResult{file: file, rules: payload.Rules, watermark: payload.Watermark, indexStale: payload.IndexStale}
	if payload.LayerCoverage != nil {
		res.coverage = *payload.LayerCoverage
		res.hasCovered = true
	}
	return res, nil
}

// obligationsLayerCompiled reports whether the "obligations" semantic layer
// has EVER been compiled for svc, via staleness_report's optional `layers`
// block (pkg/client.ClaimsSummary's per-layer claim-status counts) — a
// signal independent of any single probe FILE's scope match. This is what
// lets checkObligationsServed tell "never compiled" (this returns false)
// apart from "compiled, but none of the probed files fall in any rule's
// scope" (this returns true even though every probe served zero rules) —
// the third, honest outcome O23 asks for, distinct from both.
func obligationsLayerCompiled(hc *http.Client, baseURL, svc string) (bool, error) {
	inner, err := callMCPTool(hc, baseURL, "staleness_report", map[string]any{"service": svc})
	if err != nil {
		return false, err
	}
	var payload struct {
		Layers map[string]json.RawMessage `json:"layers"`
	}
	if err := json.Unmarshal(inner, &payload); err != nil {
		return false, err
	}
	raw, ok := payload.Layers[semantic.LayerObligations]
	if !ok {
		return false, nil
	}
	var statusCounts map[string]int
	if err := json.Unmarshal(raw, &statusCounts); err != nil {
		return false, nil // unexpected shape — no evidence either way, not an error
	}
	for _, n := range statusCounts {
		if n > 0 {
			return true, nil
		}
	}
	return false, nil
}

// checkObligationsServed resolves the service the same way the hooks do —
// EVERY candidate identity (edithook.go's serviceCandidates loop), not a
// blind first guess — intersected with the daemon's actual service list,
// so its remedy can never name a service that is not in the index (a
// `fathom compile --service <ghost>` would be a zero-section bogus run).
// A repo whose candidates match NO indexed service gets the distinct
// not-indexed verdict (that state also means the hooks cannot inject here
// — dogfood-observations.md O15 is exactly this, live). An explicit
// --service is honored but cross-checked against the hook candidates: a
// service the hooks would never resolve gets a pass-with-note, because
// obligations serving over MCP does not imply injection works.
//
// O23 (docs/dogfood-observations.md): probing CLAUDE.md alone reports FAIL
// for a genuinely healthy, code-scoped-only install (live: 6 rules served
// on cmd/conjure/main.go, zero on CLAUDE.md). This now probes SEVERAL
// candidates (obligationsProbeCandidates: a real indexed source file first,
// then CLAUDE.md/AGENTS.md) and, only when every probe comes back empty,
// distinguishes "never compiled" (FAIL, remedy = compile) from "compiled,
// but none of the probed files matched any rule's scope" (obligationsLayer
// Compiled — a distinct WARN, never conflated with the FAIL case).
func checkObligationsServed(hc *http.Client, baseURL, root, explicitSvc string, daemonUp bool, parents ...context.Context) doctorCheck {
	ctx, cancel := doctorContextArg(parents)
	defer cancel()
	const name = "obligations_served"

	if !daemonUp {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: "daemon unreachable — cannot query obligations (see daemon_reachable)"}
	}

	// Hook-candidate identities, deduped, in hook preference order — via
	// candidatesWithDaemon, the EXACT resolution the hooks use post-O15:
	// the daemon's recorded corpus paths are authoritative, so a repo whose
	// dir basename names no service still resolves to the indexed service
	// that records this root.
	var candNames []string
	seen := map[string]bool{}
	for _, c := range candidatesWithDaemon(hc, baseURL, root) {
		n := strings.ToLower(c.name)
		if !seen[n] {
			seen[n] = true
			candNames = append(candNames, n)
		}
	}

	probeNames := candNames
	if explicitSvc != "" {
		probeNames = []string{explicitSvc}
	}

	svcs, err := fetchServices(hc, baseURL)
	if err != nil {
		return doctorCheck{Name: name, Status: doctorUnknown, Reason: "cannot read daemon service registry: " + err.Error(), probeErr: err}
	}
	indexed := make([]string, 0, len(svcs))
	for s := range svcs {
		indexed = append(indexed, s)
	}
	sort.Strings(indexed)

	svc := ""
	for _, n := range probeNames {
		if _, ok := svcs[n]; ok {
			svc = n
			break
		}
	}
	if svc == "" {
		return doctorCheck{
			Name: name, Status: doctorFail,
			Reason: fmt.Sprintf("no candidate service ID (%s) is in the daemon's index (indexed: %s) — the hooks resolve the same candidates, so obligations injection cannot fire for this repo either", strings.Join(probeNames, ", "), strings.Join(indexed, ", ")),
			Remedy: fmt.Sprintf("ingest this repo under a matching name (fathom init --path %s), or re-run doctor with --service <id> if it is indexed under a different one", root),
		}
	}
	remedy := fmt.Sprintf("fathom compile --service %s --dry-run=false (this calls your configured model provider and spends model budget)", svc)

	// Explicit-service honesty note: serving obligations for a service the
	// hooks would never resolve proves the layer compiles, NOT that the
	// edit-hook can inject it (O15's exact trap).
	hookMismatchNote := ""
	if explicitSvc != "" {
		isCandidate := false
		for _, n := range candNames {
			if n == explicitSvc {
				isCandidate = true
				break
			}
		}
		if !isCandidate {
			hookMismatchNote = fmt.Sprintf(" — note: --service %s is NOT among the hook candidate IDs (%s), so the hooks will not resolve this service and obligations injection will not fire in sessions here even though serving works", explicitSvc, strings.Join(candNames, ", "))
		}
	}

	probes := obligationsProbeCandidates(hc, baseURL, svc)

	var tried []string
	var lastRPCErr error
	for _, file := range probes {
		res, err := fetchObligationsProbe(hc, baseURL, svc, file)
		if err != nil {
			lastRPCErr = err
			continue
		}
		tried = append(tried, file)
		if len(res.rules) == 0 && res.watermark == "" {
			continue // this probe answered zero — try the next candidate before giving up
		}

		// Drift honesty (rules are served FLAGGED, never dropped — serve.go's
		// Obligation.Status contract): a payload full of stale/contradicted
		// rules must not read as an unqualified clean pass.
		drifted := 0
		for _, r := range res.rules {
			if r.Status != "" {
				drifted++
			}
		}
		reason := fmt.Sprintf("obligations served for service %s, file %s (%d rule(s), watermark %s)", svc, res.file, len(res.rules), res.watermark)
		if drifted > 0 {
			reason += fmt.Sprintf(" — note: %d of %d rule(s) carry a drift status (stale/contradicted); served flagged, not dropped — recompile to refresh", drifted, len(res.rules))
		}
		if res.indexStale {
			reason += " — note: index_stale is true, the compiled index disagrees with the file on disk"
		}
		// O54 deliverable 5: a PARTIALLY compiled obligations layer serves
		// real rules — this check must stay a real, observable PASS — but a
		// budget-stopped compile can leave a file's own governing guide
		// uncompiled with nothing else in `fathom doctor` saying so. WARN,
		// never FAIL (docs/intent invariants: a config-quality signal, not
		// a wiring-broken verdict — doctorWarn's own doc comment).
		status := doctorPass
		switch {
		case res.hasCovered && res.coverage.Status == "unknown":
			// O60: the store answered honestly that it has no compile_runs
			// row for this service (a layer compiled before coverage
			// tracking existed is the live case) — a stranger reading this
			// row must be told the completeness of the layer is unrecorded
			// rather than reading silence as "nothing wrong here".
			reason += " — note: coverage unrecorded (layer compiled before coverage was recorded; recompile to record coverage)"
		case res.hasCovered && res.coverage.SectionsTotal > 0 && res.coverage.SectionsCompiled < res.coverage.SectionsTotal:
			status = doctorWarn
			// wave O4, item 3: corpus_grew names the CURRENT-corpus sections
			// no compile run has ever even seen (semantic.LiveLayerCoverage's
			// own doc comment) — distinct from a section that was compiled
			// once but has since drifted (a stale digest, counted in the
			// SectionsCompiled/SectionsTotal split above but never as
			// growth). Omitted from the sentence entirely when zero: a
			// budget-stopped run whose corpus has NOT grown since is not
			// misdescribed as having outgrown anything.
			corpusGrewNote := ""
			if res.coverage.CorpusGrew > 0 {
				corpusGrewNote = fmt.Sprintf(", corpus_grew=%d", res.coverage.CorpusGrew)
			}
			// O109: prefilter_skipped names sections the decision port's
			// pre-filter skipped (never compiled, never failed) — distinct
			// from a section the budget never reached. Omitted when zero,
			// the same discipline corpus_grew already follows.
			prefilterNote := ""
			if res.coverage.PrefilterSkipped > 0 {
				prefilterNote = fmt.Sprintf(", prefilter_skipped=%d", res.coverage.PrefilterSkipped)
			}
			reason += fmt.Sprintf(" — WARN: the obligations layer is PARTIALLY compiled (%d/%d sections, budget_stopped=%v%s%s) — some governing guides may not be compiled yet; resume with `fathom compile --service %s --layer obligations --dry-run=false --continue`",
				res.coverage.SectionsCompiled, res.coverage.SectionsTotal, res.coverage.BudgetStopped, corpusGrewNote, prefilterNote, svc)
		}
		// RT-143 / I38: an unread guide makes the coverage above a lower
		// bound — never a clean "fully compiled" PASS, and named.
		if res.hasCovered && len(res.coverage.UnreadCorpus) > 0 {
			status = doctorWarn
			reason += fmt.Sprintf(" — WARN: %d governing guide(s) could not be read, so coverage is a lower bound: %s",
				len(res.coverage.UnreadCorpus), strings.Join(res.coverage.UnreadCorpus, "; "))
		}
		return doctorCheck{Name: name, Status: status, Reason: reason + hookMismatchNote}
	}

	if len(tried) == 0 && lastRPCErr != nil {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: lastRPCErr.Error(), probeErr: lastRPCErr}
	}

	// Every probe served zero rules and no watermark. O23's third outcome:
	// distinguish "never compiled" from "compiled, but none of the probed
	// files fall in any rule's scope" via obligationsLayerCompiled — a
	// signal independent of any single probe file.
	compiled, compErr := obligationsLayerCompiled(hc, baseURL, svc)
	// The only remedy for an uncompiled layer is a model-backed compile. With
	// no provider configured that cannot run, so say so instead of failing a
	// fresh install on something its owner has not opted into.
	if (compErr != nil || !compiled) && !obligationsProviderConfigured(ctx, root) {
		return doctorCheck{
			Name: name, Status: doctorSkipped,
			Reason: fmt.Sprintf("no obligations compiled for service %q and no model provider is configured — compiling obligations calls a model and spends budget, so it is opt-in", svc),
			Remedy: "optional: configure a provider in .fathom/config.toml (see `fathom providers`), then `fathom compile --service " + svc + " --dry-run=false` (spends model budget)",
		}
	}
	switch {
	case compErr != nil:
		return doctorCheck{
			Name: name, Status: doctorUnknown,
			Reason:   fmt.Sprintf("obligations(%s, ...) served zero rules; compile facts unavailable via staleness_report: %v", svc, compErr),
			Remedy:   "check daemon reachability (see daemon_reachable), then re-run fathom doctor",
			probeErr: compErr,
		}
	case !compiled:
		return doctorCheck{
			Name: name, Status: doctorFail,
			Reason: fmt.Sprintf("the obligations layer has never been compiled for service %q (probed %s, all served zero rules and no watermark)", svc, strings.Join(tried, ", ")),
			Remedy: remedy,
		}
	default:
		return doctorCheck{
			Name: name, Status: doctorWarn,
			Reason: fmt.Sprintf("the obligations layer IS compiled for service %q, but none of the probed files (%s) fall within any compiled rule's scope — this may be correct (rules scoped narrower than these probes) rather than broken; verify with obligations(service:%q, file:<a file you actually edit>)", svc, strings.Join(tried, ", "), svc),
		}
	}
}

// ─── 11. surface_traffic flowing ────────────────────────────────────────────

// checkSurfaceTraffic is the CLI-native twin of
// scripts/check-surface-traffic.sh's existence-check semantics ("has this
// surface fired", not "did it fire recently"), scoped to the "agent"
// surface (the recommended registration target per §1). Scope caveat, from
// status(section:"daemon")'s own tool description (internal/mcp/meta.go;
// the retired server_stats tool's field, unchanged — SR-23): the
// surface_traffic counters are PROCESS-LIFETIME — "since this process
// started", reset on every daemon restart — so a zero here means "no
// calls since this daemon process started", never "no calls ever".
func checkSurfaceTraffic(hc *http.Client, baseURL string, daemonUp bool, root string) doctorCheck {
	const name = "surface_traffic"
	remedy := "deploy a current fathom binary (see docs/adoption/claude-code.md in the fathom repo)"

	if !daemonUp {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: "daemon unreachable — cannot query status (see daemon_reachable)"}
	}

	req, err := originHeader(baseURL, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"status","arguments":{"section":"daemon"}}}`))
	if err != nil {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: err.Error(), probeErr: err}
	}
	resp, err := hc.Do(req)
	if err != nil {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: fmt.Sprintf("POST %s/rpc: %v", baseURL, err), probeErr: err}
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: err.Error(), probeErr: err}
	}
	inner, err := unwrapRPCResult(raw)
	if err != nil {
		return doctorCheck{Name: name, Status: doctorFail, Reason: fmt.Sprintf("status: %v", err), Remedy: remedy}
	}
	var payload struct {
		SurfaceTraffic map[string]struct {
			Calls  int    `json:"calls"`
			LastTS string `json:"last_ts"`
		} `json:"surface_traffic"`
	}
	if err := json.Unmarshal(inner, &payload); err != nil {
		return doctorCheck{Name: name, Status: doctorSkipped, Reason: fmt.Sprintf("parsing status response: %v", err)}
	}
	if payload.SurfaceTraffic == nil {
		return doctorCheck{Name: name, Status: doctorFail, Reason: "status(section:daemon) has no surface_traffic block — the daemon predates A1-1 call-log instrumentation, or no call logger is wired (--call-log)", Remedy: remedy}
	}
	entry, ok := payload.SurfaceTraffic["agent"]
	if ok && entry.Calls > 0 {
		return doctorCheck{Name: name, Status: doctorPass, Reason: fmt.Sprintf("agent surface: %d call(s) since this daemon process started, last at %s", entry.Calls, entry.LastTS)}
	}

	// IT-4: the live counter is process-lifetime and reads zero after every
	// daemon restart (comment above) — that alone is not proof nothing ever
	// happened. Fall back to the persistent call log, resolved the same way
	// hooks_firing resolves it, before reporting a fail.
	if n, lastTS, found := recentAgentCallsInPersistentLog(hc, baseURL, daemonUp, root, 7*24*time.Hour); found {
		return doctorCheck{Name: name, Status: doctorPass, Reason: fmt.Sprintf("live counter reset by restart; persistent log shows %d agent call(s), last %s", n, lastTS)}
	}
	// Before the first recorded call nothing is broken yet: WARN, not FAIL.
	return doctorCheck{Name: name, Status: doctorWarn, Reason: "no call recorded yet: the \"agent\" surface (/rpc/agent) shows zero calls since this daemon process started (the counters are process-lifetime — they reset on restart), and the persistent call log shows no agent-surface call in the last 7 days either", Remedy: "make one real tool call against /rpc/agent (a live session, or the fathom-doctor skill's tie-in step), then re-run doctor"}
}

// recentAgentCallsInPersistentLog scans the persistent call log — every
// candidate resolveCallLogCandidates offers, the same resolution
// checkHooksFiring uses — for rows whose surface names "agent" and whose
// timestamp falls within `within` of now. found is false when no candidate
// log carries such a row, including when no candidate is even resolvable.
func recentAgentCallsInPersistentLog(hc *http.Client, baseURL string, daemonUp bool, root string, within time.Duration) (count int, lastTS string, found bool) {
	cutoff := time.Now().Add(-within)
	for _, logPath := range resolveCallLogCandidates(hc, baseURL, daemonUp, root) {
		entries, _, read := readCallLog(logPath)
		if !read {
			continue
		}
		for _, e := range entries {
			if !strings.Contains(e.Surface, "agent") {
				continue
			}
			ts, err := time.Parse(time.RFC3339, e.TS)
			if err != nil || ts.Before(cutoff) {
				continue
			}
			count++
			if e.TS > lastTS {
				lastTS = e.TS
			}
		}
	}
	return count, lastTS, count > 0
}

// ─── output ─────────────────────────────────────────────────────────────────

func printDoctorTable(w io.Writer, harness, svc, root string, checks []doctorCheck) {
	fmt.Fprintf(w, "fathom doctor — service %q, harness %q, root %s\n\n", svc, harness, root)
	fmt.Fprintf(w, "%-20s %-7s %s\n", "CHECK", "STATUS", "REASON")
	for _, c := range checks {
		fmt.Fprintf(w, "%-20s %-7s %s\n", c.Name, statusLabel(c.Status), c.Reason)
		if c.Remedy != "" {
			fmt.Fprintf(w, "%-20s %-7s remedy: %s\n", "", "", c.Remedy)
		}
	}

	pass, warn, fail, skipped := tallyDoctorChecks(checks)
	if warn > 0 {
		fmt.Fprintf(w, "\n%d checks: %d pass, %d warn, %d fail, %d skipped\n", len(checks), pass, warn, fail, skipped)
	} else {
		fmt.Fprintf(w, "\n%d checks: %d pass, %d fail, %d skipped\n", len(checks), pass, fail, skipped)
	}
}

func statusLabel(s doctorStatus) string {
	switch s {
	case doctorPass:
		return "PASS"
	case doctorFail:
		return "FAIL"
	case doctorWarn:
		return "WARN"
	case doctorUnknown:
		return "UNKNOWN"
	case doctorTimeout:
		return "TIMEOUT"
	default:
		return "SKIP"
	}
}

func tallyDoctorChecks(checks []doctorCheck) (pass, warn, fail, skipped int) {
	for _, c := range checks {
		switch c.Status {
		case doctorPass:
			pass++
		case doctorWarn:
			warn++
		case doctorFail:
			fail++
		case doctorSkipped, doctorUnknown, doctorTimeout:
			skipped++
		}
	}
	return pass, warn, fail, skipped
}

func printDoctorJSON(w io.Writer, harness, svc, root string, checks []doctorCheck, channels ...[]doctorChannel) {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(doctorJSONPayload(harness, svc, root, checks, channels...))
}

// doctorRunner runs `fathom doctor` (registered below; EW-45).
type doctorRunner struct{}

func (doctorRunner) execVerb(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return cmdDoctor(args, stdout, stderr)
}

func init() {
	declareVerb(verb{Name: "doctor", Group: "operate", Order: 130, Run: doctorRunner{},
		Summary: "Deterministic wiring diagnosis: registration, daemon, hooks, stanza, skills, config, obligations, traffic"})
}

// obligationsProviderConfigured reports whether a model provider is usable
// for root: a [profiles] entry, or an env-overlay lane that builds. A seam so
// tests need no real credential.
var obligationsProviderConfigured = func(ctx context.Context, root string) bool {
	loaded, err := config.Load(config.DefaultUserPath(), projectdir.Config.Path(root))
	if err != nil {
		return false
	}
	if len(loaded.Config.Profiles) > 0 {
		return true
	}
	p, _, err := config.ResolveProfile(loaded.Config, loaded.Provenance, "")
	if err != nil {
		return false
	}
	_, err = buildLLMClientForProfileContext(ctx, p)
	return err == nil
}

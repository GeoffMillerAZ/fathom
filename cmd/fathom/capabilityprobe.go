package main

// capabilityprobe.go — CR-2's caller side. Filesystem reads and HTTP calls
// for capability detection stay in this command package, so
// internal/capability remains pure and testable in a table.
//
// The probe is deliberately best-effort: an unreachable daemon or an
// unreadable file leaves a field at its zero value, and the registry then
// reports that capability off WITH a reason. A degraded probe therefore
// produces an honest, useful table rather than an error — which is the O23
// lesson (doctor's repo-relative assumptions) applied at the source.

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/capability"
	"github.com/geoffmilleraz/fathom/internal/custodian"
	"github.com/geoffmilleraz/fathom/internal/intent"
)

// buildCapabilityProbe gathers every observable fact the registry reads.
func buildCapabilityProbe(baseURL, root, svc string, now time.Time) capability.Probe {
	p := capability.Probe{RepoRoot: root, Service: svc}

	// The intent corpus is read from disk only — no daemon, no RPC — so a
	// probe stays cheap even when the daemon is down (EP-10).
	if c, err := intent.Load(root); err == nil {
		p.IntentRouterPresent = c.RouterPath != ""
		p.IntentDecisionsHome = c.DecisionsHome
		p.IntentConditions = len(c.Conditions)
	}

	m := cachedObservatoryModel(baseURL, now)
	p.LedgerSessions = m.Impact.LedgerSessions
	p.AgentSurfaceTools = m.AgentSurfaceTools
	p.AgentSchemaBytes = m.AgentSchemaBytes

	// Find this repository's row. Matching by PATH first is what makes the
	// verb correct in a consumer repo: the service id there is whatever
	// its owner chose, and guessing it from the directory name is the O23
	// mistake in miniature.
	var row *obsServiceRow
	for i := range m.Services {
		s := &m.Services[i]
		if sameRepoPath(s.Path, root) || (svc != "" && strings.EqualFold(s.Service, svc)) {
			row = s
			break
		}
	}
	if row != nil {
		if p.Service == "" {
			p.Service = row.Service
		}
		p.IndexedSymbols = row.Symbols
		// fleetreport's verdict is one of "fresh", "stale: …" or
		// "unknown: …". Only the middle one is a defect; collapsing
		// unknown into stale would fail every repository whose daemon is
		// merely unreachable.
		p.IndexFresh = row.Freshness == "fresh"
		p.IndexStale = strings.HasPrefix(row.Freshness, "stale")
		if !p.IndexFresh && row.Freshness != "" {
			p.IndexNote = row.Freshness
		}
		p.HooksNote = row.HooksNote
		p.HooksReferenced = strings.Contains(row.HooksNote, "hook reference")
		p.StanzaPresent = row.StanzaNote == "present"
		p.ObligationsCompiled = strings.Contains(row.CompileNote, "obligations")
		if row.Config != nil {
			for _, l := range row.Config.Layers {
				p.LayerPacks = append(p.LayerPacks, l.Name)
			}
		}
	}

	// Facts that are cheaper to read locally than to fetch, and that must
	// work when the daemon is down.
	p.ObligationRules = countRuleSources(root)
	p.AssessRuns = countAssessRuns(root)
	p.ModuleBindings = countModuleBindings(root)
	p.VerifyInLefthook = lefthookRunsVerify(root)
	guards := inspectGuardRules(root)
	p.GuardRules = guards.Count
	if guards.Broken {
		p.GuardProblem = guards.Note
	}
	if row == nil || row.Config == nil {
		p.LayerPacks = localLayerPacks(root)
	}
	return p
}

// probeTarget names what the table is about, preferring the service id
// and falling back to the path — never printing an empty subject.
func probeTarget(p capability.Probe) string {
	if p.Service != "" {
		return p.Service
	}
	if p.RepoRoot != "" {
		return filepath.Base(p.RepoRoot)
	}
	return "this repository"
}

// sameRepoPath compares two repository roots tolerantly: a trailing
// separator or a symlinked home should not decide whether fathom thinks it
// is looking at your repository.
func sameRepoPath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	ca := filepath.Clean(a)
	cb := filepath.Clean(b)
	if strings.EqualFold(ca, cb) {
		return true
	}
	ra, errA := filepath.EvalSymlinks(ca)
	rb, errB := filepath.EvalSymlinks(cb)
	return errA == nil && errB == nil && strings.EqualFold(ra, rb)
}

// countRuleSources counts authored obligation rule files. Authored and
// compiled are different facts (see the obligations detector), so this
// counts sources on disk and says nothing about whether they are live.
func countRuleSources(root string) int {
	entries, err := os.ReadDir(filepath.Join(root, ".fathom", "rules"))
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		switch filepath.Ext(e.Name()) {
		case ".cue", ".md", ".yaml", ".yml":
			n++
		}
	}
	return n
}

// countModuleBindings counts bindings in .fathom/modules.cue. It counts
// `module:` keys rather than parsing CUE: a probe that fails to load must
// degrade to zero-with-a-reason, not to an error that stops the table.
func countModuleBindings(root string) int {
	data, err := os.ReadFile(filepath.Join(root, ".fathom", "modules.cue"))
	if err != nil {
		return 0
	}
	n := 0
	for _, line := range strings.Split(string(data), "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "//") {
			continue
		}
		if strings.HasPrefix(t, "module:") || strings.HasPrefix(t, "module :") {
			n++
		}
	}
	return n
}

func localLayerPacks(root string) []string {
	entries, err := os.ReadDir(filepath.Join(root, ".fathom", "layers"))
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	return out
}

// lefthookRunsVerify reads the lefthook pack's lefthook_pre_push_verify —
// the entry doctor's lefthook_gates row renders — so the capability table
// and the doctor row can never disagree about what "the gate is wired"
// means: a verify on a non-comment line of the pre-push hook. The pack's
// corpus is lefthook.yml / lefthook.yaml; `.lefthook.yml`, which it does
// not resolve yet (corpus.go names the gap), keeps the raw-line matcher.
func lefthookRunsVerify(root string) bool {
	if fs, err := doctorPackFindings(root, []packRef{{"lefthook", "lefthook_pre_push_verify"}}); err == nil && len(fs[0].Files) > 0 {
		return fs[0].Status == custodian.StatusHolds
	}
	data, err := os.ReadFile(filepath.Join(root, ".lefthook.yml"))
	return err == nil && lefthookInvokes(string(data), "verify")
}

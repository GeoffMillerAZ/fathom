//go:build harness_maintenance

package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"math/bits"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/internal/hookeval"
)

// Hook records are immutable within one invocation. The receiver's protected
// file store serializes concurrent state transitions.
type hookReference struct {
	ProjectID string `json:"projectId"`
	Host      string `json:"host"`
	Path      string `json:"path"`
	SHA256    string `json:"sha256"`
}
type hookEvaluator struct {
	Profile       string `json:"profile"`
	Executable    string `json:"executable"`
	SHA256        string `json:"sha256"`
	TimeoutMillis int    `json:"timeoutMillis"`
}
type hookPolicy struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}
type hookProfile struct {
	Version           string        `json:"version"`
	Profile           string        `json:"profile"`
	IssuedAt          string        `json:"issuedAt"`
	ExpiresAt         string        `json:"expiresAt"`
	Host              string        `json:"host"`
	HostID            string        `json:"hostId"`
	ProjectID         string        `json:"projectId"`
	PrincipalID       string        `json:"principalId"`
	RequiredControlID string        `json:"requiredControlId"`
	AdvisoryControlID string        `json:"advisoryControlId"`
	SamplingKey       string        `json:"samplingKey"`
	Evaluator         hookEvaluator `json:"evaluator"`
	Policy            hookPolicy    `json:"policy"`
	MaxBoundaryMillis int           `json:"maxBoundaryMillis"`
}
type hookReport struct {
	Version              string   `json:"version"`
	Status               string   `json:"status"`
	Code                 string   `json:"code"`
	RequiredDecision     string   `json:"requiredDecision"`
	Codes                []string `json:"codes"`
	Class                string   `json:"class"`
	ClassSource          string   `json:"classSource"`
	AdvisorySelected     bool     `json:"advisorySelected"`
	AdvisoryEvaluated    bool     `json:"advisoryEvaluated"`
	AdviceMatches        int      `json:"adviceMatches"`
	NoActionAuthority    bool     `json:"noActionAuthority"`
	Coverage             string   `json:"coverage"`
	Installation         string   `json:"installation"`
	StateWritesAttempted int      `json:"stateWritesAttempted"`
	StateWritesCommitted int      `json:"stateWritesCommitted"`
	NativeStampAttempts  int      `json:"nativeStampAttempts"`
	NativeStampInvoked   int      `json:"nativeStampInvoked"`
	NativeStampCompleted int      `json:"nativeStampCompleted"`
	EvaluatorAttempts    int      `json:"evaluatorAttempts"`
	EvaluatorInvoked     int      `json:"evaluatorInvoked"`
	// Completed means the OS child exit was observed, regardless of reply validity.
	EvaluatorCompleted int `json:"evaluatorCompleted"`
}

func hookBaseReport() hookReport {
	return hookReport{Version: "harnessctl.spawn-report/1", Status: "hold", Code: "input_invalid", Codes: []string{}, NoActionAuthority: true, Coverage: "selected-project-spawn-events-only", Installation: "unobserved"}
}
func hookHold(r *hookReport, code string) int {
	r.Status = "hold"
	r.Code = code
	r.RequiredDecision = ""
	r.Codes = []string{}
	r.Class = ""
	r.ClassSource = ""
	r.AdvisorySelected = false
	r.AdvisoryEvaluated = false
	r.AdviceMatches = 0
	return 2
}
func hookSelectedReference(c maintenanceController, projectID, host string) (hookReference, bool) {
	var out hookReference
	found := false
	for _, ref := range c.Hooks {
		if ref.ProjectID == projectID && ref.Host == host {
			if found {
				return out, false
			}
			out = ref
			found = true
		}
	}
	return out, found
}
func hookControllerValid(c maintenanceController) bool {
	if len(c.Hooks) > 512 || (c.Hooks != nil && len(c.Hooks) == 0) {
		return false
	}
	seen := map[string]bool{}
	projects := map[string]bool{}
	for _, p := range c.Projects {
		projects[p.ID] = true
	}
	for _, h := range c.Hooks {
		k := h.ProjectID + "\x00" + h.Host
		if !projects[h.ProjectID] || !oneOf(h.Host, "claude", "codex") || !maintenanceAbs(h.Path) || !maintenanceHash(h.SHA256) || seen[k] {
			return false
		}
		seen[k] = true
	}
	return true
}
func hookProfileValid(p hookProfile, ref hookReference, ctx maintenanceContext) bool {
	if p.Version != "harnessctl.spawn-profile/1" || p.Profile != "spawn-economics/1" || p.Host != ref.Host || p.HostID != ctx.HostID || p.ProjectID != ctx.ProjectID || p.PrincipalID != ctx.PrincipalID || !validID(p.RequiredControlID) || !validID(p.AdvisoryControlID) || p.RequiredControlID == p.AdvisoryControlID || !maintenanceHash(p.SamplingKey) || p.Evaluator.Profile != "agent-hooks/spawn-eval/1" || !maintenanceAbs(p.Evaluator.Executable) || !maintenanceHash(p.Evaluator.SHA256) || p.Evaluator.TimeoutMillis < 1 || p.Evaluator.TimeoutMillis > 1000 || !maintenanceAbs(p.Policy.Path) || !maintenanceHash(p.Policy.SHA256) || p.MaxBoundaryMillis < 1 || p.MaxBoundaryMillis > 5000 {
		return false
	}
	_, _, err := interval(p.IssuedAt, p.ExpiresAt, maintenanceStampTime(ctx.Stamp), 366*24*time.Hour)
	return err == nil
}
func hookControls(controls []effectiveControl, p hookProfile) (required, advisory effectiveControl, ok bool) {
	r, a := false, false
	for _, c := range controls {
		if c.ID == p.RequiredControlID {
			if r {
				return
			}
			required = c
			r = true
		}
		if c.ID == p.AdvisoryControlID {
			if a {
				return
			}
			advisory = c
			a = true
		}
	}
	ok = r && a && required.Event == "before-tool" && required.Class == "mandatory" && required.SamplePPM == 1000000 && required.OnFault == "hold" && !required.AllowMaintenance && advisory.Event == "before-tool" && advisory.Class == "advisory" && advisory.OnFault == "hold" && required.MaxWorkMillis >= 1 && advisory.MaxWorkMillis >= 1
	return
}

type hookSamplingUnit struct {
	Version    string `json:"version"`
	Host       string `json:"host"`
	ProjectID  string `json:"projectId"`
	ControlID  string `json:"controlId"`
	InstanceID string `json:"instanceId"`
	SessionID  string `json:"sessionId"`
	ToolUseID  string `json:"toolUseId"`
}

func hookAdviceSelected(key string, unit hookSamplingUnit, ppm int) bool {
	if ppm <= 0 {
		return false
	}
	if ppm >= 1000000 {
		return true
	}
	b, _ := hex.DecodeString(key)
	u, _ := json.Marshal(unit)
	mac := hmac.New(sha256.New, b)
	_, _ = mac.Write(u)
	sum := mac.Sum(nil)
	hi, _ := bits.Mul64(binary.BigEndian.Uint64(sum[:8]), 1000000)
	return hi < uint64(ppm)
}
func hookApplyResult(r *hookReport, result hookeval.Result) int {
	r.RequiredDecision = result.RequiredDecision
	r.Codes = append([]string{}, result.Codes...)
	r.Class = result.Class
	r.ClassSource = result.ClassSource
	r.AdvisoryEvaluated = result.AdvisoryEvaluated
	r.AdviceMatches = result.AdviceMatches
	r.Status = "complete"
	switch result.RequiredDecision {
	case "pass":
		r.Code = "ok"
		return 0
	case "deny":
		r.Code = "spawn_denied"
		return 1
	case "not-applicable":
		r.Status = "not-applicable"
		r.Code = "resume_not_spawn"
		return 0
	default:
		return hookHold(r, "evaluator_incomplete")
	}
}

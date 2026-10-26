//go:build harness_maintenance

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"reflect"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/internal/hookeval"
)

type hookArgs struct{ operation, controller, pin, project, host, input, format string }

func hookParseArgs(args []string) (hookArgs, bool) {
	var a hookArgs
	if len(args) == 0 || !oneOf(args[0], "evaluate", "run") || len(args[1:])%2 != 0 {
		return a, false
	}
	a.operation = args[0]
	a.format = "json"
	seen := map[string]bool{}
	for i := 1; i < len(args); i += 2 {
		name, value := args[i], args[i+1]
		if seen[name] {
			return a, false
		}
		seen[name] = true
		switch name {
		case "--controller":
			a.controller = value
		case "--controller-sha256":
			a.pin = value
		case "--project":
			a.project = value
		case "--host":
			a.host = value
		case "--input":
			a.input = value
		case "--format":
			a.format = value
		default:
			return a, false
		}
	}
	if !maintenanceAbs(a.controller) || !maintenanceHash(a.pin) || !maintenanceAbs(a.project) || !oneOf(a.host, "claude", "codex") {
		return a, false
	}
	if a.operation == "evaluate" {
		if !maintenanceAbs(a.input) || !oneOf(a.format, "json", "text") {
			return a, false
		}
	} else if seen["--input"] || seen["--format"] {
		return a, false
	}
	return a, true
}
func hooksUsage() string {
	return "harnessctl hooks evaluate --controller ABS --controller-sha256 HASH --project ABS --host claude|codex --input ABS --format json|text\nharnessctl hooks run --controller ABS --controller-sha256 HASH --project ABS --host claude|codex\n"
}

type hookChildPort interface {
	Evaluate(context.Context, []byte, time.Duration) ([]byte, bool, bool, int, error)
	intact() bool
	Close() error
}
type hookDeps struct {
	maintenance maintenanceCLIDeps
	stdin       io.Reader
	now         func() time.Time
	prepare     func(hookEvaluator, string) (hookChildPort, error)
}

func hookDefaultDeps() hookDeps {
	return hookDeps{maintenance: maintenanceDefaultDeps(), stdin: os.Stdin, now: time.Now, prepare: func(e hookEvaluator, root string) (hookChildPort, error) {
		child, err := hookPrepareChild(e, root)
		if err != nil {
			return nil, err
		}
		return child, nil
	}}
}
func runHooks(args []string, out, errOut io.Writer) int {
	return runHooksWithDeps(args, out, errOut, hookDefaultDeps())
}
func hookOutput(out io.Writer, format string, r hookReport, exit int) int {
	if out == nil {
		return 2
	}
	var b []byte
	var e error
	if format == "text" {
		b, e = json.MarshalIndent(r, "", "  ")
	} else {
		b, e = json.Marshal(r)
	}
	if e != nil || len(b)+1 > 16<<10 || hookValidateReport(r) != nil {
		return 2
	}
	b = append(b, '\n')
	n, e := out.Write(b)
	if e != nil || n != len(b) {
		return 2
	}
	return exit
}
func hookValidateReport(r hookReport) error {
	if r.Version != "harnessctl.spawn-report/1" || !r.NoActionAuthority || r.Coverage != "selected-project-spawn-events-only" || r.Installation != "unobserved" || r.Codes == nil || r.StateWritesAttempted < 0 || r.StateWritesAttempted > 2 || r.StateWritesCommitted < 0 || r.StateWritesCommitted > r.StateWritesAttempted || r.NativeStampAttempts < 0 || r.NativeStampAttempts > 3 || r.NativeStampInvoked < 0 || r.NativeStampCompleted < 0 || r.NativeStampCompleted > r.NativeStampInvoked || r.NativeStampInvoked > r.NativeStampAttempts || r.EvaluatorAttempts < 0 || r.EvaluatorInvoked < 0 || r.EvaluatorCompleted < 0 || r.EvaluatorCompleted > r.EvaluatorInvoked || r.EvaluatorInvoked > r.EvaluatorAttempts || r.EvaluatorAttempts > 1 {
		return errHookInput
	}
	if r.Status == "hold" {
		if r.RequiredDecision != "" || len(r.Codes) != 0 || r.Class != "" || r.ClassSource != "" || r.AdvisorySelected || r.AdvisoryEvaluated || r.AdviceMatches != 0 || !oneOf(r.Code, "unsupported", "input_invalid", "controller_invalid", "controller_changed", "project_unregistered", "hook_unregistered", "profile_invalid", "policy_invalid", "config_incomplete", "policy_conflict", "state_unavailable", "state_invalid", "state_busy", "state_ambiguous", "clock_invalid", "rearm_required", "native_unavailable", "native_incomplete", "signature_invalid", "binding_mismatch", "storage_incomplete", "evaluator_unavailable", "evaluator_incomplete", "boundary_expired", "output_incomplete") {
			return errHookInput
		}
		return nil
	}
	if r.Status != "complete" && r.Status != "not-applicable" {
		return errHookInput
	}
	if r.NativeStampCompleted != 3 || r.EvaluatorAttempts != 1 || r.EvaluatorInvoked != 1 || r.EvaluatorCompleted != 1 {
		return errHookInput
	}
	var emptyHash [32]byte
	digest := hex.EncodeToString(emptyHash[:])
	peer := hookeval.Result{Version: hookeval.ResultVersion, RequestID: digest, ContextSHA256: digest, PolicySHA256: digest, NoActionAuthority: true, RequiredDecision: r.RequiredDecision, Codes: r.Codes, Class: r.Class, ClassSource: r.ClassSource, AdvisoryEvaluated: r.AdvisoryEvaluated, AdviceMatches: r.AdviceMatches}
	if hookeval.ValidateResult(peer) != nil {
		return errHookInput
	}
	if r.Status == "not-applicable" {
		if r.Code != "resume_not_spawn" || r.RequiredDecision != "not-applicable" || len(r.Codes) != 1 || r.Codes[0] != "resume_not_spawn" || r.Class != "not-applicable" || r.ClassSource != "not-applicable" || r.AdvisorySelected || r.AdvisoryEvaluated || r.AdviceMatches != 0 {
			return errHookInput
		}
		return nil
	}
	if r.Code == "ok" && r.RequiredDecision == "pass" && r.AdvisorySelected == r.AdvisoryEvaluated || r.Code == "spawn_denied" && r.RequiredDecision == "deny" && !r.AdvisoryEvaluated && r.AdviceMatches == 0 {
		return nil
	}
	return errHookInput
}
func hookReadInput(a hookArgs, deps hookDeps) ([]byte, string) {
	if a.operation == "evaluate" {
		s, e := deps.maintenance.read(a.input, 1<<20)
		if e != nil {
			return nil, "input_invalid"
		}
		return s.data, ""
	}
	if deps.stdin == nil {
		return nil, "input_invalid"
	}
	raw, e := io.ReadAll(io.LimitReader(deps.stdin, 1<<20+1))
	if e != nil || len(raw) > 1<<20 {
		return nil, "input_invalid"
	}
	return raw, ""
}
func hookStateCheck(r *hookReport, store maintenanceStorePort, ctx maintenanceContext) (*maintenanceState, []effectiveControl, string) {
	current, e := store.Load()
	if e != nil {
		return nil, nil, maintenanceStoreCode(e)
	}
	if current == nil {
		return nil, nil, "state_unavailable"
	}
	transition := maintenanceTransition(current, maintenanceTransitionInput{Operation: "check"}, ctx)
	if transition.Write {
		if transition.State == nil {
			return nil, nil, "state_invalid"
		}
		r.StateWritesAttempted++
		if e := store.Commit(*transition.State, "check"); e != nil {
			return nil, nil, maintenanceStoreCode(e)
		}
		r.StateWritesCommitted++
	}
	if transition.Exit != 0 {
		if oneOf(transition.Code, "rearm_required", "clock_invalid", "binding_mismatch", "state_invalid") {
			return nil, nil, transition.Code
		}
		return nil, nil, "state_invalid"
	}
	if transition.State == nil {
		return nil, nil, "state_invalid"
	}
	return transition.State, transition.Controls, ""
}
func hookStateEquivalent(a, b *maintenanceState) bool {
	if a == nil || b == nil {
		return false
	}
	x, y := *a, *b
	x.Sequence = 0
	y.Sequence = 0
	x.Stamp = maintenanceHostStamp{}
	y.Stamp = maintenanceHostStamp{}
	x.ClockAnchor = maintenanceHostStamp{}
	y.ClockAnchor = maintenanceHostStamp{}
	return reflect.DeepEqual(x, y)
}

type hookContextUnit struct {
	Version            string `json:"version"`
	ControllerSHA256   string `json:"controllerSHA256"`
	ProfileSHA256      string `json:"profileSHA256"`
	PolicySHA256       string `json:"policySHA256"`
	EvaluatorSHA256    string `json:"evaluatorSHA256"`
	ProjectID          string `json:"projectId"`
	PrincipalID        string `json:"principalId"`
	Host               string `json:"host"`
	HostID             string `json:"hostId"`
	InstanceID         string `json:"instanceId"`
	AuthorizationEpoch uint64 `json:"authorizationEpoch"`
	Sequence           uint64 `json:"sequence"`
	RequiredRate       int    `json:"requiredRate"`
	AdvisoryRate       int    `json:"advisoryRate"`
	NativeSHA256       string `json:"nativeSHA256"`
}

func hookDigest(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }
func hookContextHash(c maintenanceContext, p hookProfile, profileHash, policyHash string, state *maintenanceState, required, advisory effectiveControl, raw []byte) string {
	unit := hookContextUnit{Version: "harnessctl.spawn-context/1", ControllerSHA256: c.ControllerSHA256, ProfileSHA256: profileHash, PolicySHA256: policyHash, EvaluatorSHA256: p.Evaluator.SHA256, ProjectID: c.ProjectID, PrincipalID: c.PrincipalID, Host: p.Host, HostID: c.HostID, InstanceID: state.InstanceID, AuthorizationEpoch: state.AuthorizationEpoch, Sequence: state.Sequence, RequiredRate: required.SamplePPM, AdvisoryRate: advisory.SamplePPM, NativeSHA256: hookDigest(raw)}
	b, _ := json.Marshal(unit)
	return hookDigest(b)
}
func runHooksWithDeps(args []string, out, errOut io.Writer, deps hookDeps) int {
	a, parsed := hookParseArgs(args)
	if !parsed && len(args) > 0 && args[0] == "run" {
		a.operation = "run"
	}
	r := hookBaseReport()
	var native maintenanceNativePort
	var store maintenanceStorePort
	var child hookChildPort
	whole, cancelWhole := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelWhole()
	active := whole
	budget := 5 * time.Second
	var cancelProfile context.CancelFunc
	var startTime time.Time
	code := func() int {
		if !parsed {
			return hookHold(&r, "input_invalid")
		}
		if deps.maintenance.read == nil || deps.maintenance.newNative == nil || deps.maintenance.newStore == nil || deps.now == nil || deps.prepare == nil {
			return hookHold(&r, "unsupported")
		}
		startTime = deps.now()
		c, controllerSnap, fail := maintenancePinnedController(deps.maintenance.read, a.controller, a.pin)
		if fail != "" {
			return hookHold(&r, fail)
		}
		if !hookControllerValid(c) {
			return hookHold(&r, "controller_invalid")
		}
		selected, ok := maintenanceControllerProject(c, a.project)
		if !ok {
			return hookHold(&r, "project_unregistered")
		}
		ref, ok := hookSelectedReference(c, selected.ID, a.host)
		if !ok {
			return hookHold(&r, "hook_unregistered")
		}
		var e error
		native, e = deps.maintenance.newNative(c.Native)
		if e != nil {
			return hookHold(&r, "native_unavailable")
		}
		stamp, e := native.Stamp(active)
		if e != nil {
			return hookHold(&r, "native_incomplete")
		}
		ctx, fail := maintenanceBoundary(deps.maintenance, c, controllerSnap, a.controller, a.pin, a.project, stamp)
		if fail != "" {
			return hookHold(&r, fail)
		}
		profileSnap, e := deps.maintenance.read(ref.Path, 65536)
		if e != nil || hookDigest(profileSnap.data) != ref.SHA256 {
			return hookHold(&r, "profile_invalid")
		}
		var profile hookProfile
		if strictJSON(profileSnap.data, &profile) != nil || !hookProfileValid(profile, ref, ctx) {
			return hookHold(&r, "profile_invalid")
		}
		budget = time.Duration(profile.MaxBoundaryMillis) * time.Millisecond
		remainingBudget := budget - deps.now().Sub(startTime)
		if remainingBudget <= 0 || whole.Err() != nil {
			return hookHold(&r, "boundary_expired")
		}
		active, cancelProfile = context.WithTimeout(whole, remainingBudget)
		policySnap, e := deps.maintenance.read(profile.Policy.Path, hookeval.MaxPolicyBytes)
		if e != nil || hookDigest(policySnap.data) != profile.Policy.SHA256 {
			return hookHold(&r, "policy_invalid")
		}
		store, e = deps.maintenance.newStore(selected.StateRoot, nil)
		if e != nil {
			return hookHold(&r, maintenanceStoreCode(e))
		}
		stamp, e = native.Stamp(active)
		if e != nil {
			return hookHold(&r, "native_incomplete")
		}
		ctx, fail = maintenanceBoundary(deps.maintenance, c, controllerSnap, a.controller, a.pin, a.project, stamp)
		if fail != "" {
			return hookHold(&r, fail)
		}
		if !sameSnapshot(deps.maintenance.read, profileSnap, 65536) || !sameSnapshot(deps.maintenance.read, policySnap, hookeval.MaxPolicyBytes) || !hookProfileValid(profile, ref, ctx) {
			return hookHold(&r, "binding_mismatch")
		}
		pre, controls, fail := hookStateCheck(&r, store, ctx)
		if fail != "" {
			return hookHold(&r, fail)
		}
		required, advisory, ok := hookControls(controls, profile)
		if !ok {
			return hookHold(&r, "config_incomplete")
		}
		child, e = deps.prepare(profile.Evaluator, c.Native.PreparationRoot)
		if e != nil {
			return hookHold(&r, "evaluator_unavailable")
		}
		raw, fail := hookReadInput(a, deps)
		if fail != "" {
			return hookHold(&r, fail)
		}
		input, e := hookNormalize(raw, a.host, a.project)
		if e != nil {
			return hookHold(&r, "input_invalid")
		}
		selectedAdvice := false
		if a.operation == "evaluate" && input.Subject.Operation == "spawn" {
			selectedAdvice = hookAdviceSelected(profile.SamplingKey, hookSamplingUnit{Version: "harnessctl.advice-sample/1", Host: a.host, ProjectID: selected.ID, ControlID: advisory.ID, InstanceID: pre.InstanceID, SessionID: input.SessionID, ToolUseID: input.ToolUseID}, advisory.SamplePPM)
		}
		r.AdvisorySelected = selectedAdvice
		request, e := hookeval.NewRequest(hookContextHash(ctx, profile, ref.SHA256, profile.Policy.SHA256, pre, required, advisory, raw), policySnap.data, input.Subject, selectedAdvice)
		if e != nil {
			return hookHold(&r, "input_invalid")
		}
		requestRaw, e := hookeval.MarshalRequest(request)
		if e != nil {
			return hookHold(&r, "input_invalid")
		}
		remaining := time.Duration(profile.MaxBoundaryMillis)*time.Millisecond - deps.now().Sub(startTime)
		timeout := time.Duration(profile.Evaluator.TimeoutMillis) * time.Millisecond
		for _, v := range []int{required.MaxWorkMillis} {
			if d := time.Duration(v) * time.Millisecond; d < timeout {
				timeout = d
			}
		}
		if selectedAdvice {
			if d := time.Duration(advisory.MaxWorkMillis) * time.Millisecond; d < timeout {
				timeout = d
			}
		}
		if remaining < timeout {
			timeout = remaining
		}
		if timeout <= 0 {
			return hookHold(&r, "boundary_expired")
		}
		r.EvaluatorAttempts++
		reply, invoked, completed, status, e := child.Evaluate(active, requestRaw, timeout)
		if invoked {
			r.EvaluatorInvoked++
		}
		if completed {
			r.EvaluatorCompleted++
		}
		if e != nil {
			return hookHold(&r, "evaluator_incomplete")
		}
		if len(reply) < 3 || reply[0] != '{' || reply[len(reply)-2] != '}' || reply[len(reply)-1] != '\n' || bytes.IndexByte(reply[:len(reply)-1], '\n') >= 0 {
			return hookHold(&r, "evaluator_incomplete")
		}
		result, e := hookeval.DecodeResult(reply[:len(reply)-1])
		wantStatus := 0
		if result.RequiredDecision == "deny" {
			wantStatus = 1
		}
		if result.RequiredDecision == "unknown" {
			wantStatus = 2
		}
		if e != nil || hookeval.BindResult(result, request) != nil || status != wantStatus || result.RequiredDecision == "unknown" {
			return hookHold(&r, "evaluator_incomplete")
		}
		stamp, e = native.Stamp(active)
		if e != nil {
			return hookHold(&r, "native_incomplete")
		}
		fresh, fail := maintenanceBoundary(deps.maintenance, c, controllerSnap, a.controller, a.pin, a.project, stamp)
		if fail != "" {
			return hookHold(&r, fail)
		}
		if !sameSnapshot(deps.maintenance.read, profileSnap, 65536) || !sameSnapshot(deps.maintenance.read, policySnap, hookeval.MaxPolicyBytes) || !child.intact() || !hookProfileValid(profile, ref, fresh) {
			return hookHold(&r, "binding_mismatch")
		}
		loaded, e := store.Load()
		if e != nil {
			return hookHold(&r, maintenanceStoreCode(e))
		}
		if !reflect.DeepEqual(pre, loaded) {
			return hookHold(&r, "binding_mismatch")
		}
		post, postControls, fail := hookStateCheck(&r, store, fresh)
		if fail != "" {
			return hookHold(&r, fail)
		}
		postRequired, postAdvisory, ok := hookControls(postControls, profile)
		if !ok || !hookStateEquivalent(pre, post) || !reflect.DeepEqual(required, postRequired) || !reflect.DeepEqual(advisory, postAdvisory) {
			return hookHold(&r, "binding_mismatch")
		}
		if deps.now().Sub(startTime) > time.Duration(profile.MaxBoundaryMillis)*time.Millisecond {
			return hookHold(&r, "boundary_expired")
		}
		return hookApplyResult(&r, result)
	}()
	if native != nil {
		counts := native.Counts()
		r.NativeStampAttempts = counts.StampAttempts
		r.NativeStampInvoked = counts.StampInvoked
		r.NativeStampCompleted = counts.StampCompleted
	}
	if child != nil {
		if child.Close() != nil {
			code = hookHold(&r, "evaluator_incomplete")
		}
	}
	if native != nil {
		if native.Close() != nil {
			code = hookHold(&r, "native_incomplete")
		}
	}
	if store != nil {
		if store.Close() != nil {
			code = hookHold(&r, "storage_incomplete")
		}
	}
	if !startTime.IsZero() && (active.Err() != nil || deps.now != nil && deps.now().Sub(startTime) > budget) {
		code = hookHold(&r, "boundary_expired")
	}
	if cancelProfile != nil {
		cancelProfile()
	}
	if a.operation == "run" {
		if code != 0 {
			if errOut == nil {
				return 2
			}
			msg := "harnessctl spawn check blocked\n"
			n, e := io.WriteString(errOut, msg)
			if e != nil || n != len(msg) {
				return 2
			}
			return 2
		}
		return 0
	}
	return hookOutput(out, a.format, r, code)
}

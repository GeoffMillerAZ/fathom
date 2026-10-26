//go:build harness_maintenance

package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"time"
)

type maintenanceNativePort interface {
	Stamp(context.Context) (maintenanceHostStamp, error)
	Approve(context.Context, []byte, maintenanceApprover) ([]byte, error)
	Counts() maintenanceNativeCounts
	Close() error
}
type maintenanceStorePort interface {
	Load() (*maintenanceState, error)
	Commit(maintenanceState, string) error
	Recover(maintenanceState) error
	Close() error
}
type maintenanceCLIDeps struct {
	read      func(string, int) (snapshot, error)
	newNative func(maintenanceNative) (maintenanceNativePort, error)
	newStore  func(string, func(string) error) (maintenanceStorePort, error)
	entropy   io.Reader
}

func maintenanceDefaultDeps() maintenanceCLIDeps {
	return maintenanceCLIDeps{read: readSnapshot, newNative: func(n maintenanceNative) (maintenanceNativePort, error) {
		native, err := prepareMaintenanceNative(n)
		if err != nil {
			return nil, err
		}
		return native, nil
	}, newStore: func(root string, fault func(string) error) (maintenanceStorePort, error) {
		store, err := openMaintenanceFileStore(root, fault)
		if err != nil {
			return nil, err
		}
		return store, nil
	}, entropy: rand.Reader}
}
func maintenanceUsage() string {
	return "harnessctl maintenance init|prepare|enter|check|end|recover --controller ABS --controller-sha256 HASH --project ABS [--candidate ABS for prepare] --format json|text\n"
}
func runMaintenance(args []string, out io.Writer) int {
	return runMaintenanceWithDeps(args, out, maintenanceDefaultDeps())
}
func maintenanceBaseReport(op string) maintenanceReport {
	return maintenanceReport{Version: "harnessctl.maintenance-report/1", Status: "hold", Operation: op, Code: "input_invalid", NoHostActionAuthority: true, Installation: "unobserved", Coverage: "selected-project-only"}
}
func maintenanceFail(r *maintenanceReport, code string, exit int) int {
	r.Status = "hold"
	r.Code = code
	r.Phase = ""
	return exit
}
func maintenanceSuccess(r *maintenanceReport, phase string) int {
	r.Status = "complete"
	r.Code = "ok"
	r.Phase = phase
	return 0
}
func maintenanceOutput(out io.Writer, format string, r maintenanceReport, exit int) int {
	if out == nil {
		return 2
	}
	if validateMaintenanceReport(r) != nil {
		r = maintenanceBaseReport("unknown")
		r.Code = "output_incomplete"
		exit = 2
	}
	var b []byte
	var e error
	if format == "text" {
		b, e = json.MarshalIndent(r, "", "  ")
	} else {
		b, e = json.Marshal(r)
	}
	if e != nil || len(b)+1 > 1<<20 {
		return 2
	}
	b = append(b, '\n')
	n, e := out.Write(b)
	if e != nil || n != len(b) {
		return 2
	}
	return exit
}
func maintenanceOp(s string) bool {
	return oneOf(s, "init", "prepare", "enter", "check", "end", "recover")
}

type maintenanceCLIArgs struct{ operation, controller, pin, project, candidate, format string }

func parseMaintenanceArgs(args []string) (maintenanceCLIArgs, bool) {
	var a maintenanceCLIArgs
	if len(args) == 0 || !maintenanceOp(args[0]) || len(args[1:])%2 != 0 {
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
		case "--candidate":
			a.candidate = value
		case "--format":
			a.format = value
		default:
			return a, false
		}
	}
	if !maintenanceAbs(a.controller) || !maintenanceHash(a.pin) || !maintenanceAbs(a.project) || !oneOf(a.format, "json", "text") {
		return a, false
	}
	if a.operation == "prepare" {
		if !seen["--candidate"] || !maintenanceAbs(a.candidate) {
			return a, false
		}
	} else if seen["--candidate"] {
		return a, false
	}
	return a, true
}

func maintenanceEntropy(reader io.Reader) ([]byte, error) {
	if reader == nil {
		return nil, errMaintenance
	}
	b := make([]byte, 32)
	if _, e := io.ReadFull(reader, b); e != nil {
		return nil, errMaintenance
	}
	return b, nil
}
func maintenanceNativeCounters(r *maintenanceReport, c maintenanceNativeCounts) {
	r.NativeStampAttempts = c.StampAttempts
	r.NativeStampInvoked = c.StampInvoked
	r.NativeStampCompleted = c.StampCompleted
	r.NativeApprovalAttempts = c.ApprovalAttempts
	r.NativeApprovalInvoked = c.ApprovalInvoked
	r.NativeApprovalCompleted = c.ApprovalCompleted
}
func maintenanceApplyTransition(r *maintenanceReport, store maintenanceStorePort, t maintenanceTransitionResult, op string) int {
	if t.Write {
		if t.State == nil {
			return maintenanceFail(r, "state_invalid", 2)
		}
		r.StateWritesAttempted++
		if e := store.Commit(*t.State, op); e != nil {
			return maintenanceFail(r, maintenanceStoreCode(e), 2)
		}
		r.StateWritesCommitted++
	}
	if t.Exit != 0 {
		return maintenanceFail(r, t.Code, t.Exit)
	}
	if t.State == nil {
		return maintenanceFail(r, "state_invalid", 2)
	}
	return maintenanceSuccess(r, t.State.Phase)
}
func maintenanceControllerProject(c maintenanceController, root string) (maintenanceProject, bool) {
	var selected maintenanceProject
	found := false
	for _, p := range c.Projects {
		if p.Root == root {
			if found {
				return selected, false
			}
			selected = p
			found = true
		}
	}
	return selected, found
}
func maintenancePinnedController(read func(string, int) (snapshot, error), path, pin string) (maintenanceController, snapshot, string) {
	snap, e := read(path, 2<<20)
	if e != nil {
		return maintenanceController{}, snapshot{}, "controller_invalid"
	}
	h := sha256.Sum256(snap.data)
	if hex.EncodeToString(h[:]) != pin {
		return maintenanceController{}, snapshot{}, "controller_changed"
	}
	c, e := decodeMaintenanceController(snap.data)
	if e != nil || c.Version != "harnessctl.controller/1" || !maintenanceAbs(c.Native.Executable) || !maintenanceHash(c.Native.SHA256) || !maintenanceAbs(c.Native.PreparationRoot) || c.Native.Profile != "darwin-protected-key/1" || c.Native.TimeoutMillis < 1 || c.Native.TimeoutMillis > 120000 {
		return maintenanceController{}, snapshot{}, "controller_invalid"
	}
	return c, snap, ""
}
func maintenanceBoundary(deps maintenanceCLIDeps, c maintenanceController, controllerSnap snapshot, controllerPath, controllerPin, projectRoot string, stamp maintenanceHostStamp) (maintenanceContext, string) {
	var ctx maintenanceContext
	if validateMaintenanceHostStamp(stamp) != nil {
		return ctx, "clock_invalid"
	}
	fresh, _, code := maintenancePinnedController(deps.read, controllerPath, controllerPin)
	if code != "" {
		return ctx, code
	}
	if fresh.Epoch != c.Epoch || !sameSnapshot(deps.read, controllerSnap, 2<<20) {
		return ctx, "controller_changed"
	}
	selected, ok := maintenanceControllerProject(c, projectRoot)
	if !ok || !maintenanceAbs(selected.Registration) || !maintenanceAbs(selected.StateRoot) {
		return ctx, "project_unregistered"
	}
	regSnap, e := deps.read(selected.Registration, 2<<20)
	if e != nil {
		return ctx, "config_incomplete"
	}
	var reg registration
	if strictJSON(regSnap.data, &reg) != nil {
		return ctx, "config_incomplete"
	}
	p, _, _, e := validateRegistration(reg, projectRoot, selected.Registration, maintenanceStampTime(stamp))
	if e != nil {
		return ctx, "config_incomplete"
	}
	if _, e := validateMaintenanceController(c, reg, p, selected.Registration, maintenanceStampTime(stamp)); e != nil {
		return ctx, "controller_invalid"
	}
	checkReport, exit := check(selected.Registration, projectRoot, true, seams{now: func() time.Time { return maintenanceStampTime(stamp) }, read: deps.read})
	if exit != 0 {
		if checkReport.Category == "policy" {
			return ctx, "policy_conflict"
		}
		return ctx, "config_incomplete"
	}
	if !sameSnapshot(deps.read, regSnap, 2<<20) || !sameSnapshot(deps.read, controllerSnap, 2<<20) {
		return ctx, "binding_mismatch"
	}
	h := sha256.Sum256(regSnap.data)
	if hex.EncodeToString(h[:]) != checkReport.RegistrationSHA256 || checkReport.ProjectID != p.ID {
		return ctx, "binding_mismatch"
	}
	ctx = maintenanceContext{Controller: c, ControllerEpoch: c.Epoch, ControllerSHA256: controllerPin, RegistrationEpoch: reg.Epoch, RegistrationSHA256: checkReport.RegistrationSHA256, SnapshotSHA256: checkReport.SnapshotSHA256, HostID: reg.HostID, ProjectID: p.ID, PrincipalID: p.PrincipalID, ConfigExpiresAt: checkReport.ExpiresAt, EffectiveControls: checkReport.EffectiveControls, Stamp: stamp}
	return ctx, ""
}
func runMaintenanceWithDeps(args []string, out io.Writer, deps maintenanceCLIDeps) int {
	op := "unknown"
	if len(args) > 0 && maintenanceOp(args[0]) {
		op = args[0]
	}
	r := maintenanceBaseReport(op)
	format := "json"
	var native maintenanceNativePort
	var store maintenanceStorePort
	exit := func() int {
		a, ok := parseMaintenanceArgs(args)
		if !ok {
			return maintenanceFail(&r, "input_invalid", 2)
		}
		format = a.format
		if deps.read == nil || deps.newNative == nil || deps.newStore == nil || deps.entropy == nil {
			return maintenanceFail(&r, "unsupported", 2)
		}
		c, controllerSnap, code := maintenancePinnedController(deps.read, a.controller, a.pin)
		if code != "" {
			return maintenanceFail(&r, code, 2)
		}
		selected, ok := maintenanceControllerProject(c, a.project)
		if !ok {
			return maintenanceFail(&r, "project_unregistered", 2)
		}
		if !maintenanceAbs(selected.StateRoot) || !maintenanceAbs(selected.Registration) {
			return maintenanceFail(&r, "controller_invalid", 2)
		}
		var e error
		native, e = deps.newNative(c.Native)
		if e != nil {
			return maintenanceFail(&r, "native_unavailable", 2)
		}
		stamp, e := native.Stamp(context.Background())
		if e != nil {
			return maintenanceFail(&r, "native_incomplete", 2)
		}
		ctx, code := maintenanceBoundary(deps, c, controllerSnap, a.controller, a.pin, a.project, stamp)
		if code != "" {
			return maintenanceFail(&r, code, 2)
		}
		store, e = deps.newStore(selected.StateRoot, nil)
		if e != nil {
			return maintenanceFail(&r, maintenanceStoreCode(e), 2)
		}
		stamp, e = native.Stamp(context.Background())
		if e != nil {
			return maintenanceFail(&r, "native_incomplete", 2)
		}
		ctx, code = maintenanceBoundary(deps, c, controllerSnap, a.controller, a.pin, a.project, stamp)
		if code != "" {
			return maintenanceFail(&r, code, 2)
		}
		current, loadErr := store.Load()
		if a.operation == "recover" {
			if loadErr == nil {
				if current == nil {
					return maintenanceFail(&r, "state_unavailable", 2)
				}
				return maintenanceFail(&r, "state_exists", 2)
			}
			if maintenanceStoreCode(loadErr) != "state_ambiguous" {
				return maintenanceFail(&r, maintenanceStoreCode(loadErr), 2)
			}
			entropy, e := maintenanceEntropy(deps.entropy)
			if e != nil {
				return maintenanceFail(&r, "entropy_unavailable", 2)
			}
			t := maintenanceTransition(nil, maintenanceTransitionInput{Operation: "recover", InstanceEntropy: entropy}, ctx)
			if t.Exit != 0 {
				return maintenanceFail(&r, t.Code, t.Exit)
			}
			r.StateWritesAttempted++
			if e := store.Recover(*t.State); e != nil {
				return maintenanceFail(&r, maintenanceStoreCode(e), 2)
			}
			r.StateWritesCommitted++
			actual, e := store.Load()
			if e != nil || actual == nil || actual.Phase != "locked" || validateMaintenanceState(*actual, ctx) != nil {
				return maintenanceFail(&r, "state_invalid", 2)
			}
			return maintenanceSuccess(&r, actual.Phase)
		}
		if loadErr != nil {
			return maintenanceFail(&r, maintenanceStoreCode(loadErr), 2)
		}
		switch a.operation {
		case "init":
			entropy, e := maintenanceEntropy(deps.entropy)
			if e != nil {
				return maintenanceFail(&r, "entropy_unavailable", 2)
			}
			t := maintenanceTransition(current, maintenanceTransitionInput{Operation: "init", InstanceEntropy: entropy}, ctx)
			return maintenanceApplyTransition(&r, store, t, "init")
		case "prepare":
			candidate, e := deps.read(a.candidate, 8192)
			if e != nil {
				return maintenanceFail(&r, "candidate_invalid", 2)
			}
			nonce, e := maintenanceEntropy(deps.entropy)
			if e != nil {
				return maintenanceFail(&r, "entropy_unavailable", 2)
			}
			if !sameSnapshot(deps.read, candidate, 8192) {
				return maintenanceFail(&r, "candidate_invalid", 2)
			}
			t := maintenanceTransition(current, maintenanceTransitionInput{Operation: "prepare", CandidateRaw: candidate.data, Nonce: nonce}, ctx)
			return maintenanceApplyTransition(&r, store, t, "prepare")
		case "check", "end":
			if current != nil && current.Phase == "maintenance" && validateMaintenanceState(*current, ctx) == nil {
				r.StoredSignatureVerified = true
			}
			t := maintenanceTransition(current, maintenanceTransitionInput{Operation: a.operation}, ctx)
			return maintenanceApplyTransition(&r, store, t, a.operation)
		case "enter":
			if current == nil || current.Phase != "prepared" {
				return maintenanceFail(&r, "proposal_missing", 2)
			}
			begin := maintenanceTransition(current, maintenanceTransitionInput{Operation: "begin-approval"}, ctx)
			if begin.Exit != 0 {
				return maintenanceApplyTransition(&r, store, begin, "begin-approval")
			}
			if code := maintenanceApplyTransition(&r, store, begin, "begin-approval"); code != 0 {
				return code
			}
			pending, e := store.Load()
			if e != nil || pending == nil || pending.Phase != "approval-pending" || pending.Plan == nil {
				return maintenanceFail(&r, "state_ambiguous", 2)
			}
			plan, e := canonicalMaintenancePlan(*pending.Plan)
			if e != nil {
				return maintenanceFail(&r, "state_invalid", 2)
			}
			approver, e := maintenanceApproverFor(ctx.Controller, pending.Plan.ApproverID, pending.Plan.Purpose)
			if e != nil {
				return maintenanceFail(&r, "candidate_denied", 1)
			}
			reply, e := native.Approve(context.Background(), plan, approver)
			if e != nil {
				return maintenanceFail(&r, "native_incomplete", 2)
			}
			stamp, e = native.Stamp(context.Background())
			if e != nil {
				return maintenanceFail(&r, "native_incomplete", 2)
			}
			fresh, code := maintenanceBoundary(deps, c, controllerSnap, a.controller, a.pin, a.project, stamp)
			if code != "" {
				return maintenanceFail(&r, code, 2)
			}
			finish := maintenanceTransition(pending, maintenanceTransitionInput{Operation: "finish-approval", ApprovalRaw: reply}, fresh)
			if finish.Exit == 0 {
				r.NativeSignatureVerified = true
			}
			result := maintenanceApplyTransition(&r, store, finish, "finish-approval")
			if result != 0 {
				return result
			}
			if finish.State != nil && finish.State.Phase == "maintenance" {
				loaded, e := store.Load()
				if e != nil || loaded == nil || validateMaintenanceState(*loaded, fresh) != nil {
					return maintenanceFail(&r, "state_invalid", 2)
				}
				r.StoredSignatureVerified = true
			}
			return result
		}
		return maintenanceFail(&r, "input_invalid", 2)
	}()
	if native != nil {
		maintenanceNativeCounters(&r, native.Counts())
		if e := native.Close(); e != nil {
			exit = maintenanceFail(&r, "native_incomplete", 2)
		}
	}
	if store != nil {
		if e := store.Close(); e != nil {
			exit = maintenanceFail(&r, "storage_incomplete", 2)
		}
	}
	return maintenanceOutput(out, format, r, exit)
}

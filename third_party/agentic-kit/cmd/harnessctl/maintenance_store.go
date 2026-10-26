//go:build harness_maintenance

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"sync"
)

var (
	errMaintenanceStoreBusy        = errors.New("maintenance store busy")
	errMaintenanceStoreAmbiguous   = errors.New("maintenance store ambiguous")
	errMaintenanceStoreExists      = errors.New("maintenance state exists")
	errMaintenanceStoreInvalid     = errors.New("maintenance state invalid")
	errMaintenanceStoreIncomplete  = errors.New("maintenance storage incomplete")
	errMaintenanceStoreUnsupported = errors.New("maintenance storage unsupported")
	errMaintenanceStoreBinding     = errors.New("maintenance receiver binding mismatch")
)

func maintenanceStoreCode(e error) string {
	switch {
	case e == nil:
		return "ok"
	case errors.Is(e, errMaintenanceStoreBusy):
		return "state_busy"
	case errors.Is(e, errMaintenanceStoreAmbiguous):
		return "state_ambiguous"
	case errors.Is(e, errMaintenanceStoreExists):
		return "state_exists"
	case errors.Is(e, errMaintenanceStoreInvalid):
		return "state_invalid"
	case errors.Is(e, errMaintenanceStoreBinding):
		return "binding_mismatch"
	case errors.Is(e, errMaintenanceStoreUnsupported):
		return "unsupported"
	default:
		return "storage_incomplete"
	}
}

type maintenanceReceiverBinding struct {
	Version            string `json:"version"`
	ControllerSHA256   string `json:"controllerSHA256"`
	ControllerEpoch    int    `json:"controllerEpoch"`
	RegistrationSHA256 string `json:"registrationSHA256"`
	RegistrationEpoch  int    `json:"registrationEpoch"`
	HostID             string `json:"hostId"`
	ProjectID          string `json:"projectId"`
	PrincipalID        string `json:"principalId"`
	StateRoot          string `json:"stateRoot"`
}

func maintenanceBindingFor(s maintenanceState, root string) maintenanceReceiverBinding {
	return maintenanceReceiverBinding{Version: "harnessctl.receiver-binding/1", ControllerSHA256: s.ControllerSHA256, ControllerEpoch: s.ControllerEpoch, RegistrationSHA256: s.RegistrationSHA256, RegistrationEpoch: s.RegistrationEpoch, HostID: s.HostID, ProjectID: s.ProjectID, PrincipalID: s.PrincipalID, StateRoot: root}
}
func maintenanceBindingValid(b maintenanceReceiverBinding, root string) bool {
	return b.Version == "harnessctl.receiver-binding/1" && maintenanceHash(b.ControllerSHA256) && b.ControllerEpoch >= 1 && b.ControllerEpoch <= 2147483647 && maintenanceHash(b.RegistrationSHA256) && b.RegistrationEpoch >= 1 && b.RegistrationEpoch <= 2147483647 && validID(b.HostID) && validID(b.ProjectID) && validID(b.PrincipalID) && b.StateRoot == root && maintenanceAbs(b.StateRoot)
}
func maintenanceBindingMatches(b maintenanceReceiverBinding, s maintenanceState, root string) bool {
	return maintenanceBindingValid(b, root) && b == maintenanceBindingFor(s, root)
}

type maintenanceStatePointer struct {
	Version  string `json:"version"`
	Slot     string `json:"slot"`
	Sequence uint64 `json:"sequence"`
	SHA256   string `json:"sha256"`
}
type maintenanceStateIntent struct {
	Version   string                   `json:"version"`
	From      *maintenanceStatePointer `json:"from,omitempty"`
	To        maintenanceStatePointer  `json:"to"`
	Operation string                   `json:"operation"`
}
type maintenanceRecoveryEvidence struct {
	ReplacementState string `json:"replacementState"`
	Version          string `json:"version"`
	BindingSHA256    string `json:"bindingSHA256"`
	Current          string `json:"current"`
	Pending          string `json:"pending"`
	StateA           string `json:"stateA"`
	StateB           string `json:"stateB"`
	CurrentTemp      string `json:"currentTemp"`
	PendingTemp      string `json:"pendingTemp"`
	StateATempSHA256 string `json:"stateATempSHA256"`
	StateBTempSHA256 string `json:"stateBTempSHA256"`
}
type maintenanceFileStore struct {
	mu        sync.Mutex
	rootPath  string
	root      *os.Root
	rootInfo  os.FileInfo
	lock      *os.File
	lockInfo  os.FileInfo
	fault     func(string) error
	loaded    bool
	loadedRaw []byte
	uncertain bool
	closed    bool
}

const maintenanceMaxState = 65536

var maintenanceLeaves = []string{"BINDING", "BINDING.tmp", "CURRENT", "PENDING", "state-a.json", "state-b.json", "CURRENT.tmp", "PENDING.tmp", "state-a.json.tmp", "state-b.json.tmp", "recovery.json", "recovery.json.tmp"}

func openMaintenanceFileStore(rootPath string, fault func(string) error) (*maintenanceFileStore, error) {
	if !maintenanceStorageSupported() {
		return nil, errMaintenanceStoreUnsupported
	}
	if !canonical(rootPath) {
		return nil, errMaintenanceStoreInvalid
	}
	ri, e := os.Lstat(rootPath)
	if e != nil || !ri.IsDir() || ri.Mode()&os.ModeSymlink != 0 || ri.Mode().Perm()&0077 != 0 {
		return nil, errMaintenanceStoreInvalid
	}
	r, e := os.OpenRoot(rootPath)
	if e != nil {
		return nil, errMaintenanceStoreIncomplete
	}
	s := &maintenanceFileStore{rootPath: rootPath, root: r, rootInfo: ri, fault: fault}
	fail := func(err error) (*maintenanceFileStore, error) { _ = s.Close(); return nil, err }
	if !s.sameRoot() {
		return fail(errMaintenanceStoreInvalid)
	}
	li, e := r.Lstat("LOCK")
	if e != nil && !errors.Is(e, os.ErrNotExist) {
		return fail(errMaintenanceStoreIncomplete)
	}
	if e == nil && (!li.Mode().IsRegular() || li.Size() != 0 || !maintenanceSingleLink(li)) {
		return fail(errMaintenanceStoreAmbiguous)
	}
	flags := os.O_RDWR
	if e != nil {
		flags |= os.O_CREATE | os.O_EXCL
	}
	if flags&os.O_CREATE != 0 {
		if e := s.faultAt("before:LOCK:create"); e != nil {
			return fail(e)
		}
	}
	f, e := r.OpenFile("LOCK", flags, 0600)
	if e != nil {
		return fail(errMaintenanceStoreIncomplete)
	}
	s.lock = f
	if flags&os.O_CREATE != 0 {
		if e := s.faultAt("after:LOCK:create"); e != nil {
			return fail(e)
		}
		if e := s.faultAt("before:LOCK:sync"); e != nil {
			return fail(e)
		}
		if f.Sync() != nil {
			return fail(errMaintenanceStoreIncomplete)
		}
		if e := s.faultAt("after:LOCK:sync"); e != nil {
			return fail(e)
		}
		if e := s.syncDir(); e != nil {
			return fail(e)
		}
	}
	fi, e := f.Stat()
	if e != nil || !fi.Mode().IsRegular() || fi.Size() != 0 || !maintenanceSingleLink(fi) {
		return fail(errMaintenanceStoreAmbiguous)
	}
	s.lockInfo = fi
	if e := maintenanceTryLock(f); e != nil {
		return fail(e)
	}
	if !s.sameRoot() || !s.sameLock() {
		return fail(errMaintenanceStoreAmbiguous)
	}
	return s, nil
}
func (s *maintenanceFileStore) sameRoot() bool {
	if s == nil || s.root == nil {
		return false
	}
	a, e := os.Lstat(s.rootPath)
	if e != nil || !a.IsDir() || a.Mode()&os.ModeSymlink != 0 || !os.SameFile(a, s.rootInfo) {
		return false
	}
	b, e := s.root.Stat(".")
	return e == nil && os.SameFile(b, s.rootInfo)
}
func (s *maintenanceFileStore) sameLock() bool {
	if s.lock == nil {
		return false
	}
	a, e := s.root.Lstat("LOCK")
	if e != nil || !a.Mode().IsRegular() || a.Size() != 0 || !maintenanceSingleLink(a) || !os.SameFile(a, s.lockInfo) {
		return false
	}
	b, e := s.lock.Stat()
	return e == nil && os.SameFile(b, s.lockInfo)
}
func (s *maintenanceFileStore) ensure() error {
	if s == nil || s.closed || s.root == nil || s.lock == nil {
		return errMaintenanceStoreInvalid
	}
	if !s.sameRoot() || !s.sameLock() {
		return errMaintenanceStoreAmbiguous
	}
	if s.uncertain {
		return errMaintenanceStoreAmbiguous
	}
	return nil
}
func (s *maintenanceFileStore) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	var bad bool
	if s.lock != nil {
		if maintenanceUnlock(s.lock) != nil {
			bad = true
		}
		if s.lock.Close() != nil {
			bad = true
		}
	}
	if s.root != nil && s.root.Close() != nil {
		bad = true
	}
	if bad {
		return errMaintenanceStoreIncomplete
	}
	return nil
}
func (s *maintenanceFileStore) faultAt(where string) error {
	if s.fault != nil && s.fault(where) != nil {
		s.uncertain = true
		return errMaintenanceStoreIncomplete
	}
	return nil
}
func (s *maintenanceFileStore) exists(leaf string) (bool, error) {
	st, e := s.root.Lstat(leaf)
	if errors.Is(e, os.ErrNotExist) {
		return false, nil
	}
	if e != nil {
		return false, errMaintenanceStoreIncomplete
	}
	if !st.Mode().IsRegular() || !maintenanceSingleLink(st) {
		return false, errMaintenanceStoreAmbiguous
	}
	return true, nil
}
func (s *maintenanceFileStore) readBinding() (maintenanceReceiverBinding, []byte, bool, error) {
	raw, p, e := s.readLeaf("BINDING", 4096)
	if e != nil {
		return maintenanceReceiverBinding{}, nil, false, e
	}
	if !p {
		return maintenanceReceiverBinding{}, nil, false, nil
	}
	var b maintenanceReceiverBinding
	if strictJSON(raw, &b) != nil || !maintenanceBindingValid(b, s.rootPath) {
		return b, nil, false, errMaintenanceStoreAmbiguous
	}
	return b, raw, true, nil
}
func (s *maintenanceFileStore) readLeaf(leaf string, max int) ([]byte, bool, error) {
	present, e := s.exists(leaf)
	if e != nil || !present {
		return nil, present, e
	}
	before, e := s.root.Lstat(leaf)
	if e != nil {
		return nil, false, errMaintenanceStoreAmbiguous
	}
	f, e := s.root.Open(leaf)
	if e != nil {
		return nil, false, errMaintenanceStoreAmbiguous
	}
	defer f.Close()
	after, e := f.Stat()
	if e != nil || !after.Mode().IsRegular() || !maintenanceSingleLink(after) || !os.SameFile(before, after) {
		return nil, false, errMaintenanceStoreAmbiguous
	}
	raw, e := io.ReadAll(io.LimitReader(f, int64(max)+1))
	if e != nil || len(raw) > max {
		return nil, false, errMaintenanceStoreAmbiguous
	}
	return raw, true, nil
}
func (s *maintenanceFileStore) syncDir() error {
	if e := s.faultAt("before:DIR:sync"); e != nil {
		return e
	}
	f, e := s.root.Open(".")
	if e != nil {
		return errMaintenanceStoreIncomplete
	}
	e = f.Sync()
	_ = f.Close()
	if e != nil {
		return errMaintenanceStoreIncomplete
	}
	return s.faultAt("after:DIR:sync")
}
func (s *maintenanceFileStore) writeTemp(leaf string, raw []byte) error {
	if e := s.faultAt("before:" + leaf + ":write"); e != nil {
		return e
	}
	f, e := s.root.OpenFile(leaf, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		return errMaintenanceStoreAmbiguous
	}
	n, e := f.Write(raw)
	if e != nil || n != len(raw) {
		_ = f.Close()
		return errMaintenanceStoreIncomplete
	}
	if e := s.faultAt("after:" + leaf + ":write"); e != nil {
		_ = f.Close()
		return e
	}
	if e := s.faultAt("before:" + leaf + ":sync"); e != nil {
		_ = f.Close()
		return e
	}
	if e = f.Sync(); e != nil {
		_ = f.Close()
		return errMaintenanceStoreIncomplete
	}
	if e := s.faultAt("after:" + leaf + ":sync"); e != nil {
		_ = f.Close()
		return e
	}
	if f.Close() != nil {
		return errMaintenanceStoreIncomplete
	}
	return nil
}
func (s *maintenanceFileStore) rename(src, dst string) error {
	if e := s.faultAt("before:" + dst + ":rename"); e != nil {
		return e
	}
	if e := s.root.Rename(src, dst); e != nil {
		return errMaintenanceStoreIncomplete
	}
	return s.faultAt("after:" + dst + ":rename")
}
func (s *maintenanceFileStore) remove(leaf string) error {
	if e := s.faultAt("before:" + leaf + ":remove"); e != nil {
		return e
	}
	if e := s.root.Remove(leaf); e != nil {
		return errMaintenanceStoreIncomplete
	}
	return s.faultAt("after:" + leaf + ":remove")
}
func (s *maintenanceFileStore) replace(leaf string, raw []byte) error {
	temp := leaf + ".tmp"
	if e := s.writeTemp(temp, raw); e != nil {
		return e
	}
	if e := s.rename(temp, leaf); e != nil {
		return e
	}
	return s.syncDir()
}
func maintenanceLockedStateOnly(s maintenanceState) bool {
	return s.Phase == "locked" && s.Plan == nil && s.PendingOrigin == nil && s.CandidatePayload == nil && s.Lease == nil
}
func maintenanceEvidenceEncode(b []byte) string {
	if len(b) == 0 {
		return "."
	}
	return base64.StdEncoding.EncodeToString(b)
}
func maintenanceEvidenceDecode(encoded string) ([]byte, bool) {
	if encoded == "." {
		return []byte{}, true
	}
	b, e := base64.StdEncoding.DecodeString(encoded)
	return b, e == nil && base64.StdEncoding.EncodeToString(b) == encoded
}
func maintenanceRecoveryValid(v maintenanceRecoveryEvidence) bool {
	if v.Version != "harnessctl.recovery-evidence/1" || !maintenanceHash(v.BindingSHA256) {
		return false
	}
	replacement, e := base64.StdEncoding.DecodeString(v.ReplacementState)
	if e != nil || len(replacement) == 0 || len(replacement) > maintenanceMaxState || base64.StdEncoding.EncodeToString(replacement) != v.ReplacementState {
		return false
	}
	locked, e := decodeMaintenanceState(replacement)
	if e != nil || !maintenanceLockedStateOnly(locked) {
		return false
	}
	any := false
	for _, item := range []struct {
		encoded string
		max     int
	}{{v.Current, 4096}, {v.Pending, 8192}, {v.StateA, maintenanceMaxState}, {v.StateB, maintenanceMaxState}, {v.CurrentTemp, 4096}, {v.PendingTemp, 8192}} {
		if item.encoded == "" {
			continue
		}
		any = true
		b, ok := maintenanceEvidenceDecode(item.encoded)
		if !ok || len(b) > item.max {
			return false
		}
	}
	for _, hash := range []string{v.StateATempSHA256, v.StateBTempSHA256} {
		if hash != "" {
			any = true
			if !maintenanceHash(hash) {
				return false
			}
		}
	}
	return any || maintenanceHash(v.BindingSHA256)
}
func maintenancePointerValid(p maintenanceStatePointer) bool {
	return p.Version == "harnessctl.state-pointer/1" && (p.Slot == "a" || p.Slot == "b") && p.Sequence >= 1 && maintenanceHash(p.SHA256)
}
func maintenanceStoreOperation(op string) bool {
	return oneOf(op, "init", "prepare", "begin-approval", "finish-approval", "enter", "check", "end", "lock", "expire")
}
func maintenanceSlot(p maintenanceStatePointer) string {
	if p.Slot == "a" {
		return "state-a.json"
	}
	return "state-b.json"
}
func (s *maintenanceFileStore) loadLocked() (*maintenanceState, []byte, error) {
	if e := s.ensure(); e != nil {
		return nil, nil, e
	}
	binding, _, bound, e := s.readBinding()
	if e != nil {
		return nil, nil, e
	}
	bindingTemp, e := s.exists("BINDING.tmp")
	if e != nil {
		return nil, nil, e
	}
	if bindingTemp {
		return nil, nil, errMaintenanceStoreAmbiguous
	}
	if !bound {
		for _, leaf := range maintenanceLeaves {
			if leaf == "BINDING" || leaf == "BINDING.tmp" {
				continue
			}
			present, e := s.exists(leaf)
			if e != nil {
				return nil, nil, e
			}
			if present {
				return nil, nil, errMaintenanceStoreAmbiguous
			}
		}
		return nil, nil, nil
	}
	for _, leaf := range []string{"PENDING", "CURRENT.tmp", "PENDING.tmp", "state-a.json.tmp", "state-b.json.tmp", "recovery.json.tmp"} {
		present, e := s.exists(leaf)
		if e != nil {
			return nil, nil, e
		}
		if present {
			return nil, nil, errMaintenanceStoreAmbiguous
		}
	}
	current, present, e := s.readLeaf("CURRENT", 4096)
	if e != nil {
		return nil, nil, e
	}
	if !present {
		return nil, nil, errMaintenanceStoreAmbiguous
	}
	var p maintenanceStatePointer
	if strictJSON(current, &p) != nil || !maintenancePointerValid(p) {
		return nil, nil, errMaintenanceStoreAmbiguous
	}
	slot, ok, e := s.readLeaf(maintenanceSlot(p), maintenanceMaxState)
	if e != nil || !ok {
		return nil, nil, errMaintenanceStoreAmbiguous
	}
	h := sha256.Sum256(slot)
	if hex.EncodeToString(h[:]) != p.SHA256 {
		return nil, nil, errMaintenanceStoreAmbiguous
	}
	st, e := decodeMaintenanceState(slot)
	if e != nil || st.Sequence != p.Sequence || !maintenanceBindingMatches(binding, st, s.rootPath) {
		return nil, nil, errMaintenanceStoreAmbiguous
	}
	other := "state-a.json"
	if p.Slot == "a" {
		other = "state-b.json"
	}
	oldRaw, exists, e := s.readLeaf(other, maintenanceMaxState)
	if e != nil {
		return nil, nil, e
	}
	if exists {
		old, e := decodeMaintenanceState(oldRaw)
		if e != nil || old.Sequence >= p.Sequence || old.InstanceID != st.InstanceID || !maintenanceBindingMatches(binding, old, s.rootPath) {
			return nil, nil, errMaintenanceStoreAmbiguous
		}
	}
	evidence, present, e := s.readLeaf("recovery.json", 256<<10)
	if e != nil {
		return nil, nil, e
	}
	if present {
		var v maintenanceRecoveryEvidence
		if strictJSON(evidence, &v) != nil || !maintenanceRecoveryValid(v) {
			return nil, nil, errMaintenanceStoreAmbiguous
		}
	}
	return &st, current, nil
}
func (s *maintenanceFileStore) Load() (*maintenanceState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, raw, e := s.loadLocked()
	if e != nil {
		s.loaded = false
		return nil, e
	}
	s.loaded = true
	s.loadedRaw = append([]byte(nil), raw...)
	return st, nil
}
func maintenanceStateBytes(next maintenanceState) ([]byte, error) {
	raw, e := json.Marshal(next)
	if e != nil || len(raw) > maintenanceMaxState || len(raw) == 0 {
		return nil, errMaintenanceStoreInvalid
	}
	if _, e := decodeMaintenanceState(raw); e != nil {
		return nil, errMaintenanceStoreInvalid
	}
	return raw, nil
}
func maintenancePointerBytes(p maintenanceStatePointer) ([]byte, error) {
	raw, e := json.Marshal(p)
	if e != nil || len(raw) > 4096 {
		return nil, errMaintenanceStoreInvalid
	}
	return raw, nil
}
func (s *maintenanceFileStore) Commit(next maintenanceState, operation string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.ensure(); e != nil {
		return e
	}
	if !maintenanceStoreOperation(operation) {
		return errMaintenanceStoreInvalid
	}
	raw, e := maintenanceStateBytes(next)
	if e != nil {
		return e
	}
	binding := maintenanceBindingFor(next, s.rootPath)
	if !maintenanceBindingValid(binding, s.rootPath) {
		return errMaintenanceStoreInvalid
	}
	bindingRaw, e := json.Marshal(binding)
	if e != nil || len(bindingRaw) > 4096 {
		return errMaintenanceStoreInvalid
	}
	current, oldRaw, e := s.loadLocked()
	if e != nil {
		return e
	}
	if operation == "init" {
		if current != nil || oldRaw != nil {
			return errMaintenanceStoreExists
		}
		for _, leaf := range maintenanceLeaves {
			present, e := s.exists(leaf)
			if e != nil {
				return e
			}
			if present {
				return errMaintenanceStoreExists
			}
		}
		if next.Phase != "armed" || next.Sequence != 1 || next.AuthorizationEpoch != 1 {
			return errMaintenanceStoreInvalid
		}
	} else {
		prior, _, present, e := s.readBinding()
		if e != nil {
			return e
		}
		if !present || !maintenanceBindingMatches(prior, next, s.rootPath) {
			return errMaintenanceStoreBinding
		}
		if !s.loaded || current == nil || !bytes.Equal(oldRaw, s.loadedRaw) || current.Sequence == ^uint64(0) || next.Sequence != current.Sequence+1 || next.InstanceID != current.InstanceID {
			return errMaintenanceStoreInvalid
		}
	}
	slotName := "a"
	var from *maintenanceStatePointer
	if current != nil {
		var prior maintenanceStatePointer
		_ = strictJSON(oldRaw, &prior)
		from = &prior
		if prior.Slot == "a" {
			slotName = "b"
		}
	}
	h := sha256.Sum256(raw)
	to := maintenanceStatePointer{Version: "harnessctl.state-pointer/1", Slot: slotName, Sequence: next.Sequence, SHA256: hex.EncodeToString(h[:])}
	toRaw, e := maintenancePointerBytes(to)
	if e != nil {
		return e
	}
	intent := maintenanceStateIntent{Version: "harnessctl.state-intent/1", From: from, To: to, Operation: operation}
	intentRaw, e := json.Marshal(intent)
	if e != nil || len(intentRaw) > 8192 {
		return errMaintenanceStoreInvalid
	}
	s.uncertain = true // Any failure after the first write requires inspection or recovery.
	if operation == "init" {
		if e := s.replace("BINDING", bindingRaw); e != nil {
			return e
		}
	}
	if e := s.replace("PENDING", intentRaw); e != nil {
		return e
	}
	if e := s.replace(maintenanceSlot(to), raw); e != nil {
		return e
	}
	if e := s.replace("CURRENT", toRaw); e != nil {
		return e
	}
	if e := s.remove("PENDING"); e != nil {
		return e
	}
	if e := s.syncDir(); e != nil {
		return e
	}
	s.uncertain = false
	s.loaded = true
	s.loadedRaw = append([]byte(nil), toRaw...)
	return nil
}
func maintenanceRecoverySurvivor(ev maintenanceRecoveryEvidence, name string, b, replacement []byte) bool {
	orig := ""
	switch name {
	case "CURRENT":
		orig = ev.Current
	case "PENDING":
		orig = ev.Pending
	case "state-a.json":
		orig = ev.StateA
	case "state-b.json":
		orig = ev.StateB
	case "CURRENT.tmp":
		orig = ev.CurrentTemp
	case "PENDING.tmp":
		orig = ev.PendingTemp
	case "state-a.json.tmp":
		orig = ev.StateATempSHA256
	case "state-b.json.tmp":
		orig = ev.StateBTempSHA256
	}
	if orig != "" {
		if name == "state-a.json.tmp" || name == "state-b.json.tmp" {
			h := sha256.Sum256(b)
			if hex.EncodeToString(h[:]) == orig {
				return true
			}
		} else {
			old, ok := maintenanceEvidenceDecode(orig)
			if ok && bytes.Equal(old, b) {
				return true
			}
		}
	}
	if name == "state-a.json" || name == "state-a.json.tmp" {
		return bytes.Equal(b, replacement)
	}
	if name == "CURRENT" || name == "CURRENT.tmp" {
		var p maintenanceStatePointer
		if strictJSON(b, &p) != nil || !maintenancePointerValid(p) || p.Slot != "a" {
			return false
		}
		h := sha256.Sum256(replacement)
		state, e := decodeMaintenanceState(replacement)
		return e == nil && p.Sequence == state.Sequence && p.SHA256 == hex.EncodeToString(h[:])
	}
	return false
}
func (s *maintenanceFileStore) bindingForRecovery(next maintenanceState) ([]byte, error) {
	binding, raw, present, e := s.readBinding()
	if e != nil {
		return nil, e
	}
	temp, tp, e := s.readLeaf("BINDING.tmp", 4096)
	if e != nil {
		return nil, e
	}
	if present && tp {
		return nil, errMaintenanceStoreAmbiguous
	}
	if !present {
		if !tp {
			return nil, errMaintenanceStoreAmbiguous
		}
		if strictJSON(temp, &binding) != nil || !maintenanceBindingMatches(binding, next, s.rootPath) {
			return nil, errMaintenanceStoreBinding
		}
		s.uncertain = true
		if e := s.rename("BINDING.tmp", "BINDING"); e != nil {
			return nil, e
		}
		if e := s.syncDir(); e != nil {
			return nil, e
		}
		s.uncertain = false
		raw = temp
	}
	if !maintenanceBindingMatches(binding, next, s.rootPath) {
		return nil, errMaintenanceStoreBinding
	}
	return raw, nil
}
func (s *maintenanceFileStore) Recover(next maintenanceState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s == nil || s.closed || !s.sameRoot() || !s.sameLock() {
		return errMaintenanceStoreAmbiguous
	}
	if !maintenanceLockedStateOnly(next) || next.Sequence < 1 || next.AuthorizationEpoch < 1 {
		return errMaintenanceStoreInvalid
	}
	raw, e := maintenanceStateBytes(next)
	if e != nil {
		return e
	}
	bindingRaw, e := s.bindingForRecovery(next)
	if e != nil {
		return e
	}
	bindingHash := sha256.Sum256(bindingRaw)
	existingRecovery, exists, e := s.readLeaf("recovery.json", 256<<10)
	if e != nil {
		return e
	}
	if exists {
		extra, e := s.exists("recovery.json.tmp")
		if e != nil {
			return e
		}
		if extra {
			return errMaintenanceStoreAmbiguous
		}
	}
	if !exists {
		temp, p, e := s.readLeaf("recovery.json.tmp", 256<<10)
		if e != nil {
			return e
		}
		if p {
			var prior maintenanceRecoveryEvidence
			if strictJSON(temp, &prior) == nil && maintenanceRecoveryValid(prior) {
				s.uncertain = true
				if e := s.rename("recovery.json.tmp", "recovery.json"); e != nil {
					return e
				}
				if e := s.syncDir(); e != nil {
					return e
				}
				existingRecovery = temp
				exists = true
				s.uncertain = false
			} else {
				s.uncertain = true
				if e := s.remove("recovery.json.tmp"); e != nil {
					return e
				}
				if e := s.syncDir(); e != nil {
					return e
				}
				s.uncertain = false
			}
		}
	}
	evidence := maintenanceRecoveryEvidence{Version: "harnessctl.recovery-evidence/1", BindingSHA256: hex.EncodeToString(bindingHash[:]), ReplacementState: base64.StdEncoding.EncodeToString(raw)}
	if exists {
		if strictJSON(existingRecovery, &evidence) != nil || !maintenanceRecoveryValid(evidence) || evidence.BindingSHA256 != hex.EncodeToString(bindingHash[:]) {
			return errMaintenanceStoreAmbiguous
		}
		oldRaw, e := base64.StdEncoding.DecodeString(evidence.ReplacementState)
		if e != nil {
			return errMaintenanceStoreAmbiguous
		}
		old, e := decodeMaintenanceState(oldRaw)
		if e != nil || old.ControllerEpoch != next.ControllerEpoch || old.ControllerSHA256 != next.ControllerSHA256 || old.RegistrationEpoch != next.RegistrationEpoch || old.RegistrationSHA256 != next.RegistrationSHA256 || old.SnapshotSHA256 != next.SnapshotSHA256 || old.HostID != next.HostID || old.ProjectID != next.ProjectID || old.PrincipalID != next.PrincipalID {
			return errMaintenanceStoreAmbiguous
		}
		raw = oldRaw
		next = old
	}

	fields := []struct {
		name   string
		target *string
		limit  int
	}{{"CURRENT", &evidence.Current, 4096}, {"PENDING", &evidence.Pending, 8192}, {"state-a.json", &evidence.StateA, maintenanceMaxState}, {"state-b.json", &evidence.StateB, maintenanceMaxState}, {"CURRENT.tmp", &evidence.CurrentTemp, 4096}, {"PENDING.tmp", &evidence.PendingTemp, 8192}, {"state-a.json.tmp", &evidence.StateATempSHA256, maintenanceMaxState}, {"state-b.json.tmp", &evidence.StateBTempSHA256, maintenanceMaxState}}
	presentAny := false
	for _, f := range fields {
		b, p, e := s.readLeaf(f.name, f.limit)
		if e != nil {
			return e
		}
		if !p {
			continue
		}
		presentAny = true
		if exists {
			if !maintenanceRecoverySurvivor(evidence, f.name, b, raw) {
				return errMaintenanceStoreAmbiguous
			}
			continue
		}
		if f.name == "state-a.json.tmp" || f.name == "state-b.json.tmp" {
			h := sha256.Sum256(b)
			*f.target = hex.EncodeToString(h[:])
		} else {
			*f.target = maintenanceEvidenceEncode(b)
		}
	}
	if !presentAny && !exists { /* BINDING alone is an interrupted init. */
	}
	for _, encoded := range []string{evidence.StateA, evidence.StateB} {
		if encoded == "" {
			continue
		}
		rawOld, _ := maintenanceEvidenceDecode(encoded)
		old, e := decodeMaintenanceState(rawOld)
		if e == nil && old.InstanceID == next.InstanceID {
			return errMaintenanceStoreInvalid
		}
	}
	if current, _, e := s.loadLocked(); e == nil && current != nil {
		return errMaintenanceStoreInvalid
	}
	if !exists {
		evidenceRaw, e := json.Marshal(evidence)
		if e != nil || len(evidenceRaw) > 256<<10 {
			return errMaintenanceStoreAmbiguous
		}
		s.uncertain = true
		if e := s.replace("recovery.json", evidenceRaw); e != nil {
			return e
		}
	} else {
		s.uncertain = true
	}
	for _, f := range fields {
		present, e := s.exists(f.name)
		if e != nil {
			return e
		}
		if present {
			if e := s.remove(f.name); e != nil {
				return e
			}
			if e := s.syncDir(); e != nil {
				return e
			}
		}
	}
	h := sha256.Sum256(raw)
	to := maintenanceStatePointer{Version: "harnessctl.state-pointer/1", Slot: "a", Sequence: next.Sequence, SHA256: hex.EncodeToString(h[:])}
	toRaw, _ := maintenancePointerBytes(to)
	if e := s.replace("state-a.json", raw); e != nil {
		return e
	}
	if e := s.replace("CURRENT", toRaw); e != nil {
		return e
	}
	s.uncertain = false
	s.loaded = true
	s.loadedRaw = append([]byte(nil), toRaw...)
	return nil
}

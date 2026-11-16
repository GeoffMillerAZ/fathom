// Package benchledger defines pure benchmark cell identity and scored projections.
package benchledger

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/geoffmilleraz/fathom/internal/contextbench"
	"github.com/geoffmilleraz/fathom/internal/scenariobench"
	"io"
	"math"
	"strings"
)

type TaskFamily string

const (
	FamilyPLAN TaskFamily = "PLAN"
	FamilyHAZ  TaskFamily = "HAZ"
	FamilyFIX  TaskFamily = "FIX"
	FamilyLOC  TaskFamily = "LOC"
	FamilyREV  TaskFamily = "REV"
	FamilyLANE TaskFamily = "LANE"
	FamilyASK  TaskFamily = "ASK"
)

type CellInputs struct {
	Base             contextbench.CellInputs `json:"base"`
	SubjectCommit    string                  `json:"subject_commit"`
	ToolSchemaSHA256 string                  `json:"tool_schema_sha256"`
	ProfileSHA256    string                  `json:"profile_sha256"`
	Family           TaskFamily              `json:"family"`
	CriterionID      string                  `json:"criterion_id"`
	Trial            int                     `json:"trial"`
}
type RefusalError struct {
	Reason string `json:"reason"`
}

func (e *RefusalError) Error() string { return e.Reason }

type LimitError struct {
	Limit    int64 `json:"limit"`
	Observed int64 `json:"observed"`
}

func (e *LimitError) Error() string { return "cell ledger limit exceeded" }

type RungVerdict string

const (
	RungPass          RungVerdict = "pass"
	RungFail          RungVerdict = "fail"
	RungUnknown       RungVerdict = "unknown"
	RungNotApplicable RungVerdict = "not-applicable"
)

type PreflightReceipt struct {
	SessionID      string      `json:"session_id"`
	ProductCommit  string      `json:"product_commit"`
	SubjectCommit  string      `json:"subject_commit"`
	ProfileSHA256  string      `json:"profile_sha256"`
	BeforeAgent    bool        `json:"before_agent"`
	W0             RungVerdict `json:"w0"`
	W1             RungVerdict `json:"w1"`
	W2             RungVerdict `json:"w2"`
	W3             RungVerdict `json:"w3"`
	W6             RungVerdict `json:"w6"`
	Reason         string      `json:"reason"`
	EvidenceSHA256 string      `json:"evidence_sha256"`
}
type PreflightDisposition string

const (
	PreflightAdmitted PreflightDisposition = "admitted"
	PreflightVoid     PreflightDisposition = "void"
	PreflightUnknown  PreflightDisposition = "unknown"
)

type PairPreflight struct {
	PairID           string               `json:"pair_id"`
	TreatmentSession string               `json:"treatment_session"`
	ControlSession   string               `json:"control_session"`
	BeforeAgent      bool                 `json:"before_agent"`
	Valid            bool                 `json:"valid"`
	Disposition      PreflightDisposition `json:"disposition"`
	Reason           string               `json:"reason"`
}
type CellState string

const (
	CellScored  CellState = "scored"
	CellVoid    CellState = "void"
	CellUnknown CellState = "unknown"
)

type ScoredCell struct {
	TaskID       string    `json:"task_id"`
	AssignedArm  string    `json:"assigned_arm"`
	Trial        int       `json:"trial"`
	OraclePassed bool      `json:"oracle_passed"`
	FailToPassOK bool      `json:"fail_to_pass_ok"`
	PassToPassOK bool      `json:"pass_to_pass_ok"`
	Success      bool      `json:"success"`
	SurfaceCalls int       `json:"surface_calls"`
	TotalTokens  int64     `json:"total_tokens"`
	State        CellState `json:"state"`
}

func refuse(format string, args ...any) error { return &RefusalError{fmt.Sprintf(format, args...)} }
func digest(s string, n int) bool {
	if len(s) != n {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}
func scalar(s string) bool { return !strings.ContainsAny(s, "\r\n") }

// CellKey binds only actual inputs. A daemon-free control declares all three
// treatment fields empty, so changing a separate treatment cannot change its key.
func CellKey(in CellInputs) (string, error) {
	if err := in.Base.Validate(); err != nil {
		return "", refuse("base inputs: %v", err)
	}
	for _, s := range []string{in.Base.CorpusSHA256, in.Base.TaskID, in.Base.Arm, in.Base.ModelID, in.Base.HarnessVersion, in.Base.Scoring, in.Base.PromptSubject, in.Base.ArmSurface, in.CriterionID} {
		if !scalar(s) {
			return "", refuse("cell inputs contain CR/LF")
		}
	}
	if in.Trial < 1 || !digest(in.SubjectCommit, 40) || in.CriterionID == "" {
		return "", refuse("cell requires trial >= 1, full subject commit and criterion")
	}
	switch in.Family {
	case FamilyPLAN, FamilyHAZ, FamilyFIX, FamilyLOC, FamilyREV, FamilyLANE, FamilyASK:
	default:
		return "", refuse("unknown task family %q", in.Family)
	}
	if in.Base.ArmSurface == "" {
		if in.ToolSchemaSHA256 != "" || in.ProfileSHA256 != "" {
			return "", refuse("daemon-free control has treatment schema/profile")
		}
	} else if !digest(in.ToolSchemaSHA256, 64) || !digest(in.ProfileSHA256, 64) {
		return "", refuse("treatment requires schema/profile SHA256")
	}
	wire := struct {
		BaseKey          string     `json:"base_key"`
		SubjectCommit    string     `json:"subject_commit"`
		ToolSchemaSHA256 string     `json:"tool_schema_sha256"`
		ProfileSHA256    string     `json:"profile_sha256"`
		Family           TaskFamily `json:"family"`
		CriterionID      string     `json:"criterion_id"`
		Trial            int        `json:"trial"`
	}{contextbench.CellKey(in.Base), in.SubjectCommit, in.ToolSchemaSHA256, in.ProfileSHA256, in.Family, in.CriterionID, in.Trial}
	raw, err := json.Marshal(wire)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

// AdmitPair checks local shape and classifies scripted receipts. Its caller MUST
// first verify artifact bytes/digests, expected pins, frozen applicability,
// channel absence and every active check. A digest's presence is not verification.
// That adapter is an external B2 boundary, not implemented by this leaf. N/A is
// an adapter-attested exemption, never a pass and never permitted for W0/W6.
func AdmitPair(id string, t, c PreflightReceipt) (PairPreflight, error) {
	p := PairPreflight{PairID: id, TreatmentSession: t.SessionID, ControlSession: c.SessionID, BeforeAgent: t.BeforeAgent && c.BeforeAgent, Disposition: PreflightUnknown}
	if id == "" || !scalar(id) {
		return PairPreflight{}, refuse("pair id required without CR/LF")
	}
	missing := false
	for _, r := range []PreflightReceipt{t, c} {
		for _, s := range []string{r.SessionID, r.ProductCommit, r.SubjectCommit, r.ProfileSHA256, r.Reason} {
			if !scalar(s) {
				return PairPreflight{}, refuse("preflight scalar contains CR/LF")
			}
		}
		for _, pin := range []struct {
			s string
			n int
		}{{r.ProductCommit, 40}, {r.SubjectCommit, 40}, {r.ProfileSHA256, 64}, {r.EvidenceSHA256, 64}} {
			if pin.s != "" && !digest(pin.s, pin.n) {
				return PairPreflight{}, refuse("malformed preflight identity/digest")
			}
		}
		if r.W0 == RungNotApplicable || r.W6 == RungNotApplicable {
			return PairPreflight{}, refuse("W0/W6 cannot be not-applicable")
		}
		for _, v := range []RungVerdict{r.W0, r.W1, r.W2, r.W3, r.W6} {
			switch v {
			case "", RungPass, RungFail, RungUnknown, RungNotApplicable:
			default:
				return PairPreflight{}, refuse("unknown rung verdict %q", v)
			}
		}
		if r.SessionID == "" || r.SubjectCommit == "" || r.EvidenceSHA256 == "" || !r.BeforeAgent {
			missing = true
		}
	}
	if t.SessionID != "" && t.SessionID == c.SessionID {
		return PairPreflight{}, refuse("pair sessions must differ")
	}
	if t.SubjectCommit != "" && c.SubjectCommit != "" && t.SubjectCommit != c.SubjectCommit {
		return PairPreflight{}, refuse("pair subject commits differ")
	}
	// A treatment identity is required; baseline control product/profile may be N/A.
	if t.ProductCommit == "" || t.ProfileSHA256 == "" {
		missing = true
	}
	if missing {
		p.Reason = "missing or late preflight evidence"
		return p, nil
	}
	unknown := false
	for _, r := range []PreflightReceipt{t, c} {
		for _, v := range []RungVerdict{r.W0, r.W1, r.W2, r.W3, r.W6} {
			if v == RungFail {
				p.Disposition = PreflightVoid
				p.Reason = "scripted pre-agent failure"
				return p, nil
			}
			if v == "" || v == RungUnknown {
				unknown = true
			}
		}
	}
	if unknown {
		p.Reason = "unknown preflight check"
		return p, nil
	}
	p.Disposition = PreflightAdmitted
	p.Valid = true
	p.Reason = "applicable scripted checks passed"
	return p, nil
}

func pairState(p PairPreflight) (CellState, error) {
	if p == (PairPreflight{}) {
		return CellUnknown, nil
	}
	for _, s := range []string{p.PairID, p.TreatmentSession, p.ControlSession, p.Reason} {
		if !scalar(s) {
			return "", refuse("pair scalar contains CR/LF")
		}
	}
	if p.Valid != (p.Disposition == PreflightAdmitted) {
		return "", refuse("preflight valid/disposition mismatch")
	}
	switch p.Disposition {
	case PreflightAdmitted, PreflightVoid:
		if !p.BeforeAgent || p.PairID == "" || p.TreatmentSession == "" || p.ControlSession == "" || p.TreatmentSession == p.ControlSession {
			return "", refuse("admitted/void pair requires distinct pre-agent sessions and pair id")
		}
		if p.Disposition == PreflightVoid {
			return CellVoid, nil
		}
		return CellScored, nil
	case PreflightUnknown:
		return CellUnknown, nil
	default:
		return "", refuse("unknown preflight disposition %q", p.Disposition)
	}
}

// ProjectCell persists the explicit allowlist only. No telemetry or artifact
// content can enter the result, and observed use never affects admission.
func ProjectCell(in scenariobench.CellResult, arm string, p PairPreflight) (ScoredCell, error) {
	state, err := pairState(p)
	if err != nil {
		return ScoredCell{}, err
	}
	out := ScoredCell{TaskID: in.TaskID, AssignedArm: arm, Trial: in.Trial, OraclePassed: in.Oracle.Passed, FailToPassOK: in.Oracle.FailToPassOK, PassToPassOK: in.Oracle.PassToPassOK, Success: in.Success, SurfaceCalls: in.Readings.SurfaceCalls, State: state}
	if in.Arm != arm {
		return ScoredCell{}, refuse("cell arm differs from frozen assignment")
	}
	for _, s := range in.Sessions {
		if s.TotalTokens < 0 {
			return ScoredCell{}, refuse("negative session total tokens")
		}
		if out.TotalTokens > math.MaxInt64-s.TotalTokens {
			return ScoredCell{}, &LimitError{Limit: math.MaxInt64 - out.TotalTokens, Observed: s.TotalTokens}
		}
		out.TotalTokens += s.TotalTokens
	}
	if err := validateScore(out); err != nil {
		return ScoredCell{}, err
	}
	return out, nil
}
func validateScore(out ScoredCell) error {
	if out.TaskID == "" || out.AssignedArm == "" || !scalar(out.TaskID) || !scalar(out.AssignedArm) || out.Trial < 1 || out.SurfaceCalls < 0 || out.TotalTokens < 0 {
		return refuse("invalid scored cell identity/count")
	}
	switch out.State {
	case CellScored, CellVoid, CellUnknown:
		return nil
	default:
		return refuse("unknown cell state %q", out.State)
	}
}

// baseWire freezes the existing contextbench labeled invariants, without
// relying on its untagged Go field names as a public JSON representation.
type baseWire struct {
	CorpusSHA256   string `json:"corpus_sha256"`
	TaskID         string `json:"task_id"`
	Arm            string `json:"arm"`
	ModelID        string `json:"model_id"`
	HarnessVersion string `json:"harness_version"`
	Scoring        string `json:"scoring"`
	PromptSubject  string `json:"prompt_subject"`
	ArmSurface     string `json:"arm_surface"`
}
type inputsWire struct {
	Base             baseWire   `json:"base"`
	SubjectCommit    string     `json:"subject_commit"`
	ToolSchemaSHA256 string     `json:"tool_schema_sha256"`
	ProfileSHA256    string     `json:"profile_sha256"`
	Family           TaskFamily `json:"family"`
	CriterionID      string     `json:"criterion_id"`
	Trial            int        `json:"trial"`
}

func (in CellInputs) MarshalJSON() ([]byte, error) {
	b := in.Base
	return json.Marshal(inputsWire{baseWire{b.CorpusSHA256, b.TaskID, b.Arm, b.ModelID, b.HarnessVersion, b.Scoring, b.PromptSubject, b.ArmSurface}, in.SubjectCommit, in.ToolSchemaSHA256, in.ProfileSHA256, in.Family, in.CriterionID, in.Trial})
}
func (in *CellInputs) UnmarshalJSON(raw []byte) error {
	var w inputsWire
	if err := decodeStrict(raw, &w); err != nil {
		return err
	}
	b := w.Base
	*in = CellInputs{contextbench.CellInputs{CorpusSHA256: b.CorpusSHA256, TaskID: b.TaskID, Arm: b.Arm, ModelID: b.ModelID, HarnessVersion: b.HarnessVersion, Scoring: b.Scoring, PromptSubject: b.PromptSubject, ArmSurface: b.ArmSurface}, w.SubjectCommit, w.ToolSchemaSHA256, w.ProfileSHA256, w.Family, w.CriterionID, w.Trial}
	return nil
}
func decodeStrict(raw []byte, dst any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err := d.Decode(dst); err != nil {
		return err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return refuse("trailing JSON data")
	}
	return nil
}

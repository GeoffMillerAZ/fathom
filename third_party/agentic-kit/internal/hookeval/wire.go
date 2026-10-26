// Package hookeval defines the private, pure spawn evaluator wire.
// Its functions retain no state and are safe for concurrent use.
package hookeval

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"unicode/utf8"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/internal/jsonwire"
)

const RequestVersion = "agent-hooks.spawn-request/1"
const ResultVersion = "agent-hooks.spawn-result/1"
const MaxPolicyBytes = 256 << 10
const MaxRequestBytes = 512 << 10
const MaxResultBytes = 8192

var errWire = errors.New("invalid spawn wire")
var digestRE = regexp.MustCompile(`^[0-9a-f]{64}$`)

type Subject struct {
	Host        string `json:"host"`
	Operation   string `json:"operation"`
	Model       string `json:"model"`
	Effort      string `json:"effort"`
	AgentType   string `json:"agentType"`
	Fork        bool   `json:"fork"`
	ContextFork bool   `json:"contextFork"`
	Brief       string `json:"brief"`
}

type Request struct {
	Version          string  `json:"version"`
	RequestID        string  `json:"requestId"`
	ContextSHA256    string  `json:"contextSHA256"`
	PolicySHA256     string  `json:"policySHA256"`
	PolicyPayload    string  `json:"policyPayload"`
	Subject          Subject `json:"subject"`
	AdvisorySelected bool    `json:"advisorySelected"`
}

type Result struct {
	Version           string   `json:"version"`
	RequestID         string   `json:"requestId"`
	ContextSHA256     string   `json:"contextSHA256"`
	PolicySHA256      string   `json:"policySHA256"`
	NoActionAuthority bool     `json:"noActionAuthority"`
	RequiredDecision  string   `json:"requiredDecision"`
	Codes             []string `json:"codes"`
	Class             string   `json:"class"`
	ClassSource       string   `json:"classSource"`
	AdvisoryEvaluated bool     `json:"advisoryEvaluated"`
	AdviceMatches     int      `json:"adviceMatches"`
}

func hash(b []byte) string { d := sha256.Sum256(b); return hex.EncodeToString(d[:]) }

func NewRequest(contextSHA string, policy []byte, subject Subject, advice bool) (Request, error) {
	r := Request{Version: RequestVersion, ContextSHA256: contextSHA, PolicySHA256: hash(policy), PolicyPayload: base64.StdEncoding.EncodeToString(policy), Subject: subject, AdvisorySelected: advice}
	b, e := json.Marshal(r)
	if e != nil {
		return Request{}, errWire
	}
	r.RequestID = hash(b)
	if ValidateRequest(r) != nil {
		return Request{}, errWire
	}
	return r, nil
}

func DecodeRequest(b []byte) (Request, error) {
	if len(b) > MaxRequestBytes {
		return Request{}, errWire
	}
	var r Request
	if jsonwire.DecodeStrict(b, &r) != nil || ValidateRequest(r) != nil {
		return Request{}, errWire
	}
	return r, nil
}

func ValidateRequest(r Request) error {
	if r.Version != RequestVersion || !digestRE.MatchString(r.RequestID) || !digestRE.MatchString(r.ContextSHA256) || !digestRE.MatchString(r.PolicySHA256) {
		return errWire
	}
	if _, err := PolicyBytes(r); err != nil {
		return errWire
	}
	s := r.Subject
	if (s.Host != "claude" && s.Host != "codex") || (s.Operation != "spawn" && s.Operation != "resume") || len(s.Model) > 256 || len(s.Effort) > 64 || len(s.AgentType) > 128 || len(s.Brief) > 65536 {
		return errWire
	}
	if !utf8.ValidString(s.Model) || !utf8.ValidString(s.Effort) || !utf8.ValidString(s.AgentType) || !utf8.ValidString(s.Brief) {
		return errWire
	}
	if s.Host == "claude" && s.ContextFork || s.Host == "codex" && s.Fork || s.Operation == "resume" && (s.Host != "claude" || s.Fork || s.ContextFork) {
		return errWire
	}
	id := r.RequestID
	r.RequestID = ""
	b, e := json.Marshal(r)
	if e != nil || id != hash(b) {
		return errWire
	}
	return nil
}

func MarshalRequest(r Request) ([]byte, error) {
	if ValidateRequest(r) != nil {
		return nil, errWire
	}
	b, e := json.Marshal(r)
	if e != nil || len(b) > MaxRequestBytes {
		return nil, errWire
	}
	return b, nil
}

func PolicyBytes(r Request) ([]byte, error) {
	if len(r.PolicyPayload) > base64.StdEncoding.EncodedLen(MaxPolicyBytes) {
		return nil, errWire
	}
	b, e := base64.StdEncoding.DecodeString(r.PolicyPayload)
	if e != nil || len(b) == 0 || len(b) > MaxPolicyBytes || base64.StdEncoding.EncodeToString(b) != r.PolicyPayload || hash(b) != r.PolicySHA256 {
		return nil, errWire
	}
	return b, nil
}

func DecodeResult(b []byte) (Result, error) {
	if len(b) > MaxResultBytes {
		return Result{}, errWire
	}
	var r Result
	if jsonwire.DecodeStrict(b, &r) != nil || ValidateResult(r) != nil {
		return Result{}, errWire
	}
	return r, nil
}

func ValidateResult(r Result) error {
	if r.Version != ResultVersion || !digestRE.MatchString(r.RequestID) || !digestRE.MatchString(r.ContextSHA256) || !digestRE.MatchString(r.PolicySHA256) || !r.NoActionAuthority || r.Codes == nil || len(r.Codes) == 0 || len(r.Codes) > 10 || r.AdviceMatches < 0 || r.AdviceMatches > 32 {
		return errWire
	}
	allowedCodes := map[string]bool{"allowed": true, "no_model": true, "unknown_model": true, "tier_ceiling": true, "no_effort": true, "fork_unmarked": true, "fork_denied": true, "policy_invalid": true, "profile_invalid": true, "resume_not_spawn": true}
	seen := map[string]bool{}
	for _, c := range r.Codes {
		if !allowedCodes[c] || seen[c] {
			return errWire
		}
		seen[c] = true
	}
	switch r.RequiredDecision {
	case "pass":
		if len(r.Codes) != 1 || r.Codes[0] != "allowed" || !resolvedClass(r.Class, r.ClassSource) {
			return errWire
		}
	case "deny":
		if !resolvedClass(r.Class, r.ClassSource) {
			return errWire
		}
		for _, c := range r.Codes {
			if c == "allowed" || c == "policy_invalid" || c == "profile_invalid" || c == "resume_not_spawn" {
				return errWire
			}
		}
	case "unknown":
		if r.Class != "unknown" || r.ClassSource != "unknown" {
			return errWire
		}
		for _, c := range r.Codes {
			if c != "policy_invalid" && c != "profile_invalid" {
				return errWire
			}
		}
	case "not-applicable":
		if len(r.Codes) != 1 || r.Codes[0] != "resume_not_spawn" || r.Class != "not-applicable" || r.ClassSource != "not-applicable" {
			return errWire
		}
	default:
		return errWire
	}
	if r.RequiredDecision != "pass" && (r.AdvisoryEvaluated || r.AdviceMatches != 0) || !r.AdvisoryEvaluated && r.AdviceMatches != 0 {
		return errWire
	}
	return nil
}

func resolvedClass(class, source string) bool {
	switch source {
	case "worker_marker", "default":
		return class == "worker"
	case "critic_marker", "critic_pattern":
		return class == "critic"
	}
	return false
}

func MarshalResult(r Result) ([]byte, error) {
	if ValidateResult(r) != nil {
		return nil, errWire
	}
	b, e := json.Marshal(r)
	if e != nil || len(b) > MaxResultBytes {
		return nil, errWire
	}
	return b, nil
}

func BindResult(result Result, request Request) error {
	if ValidateRequest(request) != nil || ValidateResult(result) != nil || result.RequestID != request.RequestID || result.ContextSHA256 != request.ContextSHA256 || result.PolicySHA256 != request.PolicySHA256 {
		return errWire
	}
	if request.Subject.Operation == "resume" && result.RequiredDecision != "not-applicable" || request.Subject.Operation == "spawn" && result.RequiredDecision == "not-applicable" {
		return errWire
	}
	if result.AdvisoryEvaluated != (request.AdvisorySelected && result.RequiredDecision == "pass") {
		return errWire
	}
	return nil
}

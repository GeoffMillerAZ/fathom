package agentharness

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"sync"
	"time"

	toolregistry "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/agent/tool-registry"
	budgetguard "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/governance/budget-guard"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/llmclient"
	injectiondetector "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/security/injection-detector"
)

// EngineeringProfileVersion identifies the governed-engineering composition
// and its coverage receipt semantics.
const EngineeringProfileVersion = "governed-engineering/v1"

// ControlStatus reports whether a required profile control was validated and
// enforced for the reported run.
type ControlStatus string

const (
	// ControlPass means the controlled profile path enforced the control.
	ControlPass ControlStatus = "pass"
	// ControlFail means the profile could not establish the control.
	ControlFail ControlStatus = "fail"
	// ControlUnknown means the profile cannot observe the requested fact.
	ControlUnknown ControlStatus = "unknown"
)

// AttemptUsage is accounting reported by one controlled provider attempt.
// ActualUSD nil means the executor could not determine its actual spend.
type AttemptUsage struct {
	ActualUSD *float64
}

// AttemptExecutor makes exactly one provider attempt for each Attempt call.
// The profile never accepts an arbitrary LLMClient because it cannot inspect
// retries hidden behind Complete. Implementations must not retry internally.
type AttemptExecutor interface {
	Attempt(ctx context.Context, req llmclient.Request) (llmclient.Response, AttemptUsage, error)
}

// AttemptExecutorFunc adapts f to AttemptExecutor.
type AttemptExecutorFunc func(context.Context, llmclient.Request) (llmclient.Response, AttemptUsage, error)

// Attempt calls f.
func (f AttemptExecutorFunc) Attempt(ctx context.Context, req llmclient.Request) (llmclient.Response, AttemptUsage, error) {
	return f(ctx, req)
}

// CompletionEvidence is independently supplied evidence for a completed task.
// Independent must be true; model output or profile self-report is not enough.
type CompletionEvidence struct {
	ID          string
	Independent bool
}

// CompletionEvidenceVerifier validates a completed task against evidence owned
// outside the model/profile execution path.
type CompletionEvidenceVerifier interface {
	VerifyCompletion(ctx context.Context, result RunResult) (CompletionEvidence, error)
}

// CompletionEvidenceVerifierFunc adapts f to CompletionEvidenceVerifier.
type CompletionEvidenceVerifierFunc func(context.Context, RunResult) (CompletionEvidence, error)

// VerifyCompletion calls f.
func (f CompletionEvidenceVerifierFunc) VerifyCompletion(ctx context.Context, result RunResult) (CompletionEvidence, error) {
	return f(ctx, result)
}

// EngineeringProfileConfig is the data-shaped strict profile configuration.
// CUE mirror: #EngineeringProfileConfig in schema.cue.
type EngineeringProfileConfig struct {
	// Harness is the underlying loop configuration. EffectPolicy must be
	// EffectStrict and InjectionAction must be InjectionBlock.
	Harness Config

	// MaxAttempts bounds every logical model call including retries.
	MaxAttempts int

	// ReservedUSDPerAttempt is charged before each controlled provider attempt.
	// It must be positive so the profile has a conservative spend reservation.
	ReservedUSDPerAttempt float64

	// AllowedTools is the exact dispatch scope. Every listed name must be in
	// Deps.Registry; all other calls receive an in-band policy denial.
	AllowedTools []string

	// Retryable decides whether a failed controlled attempt may be retried.
	// Nil means no error is retryable (one attempt).
	Retryable func(error) bool
}

// EngineeringProfileDeps supplies every required strict-profile collaborator.
type EngineeringProfileDeps struct {
	Registry *toolregistry.Registry
	Detector *injectiondetector.Detector
	Budget   *budgetguard.Guard
	Approver Approver
	Attempts AttemptExecutor
	Evidence CompletionEvidenceVerifier

	// Now is forwarded to the underlying Harness for timestamps, durations,
	// and journal entries. Nil uses the Harness's documented time.Now default.
	Now func() time.Time
}

// AttemptAccounting distinguishes conservative reservations, known spend, and
// attempts whose actual spend the controlled executor could not report.
type AttemptAccounting struct {
	Attempts       int
	ReservedUSD    float64
	SpentUSD       float64
	UnknownActuals int
	InvalidActuals int
}

// CoverageReceipt reports what this profile actually enforced for one Run.
// It does not certify provider behavior outside AttemptExecutor.Attempt.
type CoverageReceipt struct {
	Version       string
	Controls      map[string]ControlStatus
	AllowedTools  []string
	Accounting    AttemptAccounting
	NonGuarantees []string
}

// EngineeringResult is a Run result plus completion evidence and coverage.
type EngineeringResult struct {
	Run      RunResult
	Evidence CompletionEvidence
	Coverage CoverageReceipt
}

// EngineeringProfile is a validated, reusable governed-engineering
// composition. It is safe for concurrent Run calls when its injected
// collaborators are safe for concurrent use.
type EngineeringProfile struct {
	cfg  EngineeringProfileConfig
	deps EngineeringProfileDeps
}

// NewEngineeringProfile validates required control wiring before any model or
// tool work can begin. It refuses opaque LLMClient retry stacks: model traffic
// must enter through the explicit single-attempt AttemptExecutor seam.
func NewEngineeringProfile(cfg EngineeringProfileConfig, deps EngineeringProfileDeps) (*EngineeringProfile, error) {
	if cfg.MaxAttempts <= 0 {
		return nil, fmt.Errorf("agentharness: EngineeringProfileConfig.MaxAttempts must be > 0")
	}
	if cfg.ReservedUSDPerAttempt <= 0 || math.IsNaN(cfg.ReservedUSDPerAttempt) || math.IsInf(cfg.ReservedUSDPerAttempt, 0) {
		return nil, fmt.Errorf("agentharness: EngineeringProfileConfig.ReservedUSDPerAttempt must be > 0")
	}
	if cfg.Harness.MaxTokensPerCall <= 0 {
		return nil, errors.New("agentharness: engineering profile requires a positive MaxTokensPerCall")
	}
	if cfg.Harness.EffectPolicy != EffectStrict {
		return nil, errors.New("agentharness: engineering profile requires EffectPolicy strict")
	}
	if cfg.Harness.InjectionAction != InjectionBlock {
		return nil, errors.New("agentharness: engineering profile requires InjectionAction block")
	}
	if cfg.Harness.TreatMaxTokensAsError != nil && !*cfg.Harness.TreatMaxTokensAsError {
		return nil, errors.New("agentharness: engineering profile refuses TreatMaxTokensAsError=false")
	}
	// Pin the required default without retaining a caller-mutable bool pointer.
	cfg.Harness.TreatMaxTokensAsError = nil
	if cfg.Harness.Hooks != nil {
		return nil, errors.New("agentharness: engineering profile owns Hooks; custom hooks could bypass its scope policy")
	}
	if cfg.Harness.EstimatedUSDPerCall != 0 {
		return nil, errors.New("agentharness: engineering profile charges at the attempt boundary; EstimatedUSDPerCall must be 0")
	}
	if engineeringNil(deps.Registry) || engineeringNil(deps.Detector) || engineeringNil(deps.Budget) || engineeringNil(deps.Approver) || engineeringNil(deps.Attempts) || engineeringNil(deps.Evidence) {
		return nil, errors.New("agentharness: engineering profile requires Registry, Detector, Budget, Approver, Attempts, and Evidence")
	}
	if len(cfg.AllowedTools) == 0 {
		return nil, errors.New("agentharness: engineering profile requires a non-empty AllowedTools scope")
	}
	seen := make(map[string]struct{}, len(cfg.AllowedTools))
	for _, name := range cfg.AllowedTools {
		if name == "" {
			return nil, errors.New("agentharness: engineering profile scope contains an empty tool name")
		}
		if _, duplicate := seen[name]; duplicate {
			return nil, fmt.Errorf("agentharness: engineering profile scope repeats tool %q", name)
		}
		if _, ok := deps.Registry.Get(name); !ok {
			return nil, fmt.Errorf("agentharness: engineering profile scope names unregistered tool %q", name)
		}
		seen[name] = struct{}{}
	}
	// Reuse the harness's complete config validation now, before a profile can
	// begin any work. Its client is a local adapter and does not make a model
	// attempt during construction.
	if _, err := New(cfg.Harness, Deps{
		Client: newEngineeringClient(cfg, deps.Attempts, deps.Budget), Registry: deps.Registry,
		Detector: deps.Detector, Approver: deps.Approver,
		Now: deps.Now,
	}); err != nil {
		return nil, fmt.Errorf("agentharness: invalid engineering harness configuration: %w", err)
	}
	cfg.AllowedTools = append([]string(nil), cfg.AllowedTools...)
	return &EngineeringProfile{cfg: cfg, deps: deps}, nil
}

func engineeringNil(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return rv.IsNil()
	default:
		return false
	}
}

// Run executes the strict profile and verifies independent completion evidence
// before returning a passing receipt. Streaming is intentionally unsupported:
// the profile's explicit attempt boundary currently covers Complete only.
func (p *EngineeringProfile) Run(ctx context.Context, userInput string) (EngineeringResult, error) {
	receipt := p.baseReceipt()
	if err := ctx.Err(); err != nil {
		receipt.Controls["context"] = ControlFail
		return EngineeringResult{Run: RunResult{Stop: StopCanceled}, Coverage: receipt}, err
	}
	receipt.Controls["context"] = ControlPass
	client := newEngineeringClient(p.cfg, p.deps.Attempts, p.deps.Budget)
	cfg := p.cfg.Harness
	observed := &engineeringObservations{}
	cfg.Hooks = &HookConfig{
		PreDispatch:    p.scopePolicy(observed),
		PostToolResult: observed.postToolResult,
	}
	h, err := New(cfg, Deps{
		Client: client, Registry: p.deps.Registry, Detector: p.deps.Detector,
		Approver: p.deps.Approver, Now: p.deps.Now,
	})
	if err != nil {
		return EngineeringResult{Coverage: receipt}, err
	}
	run, err := h.Run(ctx, userInput)
	receipt.Accounting = client.accounting()
	p.markObservedControls(&receipt, observed.snapshot())
	if ctxErr := ctx.Err(); ctxErr != nil {
		receipt.Controls["context"] = ControlFail
		receipt.Controls["completion_evidence"] = ControlFail
		return EngineeringResult{Run: run, Coverage: receipt}, ctxErr
	}
	if err != nil {
		receipt.Controls["model_attempts"] = ControlFail
		return EngineeringResult{Run: run, Coverage: receipt}, err
	}
	if run.Stop != StopCompleted {
		receipt.Controls["completion_evidence"] = ControlFail
		return EngineeringResult{Run: run, Coverage: receipt}, nil
	}
	if err := ctx.Err(); err != nil {
		receipt.Controls["context"] = ControlFail
		receipt.Controls["completion_evidence"] = ControlFail
		return EngineeringResult{Run: run, Coverage: receipt}, err
	}
	evidence, verifyErr := p.deps.Evidence.VerifyCompletion(ctx, run)
	if err := ctx.Err(); err != nil {
		receipt.Controls["context"] = ControlFail
		receipt.Controls["completion_evidence"] = ControlFail
		return EngineeringResult{Run: run, Coverage: receipt}, err
	}
	if verifyErr != nil || !evidence.Independent || evidence.ID == "" {
		receipt.Controls["completion_evidence"] = ControlFail
		if verifyErr != nil {
			return EngineeringResult{Run: run, Coverage: receipt}, fmt.Errorf("agentharness: completion evidence: %w", verifyErr)
		}
		return EngineeringResult{Run: run, Coverage: receipt}, errors.New("agentharness: completion evidence must be independent and identified")
	}
	receipt.Controls["completion_evidence"] = ControlPass
	return EngineeringResult{Run: run, Evidence: evidence, Coverage: receipt}, nil
}

func (p *EngineeringProfile) markObservedControls(receipt *CoverageReceipt, observed engineeringObservedControls) {
	if receipt.Accounting.Attempts > 0 {
		receipt.Controls["budget"] = ControlPass
		receipt.Controls["model_attempts"] = ControlPass
	}
	if observed.scope {
		receipt.Controls["scope"] = ControlPass
	}
	if observed.effectPolicy {
		receipt.Controls["effect_policy"] = ControlPass
	}
	if observed.screenedDispatch {
		receipt.Controls["detector"] = ControlPass
		receipt.Controls["registry"] = ControlPass
	}
}

type engineeringObservedControls struct {
	scope            bool
	effectPolicy     bool
	screenedDispatch bool
}

// engineeringObservations records actual harness control boundaries for one
// Run. Hooks may be called from parallel tool execution, so snapshots lock.
type engineeringObservations struct {
	mu sync.Mutex
	engineeringObservedControls
}

func (o *engineeringObservations) preDispatch(allowed bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.scope = true
	// In a strict profile, the Harness calls effectGate immediately after an
	// allowed PreDispatch hook, before any dispatch can begin.
	if allowed {
		o.effectPolicy = true
	}
}

func (o *engineeringObservations) postToolResult(_ context.Context, _ ToolResultView, result llmclient.ContentBlock) (llmclient.ContentBlock, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	// The Harness invokes this only after registry dispatch and screening.
	o.screenedDispatch = true
	return result, nil
}

func (o *engineeringObservations) snapshot() engineeringObservedControls {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.engineeringObservedControls
}

func (p *EngineeringProfile) scopePolicy(observed *engineeringObservations) func(context.Context, ToolCallView) (bool, string, error) {
	allowed := make(map[string]struct{}, len(p.cfg.AllowedTools))
	for _, name := range p.cfg.AllowedTools {
		allowed[name] = struct{}{}
	}
	return func(_ context.Context, view ToolCallView) (bool, string, error) {
		if _, ok := allowed[view.ToolName]; !ok {
			observed.preDispatch(false)
			return false, "tool is outside the governed engineering scope", nil
		}
		observed.preDispatch(true)
		return true, "", nil
	}
}

func (p *EngineeringProfile) baseReceipt() CoverageReceipt {
	controls := map[string]ControlStatus{
		"registry": ControlUnknown, "detector": ControlUnknown, "budget": ControlUnknown,
		"effect_policy": ControlUnknown, "scope": ControlUnknown, "model_attempts": ControlUnknown,
		"completion_evidence": ControlUnknown, "context": ControlUnknown,
	}
	return CoverageReceipt{
		Version: EngineeringProfileVersion, Controls: controls,
		AllowedTools: append([]string(nil), p.cfg.AllowedTools...),
		NonGuarantees: []string{
			"AttemptExecutor is the declared single-attempt boundary; the profile cannot inspect provider behavior behind an executor.",
			"Streaming is unsupported by this profile.",
			"Independent evidence verifies configured completion criteria, not semantic truth or external side effects.",
		},
	}
}

type engineeringClient struct {
	cfg      EngineeringProfileConfig
	attempts AttemptExecutor
	budget   *budgetguard.Guard
	mu       sync.Mutex
	account  AttemptAccounting
}

func newEngineeringClient(cfg EngineeringProfileConfig, attempts AttemptExecutor, budget *budgetguard.Guard) *engineeringClient {
	return &engineeringClient{cfg: cfg, attempts: attempts, budget: budget}
}

func (c *engineeringClient) Complete(ctx context.Context, req llmclient.Request) (llmclient.Response, error) {
	var last llmclient.Response
	for attempt := 1; attempt <= c.cfg.MaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return last, err
		}
		decision, id := c.budget.ChargeTracked(c.cfg.ReservedUSDPerAttempt)
		if !decision.Allowed {
			return last, fmt.Errorf("agentharness: engineering attempt budget denied: %s", decision.Reason)
		}
		resp, usage, err := c.attempts.Attempt(ctx, req)
		if usage.ActualUSD != nil && (*usage.ActualUSD < 0 || math.IsNaN(*usage.ActualUSD) || math.IsInf(*usage.ActualUSD, 0)) {
			c.recordInvalid()
			return resp, errors.New("agentharness: engineering attempt reported invalid actual USD")
		}
		c.record(usage)
		if usage.ActualUSD == nil {
			c.budget.Settle(id, c.cfg.ReservedUSDPerAttempt)
		} else {
			c.budget.Settle(id, *usage.ActualUSD)
		}
		resp.Attempts = attempt
		last = resp
		if ctxErr := ctx.Err(); ctxErr != nil {
			return resp, ctxErr
		}
		if err == nil || attempt == c.cfg.MaxAttempts || c.cfg.Retryable == nil || !c.cfg.Retryable(err) {
			return resp, err
		}
	}
	return last, nil
}

func (c *engineeringClient) recordInvalid() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.account.Attempts++
	c.account.ReservedUSD += c.cfg.ReservedUSDPerAttempt
	c.account.InvalidActuals++
}

func (c *engineeringClient) Stream(context.Context, llmclient.Request) (<-chan llmclient.StreamChunk, error) {
	return nil, errors.New("agentharness: engineering profile does not support streaming; use Run")
}

func (c *engineeringClient) record(usage AttemptUsage) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.account.Attempts++
	c.account.ReservedUSD += c.cfg.ReservedUSDPerAttempt
	if usage.ActualUSD == nil {
		c.account.UnknownActuals++
		return
	}
	c.account.SpentUSD += *usage.ActualUSD
}

func (c *engineeringClient) accounting() AttemptAccounting {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.account
}

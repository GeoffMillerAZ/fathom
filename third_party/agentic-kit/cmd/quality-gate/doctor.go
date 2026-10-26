package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

// doctorReport describes local configuration only. It never examines a candidate
// snapshot, calls a provider, reads a cache, or establishes hook/CI adoption.
type doctorReport struct {
	Version      int           `json:"version"`
	Scope        string        `json:"scope"`
	EvaluatedAt  time.Time     `json:"evaluated_at"`
	Boundary     string        `json:"boundary"`
	State        string        `json:"state"`
	PolicySource string        `json:"policy_source"`
	PolicyID     string        `json:"policy_id,omitempty"`
	PolicyCommit string        `json:"policy_commit,omitempty"`
	PolicySHA256 string        `json:"policy_sha256,omitempty"`
	Caveats      []string      `json:"caveats"`
	Checks       []doctorCheck `json:"checks"`
}

type doctorCheck struct {
	ID     string   `json:"id"`
	State  string   `json:"state"` // pass, unknown, advisory
	Reason string   `json:"reason"`
	Action string   `json:"action"`
	IDs    []string `json:"ids,omitempty"`
}

func runDoctor(out io.Writer, o options, explicitPolicyRef bool) int {
	now := o.now
	if now == nil {
		now = time.Now
	}
	evaluatedAt := now().UTC()
	r := doctorReport{
		Version: 1, Scope: "configuration-only", EvaluatedAt: evaluatedAt, Boundary: o.boundary, State: "pass",
		PolicySource: "committed", Checks: []doctorCheck{},
		Caveats: []string{
			"No candidate snapshot or enforcement verdict was checked.",
			"Provider availability, cache, hook installation and CI invocation were not checked.",
			"A calibration eligible for one clear confidence band does not prove that a future response or provider will pass.",
		},
	}
	add := func(id, state, reason, action string, ids ...string) {
		r.Checks = append(r.Checks, doctorCheck{ID: id, State: state, Reason: reason, Action: action, IDs: ids})
		if state == "unknown" {
			r.State = "unknown"
		}
	}
	if o.policyFile != "" {
		r.PolicySource = "local-untrusted"
		r.Caveats = append(r.Caveats, "Local policy is untrusted bootstrap configuration and cannot authorize enforcement.")
	}
	if (o.boundary == "ci" || o.boundary == "pre-publish") && (!explicitPolicyRef || o.policyFile != "") {
		add("policy-authority", "unknown", "explicit-trusted-policy-ref-required", "Pass --policy-ref from an independently trusted source.")
	} else {
		add("policy-authority", "pass", "policy-source-declared", "No action.")
	}
	gitBin, err := trustedGitPath(o.repo, o.gitBin)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	g := gitSource{Dir: o.repo, GitBin: gitBin}
	gitReady := err == nil
	if gitReady {
		format, probeErr := g.git(ctx, 1024, nil, "rev-parse", "--show-object-format")
		gitReady = probeErr == nil && (string(format) == "sha1\n" || string(format) == "sha256\n")
	}
	if !gitReady {
		add("trusted-git", "unknown", "trusted-git-unavailable", "Initialize the repository and choose a usable external trusted Git executable with --git-bin.")
	} else {
		add("trusted-git", "pass", "external-git-usable", "No action.")
	}
	if r.State == "unknown" {
		return writeDoctor(out, o.format, r)
	}
	data, commit, err := loadPolicy(ctx, g, o)
	if err != nil {
		add("policy", "unknown", "policy-unavailable", "Provide a readable local policy or a committed policy at the trusted ref and path.")
		return writeDoctor(out, o.format, r)
	}
	p, err := decodePolicy(data)
	if err != nil {
		add("policy", "unknown", "policy-invalid", "Correct the policy against the quality-gate schema.")
		return writeDoctor(out, o.format, r)
	}
	hash := sha256.Sum256(data)
	r.PolicyID, r.PolicyCommit, r.PolicySHA256 = p.ID, commit, hex.EncodeToString(hash[:])
	add("policy", "pass", "strict-policy-valid", "No action.")
	if commit != "" {
		if current, e := g.resolveCommit(ctx, o.policyRef); e != nil || current != commit {
			add("policy-stability", "unknown", "trusted-policy-ref-changed", "Retry with a stable trusted policy ref.")
		} else {
			add("policy-stability", "pass", "trusted-policy-ref-stable", "No action.")
		}
	}
	var requiredRules, requiredSites, shadowSites []string
	for _, rule := range p.Rules {
		if includes(rule.Boundaries, o.boundary) {
			requiredRules = append(requiredRules, rule.ID)
		}
	}
	for _, site := range p.Model.Sites {
		if includes(site.Boundaries, o.boundary) {
			if site.Mode == "hold" {
				requiredSites = append(requiredSites, site.ID)
			} else {
				shadowSites = append(shadowSites, site.ID)
			}
		}
	}
	slices.Sort(requiredRules)
	slices.Sort(requiredSites)
	slices.Sort(shadowSites)
	if len(requiredRules)+len(requiredSites) == 0 {
		add("required-coverage", "unknown", "no-required-rules-or-sites", "Configure a required rule or hold site for this boundary.")
	} else {
		add("required-coverage", "pass", "required-configuration-present", "No action.", append(requiredRules, requiredSites...)...)
	}
	if len(shadowSites) > 0 {
		model, expected, key := os.Getenv(p.Model.ModelEnv), os.Getenv(p.Model.ExpectedModelEnv), os.Getenv(p.Model.APIKeyEnv)
		if p.Model.Provider == "none" || model == "" || expected == "" || key == "" {
			add("shadow-sites", "advisory", "optional-shadow-model-unavailable", "Set a requested model alias, expected resolved identity, and credential to run optional shadow sites.", shadowSites...)
		} else {
			add("shadow-sites", "advisory", "optional-shadow-model-configured", "No action required for boundary readiness.", shadowSites...)
		}
	}
	for _, site := range p.Model.Sites {
		if site.Mode != "hold" || !includes(site.Boundaries, o.boundary) {
			continue
		}
		model, expected, key := os.Getenv(p.Model.ModelEnv), os.Getenv(p.Model.ExpectedModelEnv), os.Getenv(p.Model.APIKeyEnv)
		if p.Model.Provider == "none" || model == "" || expected == "" {
			add("model-pin:"+site.ID, "unknown", "required-model-pin-missing", "Set both configured model alias and expected resolved-model environment values.")
		} else {
			add("model-pin:"+site.ID, "pass", "required-model-pin-present", "No action.")
		}
		if key == "" {
			add("credential:"+site.ID, "unknown", "required-credential-missing", "Set the configured provider credential environment variable.")
		} else {
			add("credential:"+site.ID, "pass", "required-credential-present", "No action.")
		}
		if model == "" || expected == "" {
			add("calibration:"+site.ID, "unknown", "model-pin-unavailable", "Set both model pins before checking calibration eligibility.")
		} else {
			status := eligibleClearCalibration(p, o.boundary, site, expected, evaluatedAt)
			if status == "pass" {
				add("calibration:"+site.ID, "pass", "required-clear-calibration-has-eligible-band", "No action.")
			} else {
				add("calibration:"+site.ID, "unknown", "required-clear-calibration-"+status, "Add or refresh a bound calibration that permits a clear result.")
			}
		}
	}
	return writeDoctor(out, o.format, r)
}

// Sample one candidate per occupied confidence band. calibrationStatus checks
// the complete bound reading for each, so no band is inferred from another.
func eligibleClearCalibration(p policy, boundary string, site modelSite, expected string, at time.Time) string {
	candidates := []float64{site.MinConfidence, 1}
	if site.Calibration != nil {
		for _, c := range site.Calibration.Cases {
			if c.Actual == "clear" && c.Confidence >= site.MinConfidence {
				candidates = append(candidates, c.Confidence)
			}
		}
	}
	seen := map[decision.Band]bool{}
	status := "missing"
	for _, confidence := range candidates {
		band := decision.BandOf(confidence)
		if seen[band] {
			continue
		}
		seen[band] = true
		current := calibrationStatus(p.ID, boundary, site, expectedProvider(p.Model.Provider), expected, "clear", confidence, at, p.Model.Sites...)
		if current == "pass" {
			return "pass"
		}
		if status == "missing" || current == "binding-mismatch" || current == "expired" || current == "insufficient-labels" {
			status = current
		}
	}
	return status
}

func writeDoctor(w io.Writer, format string, r doctorReport) int {
	code := 0
	if r.State == "unknown" {
		code = 2
	}
	if format == "json" {
		if emitJSON(w, r) != 0 {
			return 2
		}
		return code
	}
	var b strings.Builder
	b.WriteString("quality-gate doctor " + r.State + " scope=" + r.Scope + " boundary=" + r.Boundary + " policy-source=" + r.PolicySource + " evaluated-at=" + r.EvaluatedAt.Format(time.RFC3339Nano) + "\n")
	if r.PolicyID != "" {
		b.WriteString("  policy: id=" + r.PolicyID + " commit=" + r.PolicyCommit + " sha256=" + r.PolicySHA256 + "\n")
	}
	for _, caveat := range r.Caveats {
		b.WriteString("  caveat: " + caveat + "\n")
	}
	for _, c := range r.Checks {
		b.WriteString("  " + c.ID + " " + c.State + " reason=" + c.Reason + " action=" + c.Action)
		if len(c.IDs) > 0 {
			b.WriteString(" ids=" + strings.Join(c.IDs, ","))
		}
		b.WriteByte('\n')
	}
	if _, err := io.WriteString(w, b.String()); err != nil {
		return 2
	}
	return code
}

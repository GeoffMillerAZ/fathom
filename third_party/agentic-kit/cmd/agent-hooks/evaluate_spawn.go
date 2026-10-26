package main

import (
	"fmt"
	"io"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/internal/hookeval"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/internal/jsonwire"
)

// evaluateSpawn is the strict private peer. It does not read the environment,
// home directory, definitions, ledger, or any policy path.
func (a *app) evaluateSpawn(args []string) (status int) {
	defer func() {
		if recover() != nil {
			status = 2
		}
	}()
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
		const help = "Usage: agent-hooks evaluate-spawn < request.json\n"
		n, err := io.WriteString(a.stdout, help)
		if err != nil || n != len(help) {
			fmt.Fprintln(a.stderr, "agent-hooks evaluate-spawn: output failure")
			return 2
		}
		return 0
	}
	if len(args) != 0 {
		fmt.Fprintln(a.stderr, "agent-hooks evaluate-spawn: invalid request")
		return 2
	}
	b, err := io.ReadAll(io.LimitReader(a.stdin, hookeval.MaxRequestBytes+1))
	if err != nil || len(b) > hookeval.MaxRequestBytes {
		fmt.Fprintln(a.stderr, "agent-hooks evaluate-spawn: invalid request")
		return 2
	}
	r, err := hookeval.DecodeRequest(b)
	if err != nil {
		fmt.Fprintln(a.stderr, "agent-hooks evaluate-spawn: invalid request")
		return 2
	}
	result := evaluateRequest(r)
	out, err := hookeval.MarshalResult(result)
	if err != nil || len(out)+1 > hookeval.MaxResultBytes {
		fmt.Fprintln(a.stderr, "agent-hooks evaluate-spawn: invalid result")
		return 2
	}
	out = append(out, '\n')
	n, err := a.stdout.Write(out)
	if err != nil || n != len(out) {
		fmt.Fprintln(a.stderr, "agent-hooks evaluate-spawn: output failure")
		return 2
	}
	switch result.RequiredDecision {
	case "pass", "not-applicable":
		return 0
	case "deny":
		return 1
	default:
		return 2
	}
}

func evaluateRequest(r hookeval.Request) hookeval.Result {
	result := hookeval.Result{Version: hookeval.ResultVersion, RequestID: r.RequestID, ContextSHA256: r.ContextSHA256, PolicySHA256: r.PolicySHA256, NoActionAuthority: true, Codes: []string{"policy_invalid"}, RequiredDecision: "unknown", Class: "unknown", ClassSource: "unknown"}
	b, err := hookeval.PolicyBytes(r)
	if err != nil {
		return result
	}
	if jsonwire.ValidateStrictJSON(b) != nil {
		return result
	}
	p, err := parsePolicy(b)
	if err != nil {
		return result
	}
	if !p.Spawn.RequireModel || p.Spawn.Fork != forkMarker && p.Spawn.Fork != forkDeny || !p.hostMapped(r.Subject.Host) || r.Subject.Host == hostCodex && !p.requiresEffort(hostCodex) {
		result.Codes = []string{"profile_invalid"}
		return result
	}
	if r.Subject.Operation == "resume" {
		result.RequiredDecision = "not-applicable"
		result.Codes = []string{"resume_not_spawn"}
		result.Class = "not-applicable"
		result.ClassSource = "not-applicable"
		return result
	}
	class, source := classifySource(p, r.Subject.Brief)
	required := *p
	required.adviceRE = nil
	v := decideSpawn(&required, spawnRequest{Host: r.Subject.Host, Model: r.Subject.Model, Effort: r.Subject.Effort, AgentType: r.Subject.AgentType, Fork: r.Subject.Fork, ContextFork: r.Subject.ContextFork, Brief: r.Subject.Brief})
	result.Class, result.ClassSource = class, source
	result.Codes = append([]string(nil), v.Codes...)
	if r.Subject.ContextFork {
		switch p.Spawn.Fork {
		case forkMarker:
			if !hasMarker(r.Subject.Brief, p.Spawn.ForkMarker) {
				result.Codes = appendDeny(result.Codes, codeForkMarker)
			}
		case forkDeny:
			result.Codes = appendDeny(result.Codes, codeForkDenied)
		}
	}
	if len(result.Codes) == 1 && result.Codes[0] == codeAllowed {
		result.RequiredDecision = "pass"
		if r.AdvisorySelected {
			result.AdvisoryEvaluated = true
			result.AdviceMatches = len(matchAdvice(p, class, r.Subject.Brief))
		}
	} else {
		result.RequiredDecision = "deny"
	}
	return result
}

func appendDeny(codes []string, code string) []string {
	if len(codes) == 1 && codes[0] == codeAllowed {
		return []string{code}
	}
	return append(codes, code)
}

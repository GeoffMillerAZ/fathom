package main

import (
	"fmt"
	"strings"
)

// Brief classes.
const (
	classWorker = "worker"
	classCritic = "critic"
)

// Decision codes recorded in the ledger. A deny can carry several.
const (
	codeAllowed      = "allowed"
	codeNoModel      = "no_model"
	codeUnknownModel = "unknown_model"
	codeTierCeiling  = "tier_ceiling"
	codeNoEffort     = "no_effort"
	codeForkMarker   = "fork_unmarked"
	codeForkDenied   = "fork_denied"
)

// verdict is the outcome of one spawn decision.
type verdict struct {
	Deny   bool
	Codes  []string
	Reason string // shown to the agent on deny
	Class  string
	Tier   string // resolved tier name, "" when unresolved
	Notes  []note // once-per-session advisories for allowed spawns
}

// note is advice keyed for once-per-session delivery.
type note struct {
	Key  string
	Text string
}

// decideSpawn applies the policy to one spawn. It is pure: no I/O, no clock,
// so every rule is table-testable for both hosts. All failed requirements are
// reported together, so one retry can fix them all.
func decideSpawn(p *Policy, r spawnRequest) verdict {
	v := verdict{Class: classify(p, r.Brief)}

	if r.Fork {
		return decideFork(p, r, v)
	}

	var problems []string
	model := strings.TrimSpace(r.Model)
	tier := -1
	switch {
	case model == "" || strings.EqualFold(model, "inherit"):
		if p.Spawn.RequireModel {
			v.Codes = append(v.Codes, codeNoModel)
			problems = append(problems, noModelProblem(r))
		}
	case !p.hostMapped(r.Host):
		v.Notes = append(v.Notes, note{
			Key:  "unmapped-host-" + r.Host,
			Text: fmt.Sprintf("agent-hooks: the policy lists no %s models, so tier rules are not enforced for %s spawns. Add them to the policy's tiers.", r.Host, r.Host),
		})
	default:
		tier = p.tierFor(r.Host, model)
		if tier < 0 {
			v.Codes = append(v.Codes, codeUnknownModel)
			problems = append(problems, fmt.Sprintf("model %q is not in any tier of the policy", model))
		}
	}

	if tier >= 0 {
		v.Tier = p.Tiers[tier].Name
		if ceiling := p.ceiling(v.Class); tier > ceiling {
			if marker := p.Tiers[tier].Marker; marker == "" || !hasMarker(r.Brief, marker) {
				v.Codes = append(v.Codes, codeTierCeiling)
				problems = append(problems, ceilingProblem(p, v.Class, tier, ceiling))
			}
		}
	}

	if p.requiresEffort(r.Host) && r.Effort == "" {
		v.Codes = append(v.Codes, codeNoEffort)
		problem := "no reasoning_effort set: an omitted effort inherits the parent's"
		if tier >= 0 && p.Tiers[tier].Effort != "" {
			problem += fmt.Sprintf(" (recommended for %s: %s)", p.Tiers[tier].Name, p.Tiers[tier].Effort)
		}
		problems = append(problems, problem)
	}

	if len(problems) > 0 {
		v.Deny = true
		v.Reason = denyReason(p, r.Host, problems)
		return v
	}
	v.Codes = []string{codeAllowed}
	v.Notes = append(v.Notes, matchAdvice(p, v.Class, r.Brief)...)
	return v
}

// noModelProblem explains a missing model accurately: an agent whose own
// definition pins a model runs on that model, not the session's, so the
// retry should name the pinned model rather than pick a new one.
func noModelProblem(r spawnRequest) string {
	if r.DefinedModel != "" {
		return fmt.Sprintf("no model set: agent %q defines model %q; name it on the spawn (model: %q) so every spawn's cost is explicit",
			r.AgentType, r.DefinedModel, r.DefinedModel)
	}
	return "no model set: an unset model inherits the session model, usually the most expensive tier"
}

// decideFork applies the fork mode. Forks inherit the session model and
// context, so their cost is the session's; the marker makes that a choice.
func decideFork(p *Policy, r spawnRequest, v verdict) verdict {
	v.Tier = "inherit"
	switch {
	case p.Spawn.Fork == forkAllow,
		p.Spawn.Fork == forkMarker && hasMarker(r.Brief, p.Spawn.ForkMarker):
		v.Codes = []string{codeAllowed}
		v.Notes = matchAdvice(p, v.Class, r.Brief)
		return v
	case p.Spawn.Fork == forkMarker:
		v.Codes = []string{codeForkMarker}
		v.Reason = fmt.Sprintf("agent-hooks: fork blocked. A fork inherits the session model and cannot take one, so it runs at the session's cost. "+
			"Prefer a fresh subagent with an explicit model and a self-contained brief. "+
			"If the inherited context is essential, put %s in the prompt.\n\n%s", p.Spawn.ForkMarker, routingTable(p, r.Host))
	default:
		v.Codes = []string{codeForkDenied}
		v.Reason = "agent-hooks: forks are disabled by policy. Spawn a fresh subagent with an explicit model.\n\n" + routingTable(p, r.Host)
	}
	v.Deny = true
	return v
}

// classify returns the brief's class. Explicit markers win over the pattern;
// the worker marker wins over the critic marker.
func classify(p *Policy, brief string) string {
	class, _ := classifySource(p, brief)
	return class
}

func classifySource(p *Policy, brief string) (string, string) {
	switch {
	case p.Classes.WorkerMarker != "" && hasMarker(brief, p.Classes.WorkerMarker):
		return classWorker, "worker_marker"
	case p.Classes.CriticMarker != "" && hasMarker(brief, p.Classes.CriticMarker):
		return classCritic, "critic_marker"
	case p.criticRE != nil && p.criticRE.MatchString(brief):
		return classCritic, "critic_pattern"
	}
	return classWorker, "default"
}

// ceiling returns the tier index a class may use without a marker.
func (p *Policy) ceiling(class string) int {
	if class == classCritic {
		return p.tierIndex(p.Classes.Critic.MaxTier)
	}
	return p.tierIndex(p.Classes.Worker.MaxTier)
}

func ceilingProblem(p *Policy, class string, tier, ceiling int) string {
	t := p.Tiers[tier]
	msg := fmt.Sprintf("%s briefs cap at the %s tier and this model is the %s tier: retry on the %s tier or below",
		class, p.Tiers[ceiling].Name, t.Name, p.Tiers[ceiling].Name)
	escape := []string{}
	if t.Marker != "" {
		escape = append(escape, t.Marker)
	}
	if class == classWorker && p.Classes.CriticMarker != "" && tier <= p.ceiling(classCritic) {
		escape = append(escape, p.Classes.CriticMarker+" for review, verification or synthesis")
	}
	if len(escape) > 0 {
		msg += ", or justify the stronger tier by putting " + strings.Join(escape, ", or ") + " in the prompt"
	}
	return msg
}

// matchAdvice returns the advice rules that apply to an allowed brief.
func matchAdvice(p *Policy, class, brief string) []note {
	var notes []note
	for _, ad := range p.adviceRE {
		if len(ad.Classes) > 0 && !contains(ad.Classes, class) {
			continue
		}
		if !ad.when.MatchString(brief) || (ad.unless != nil && ad.unless.MatchString(brief)) {
			continue
		}
		notes = append(notes, note{Key: "advice-" + ad.ID, Text: ad.Message})
	}
	return notes
}

// denyReason renders the problems and the routing table: the guidance an
// agent needs to retry correctly, at the moment it needs it.
func denyReason(p *Policy, host string, problems []string) string {
	var b strings.Builder
	b.WriteString("agent-hooks: subagent spawn blocked.\n")
	for _, pr := range problems {
		b.WriteString("- ")
		b.WriteString(pr)
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(routingTable(p, host))
	return b.String()
}

// routingTable lists the tiers cheapest first with their models and use.
func routingTable(p *Policy, host string) string {
	var b strings.Builder
	field := "model"
	if host == hostCodex {
		field = "model and reasoning_effort"
	}
	fmt.Fprintf(&b, "Set %s on every spawn. Pick the cheapest tier that fits:\n", field)
	for _, t := range p.Tiers {
		models := p.hostModels(t, host)
		if len(models) == 0 && host == hostCodex && p.hostMapped(host) {
			continue
		}
		name := t.Name
		if len(models) > 0 {
			name += " = " + strings.Join(models, " | ")
		}
		if host == hostCodex && t.Effort != "" {
			name += " (effort " + t.Effort + ")"
		}
		if t.Marker != "" {
			name += " " + t.Marker
		}
		fmt.Fprintf(&b, "- %s: %s\n", name, t.UseFor)
	}
	fmt.Fprintf(&b, "Workers cap at %s, critics at %s.", p.Classes.Worker.MaxTier, p.Classes.Critic.MaxTier)
	if p.Classes.CriticMarker != "" {
		fmt.Fprintf(&b, " Mark judgment work %s.", p.Classes.CriticMarker)
	}
	return b.String()
}

// hasMarker reports whether the brief carries the marker, case-insensitively.
func hasMarker(brief, marker string) bool {
	return marker != "" && strings.Contains(strings.ToLower(brief), strings.ToLower(marker))
}

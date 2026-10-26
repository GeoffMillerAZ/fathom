package main

import (
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

// recoveryWindow is how soon after a deny an allowed spawn in the same
// session counts as the agent recovering from it rather than giving up.
const recoveryWindow = 15 * time.Minute

// report summarizes the ledger over the last N days: the evidence that keeps
// or retires each rule.
func (a *app) report(args []string) int {
	flags := flag.NewFlagSet("report", flag.ContinueOnError)
	flags.SetOutput(a.stderr)
	days := flags.Int("days", 7, "how many days of ledger to summarize")
	if flags.Parse(args) != nil || *days < 1 {
		return 2
	}
	since := a.now().Add(-time.Duration(*days) * 24 * time.Hour)
	s := newSummary()
	for _, f := range a.ledgerFiles() {
		scanLedger(f, func(r ledgerRecord, ts time.Time) {
			if !ts.Before(since) {
				s.add(r, ts)
			}
		})
	}
	s.write(a.stdout, *days)
	return 0
}

type summary struct {
	spawns, denied, forks      int
	denyCodes                  map[string]int
	allowedByTier              map[string]int
	tokensByModel              map[string]int64
	reportChars                map[string][]int
	mainChars, subChars        int
	contained, containedChars  int
	outputByCommand            map[string]int
	faults, unknownSpend       int
	lastFault                  string
	pendingDeny                map[string][]time.Time // session → unrecovered deny times
	recovered, recoveredAsFork int
	// agents holds one spend entry per spawned agent, so a sync Claude Code
	// spawn's spawn_result and its own subagent_stop are counted once.
	agents map[string]*agentSpend
	// anonymous holds spend records that carry no agent id.
	anonymous []*agentSpend
}

// agentSpend is what one subagent cost and returned.
type agentSpend struct {
	host, agentType, model, transcript string
	tokens                             int64
	reportChars                        int
	// final is set by a completed spawn_result: its token total is
	// authoritative and a later or earlier subagent_stop adds nothing.
	final bool
}

func newSummary() *summary {
	return &summary{
		denyCodes: map[string]int{}, allowedByTier: map[string]int{}, tokensByModel: map[string]int64{},
		reportChars: map[string][]int{}, outputByCommand: map[string]int{}, pendingDeny: map[string][]time.Time{},
		agents: map[string]*agentSpend{},
	}
}

// add folds one record into the summary. Records arrive in file order,
// which is time order per file, so deny recovery can be tracked in one pass.
func (s *summary) add(r ledgerRecord, ts time.Time) {
	switch r.Kind {
	case kindSpawn:
		s.spawns++
		if r.Decision == "deny" {
			s.denied++
			for _, c := range r.Codes {
				s.denyCodes[c]++
			}
			s.pendingDeny[r.Session] = append(s.pendingDeny[r.Session], ts)
			return
		}
		if r.Fork {
			s.forks++
		}
		tier := r.Tier
		if tier == "" {
			tier = "(unresolved)"
		}
		s.allowedByTier[r.Host+" "+tier]++
		if pending := s.pendingDeny[r.Session]; len(pending) > 0 && ts.Sub(pending[0]) <= recoveryWindow {
			s.recovered++
			if r.Fork {
				s.recoveredAsFork++
			}
			s.pendingDeny[r.Session] = pending[1:]
		}
	case kindSpawnResult:
		if r.Status != "completed" {
			return // async launch or Codex handle: the stop record carries the spend
		}
		spend := s.agent(r.SpawnedAgent)
		*spend = agentSpend{host: r.Host, agentType: r.AgentType, model: r.Resolved, tokens: r.TotalTokens, reportChars: r.ReportChars, final: true}
	case kindSubagentStop:
		spend := s.agent(r.AgentID)
		if spend.final {
			return
		}
		*spend = agentSpend{host: r.Host, agentType: r.AgentType, model: r.Resolved, tokens: r.TotalTokens, reportChars: r.ReportChars, transcript: r.Transcript}
	case kindToolOutput:
		if r.Host == hostClaude {
			if r.AgentID == "" {
				s.mainChars += r.OutputChars
			} else {
				s.subChars += r.OutputChars
			}
		}
		if r.Contained {
			s.contained++
			s.containedChars += r.OutputChars
		}
		if r.Command != "" {
			s.outputByCommand[r.Command] += r.OutputChars
		}
	case kindFault:
		s.faults++
		s.lastFault = r.Fault
	}
}

// agent returns the spend entry for an agent id, creating it; records with
// no id get an entry of their own.
func (s *summary) agent(id string) *agentSpend {
	if id == "" {
		a := &agentSpend{}
		s.anonymous = append(s.anonymous, a)
		return a
	}
	if s.agents[id] == nil {
		s.agents[id] = &agentSpend{}
	}
	return s.agents[id]
}

// settleSpend folds the per-agent entries into the totals, reading a
// transcript only for an agent no completed spawn_result already covered.
func (s *summary) settleSpend() {
	all := s.anonymous
	for _, id := range sortedKeys(s.agents) {
		all = append(all, s.agents[id])
	}
	for _, a := range all {
		if a.model == "" && a.transcript != "" {
			if u, ok := readTranscriptUsage(a.host, a.transcript); ok {
				a.model, a.tokens = u.Model, u.Tokens
			}
		}
		if a.model != "" {
			s.tokensByModel[a.model] += a.tokens
		} else {
			s.unknownSpend++ // transcript gone or unreadable: unknown, never zero
		}
		if a.reportChars > 0 {
			key := a.host + " " + orUnset(a.agentType)
			s.reportChars[key] = append(s.reportChars[key], a.reportChars)
		}
	}
}

func (s *summary) write(w io.Writer, days int) {
	s.settleSpend()
	fmt.Fprintf(w, "agent-hooks report — last %d days\n\n", days)
	fmt.Fprintf(w, "Spawns: %d decided, %d denied, %d allowed forks\n", s.spawns, s.denied, s.forks)
	if s.denied > 0 {
		fmt.Fprintf(w, "  denies by reason: %s\n", formatCounts(s.denyCodes))
		fmt.Fprintf(w, "  recovered within %s: %d of %d (%d as forks)\n", recoveryWindow, s.recovered, s.denied, s.recoveredAsFork)
	}
	if len(s.allowedByTier) > 0 {
		fmt.Fprintf(w, "  allowed by tier: %s\n", formatCounts(s.allowedByTier))
	}
	if len(s.tokensByModel) > 0 {
		fmt.Fprintln(w, "\nSubagent tokens by resolved model:")
		for _, k := range sortedKeys(s.tokensByModel) {
			fmt.Fprintf(w, "  %-28s %d\n", k, s.tokensByModel[k])
		}
	}
	if len(s.reportChars) > 0 {
		fmt.Fprintln(w, "\nSubagent report size, chars (median / p90 / n):")
		for _, k := range sortedKeys(s.reportChars) {
			v := s.reportChars[k]
			fmt.Fprintf(w, "  %-28s %d / %d / %d\n", k, percentile(v, 50), percentile(v, 90), len(v))
		}
	}
	if s.mainChars+s.subChars > 0 {
		fmt.Fprintf(w, "\nTool output into context (Claude Code): main thread %d chars, subagents %d chars\n", s.mainChars, s.subChars)
	}
	if s.contained > 0 {
		fmt.Fprintf(w, "  contained in the main thread: %d outputs, %d chars originally\n", s.contained, s.containedChars)
	}
	if len(s.outputByCommand) > 0 {
		fmt.Fprintln(w, "\nLargest shell output sinks (total chars):")
		keys := sortedKeys(s.outputByCommand)
		sort.SliceStable(keys, func(i, j int) bool { return s.outputByCommand[keys[i]] > s.outputByCommand[keys[j]] })
		for i, k := range keys {
			if i == 10 {
				break
			}
			fmt.Fprintf(w, "  %-20s %d\n", k, s.outputByCommand[k])
		}
	}
	if s.unknownSpend > 0 {
		fmt.Fprintf(w, "  subagents with unknown spend (transcript gone or unreadable): %d\n", s.unknownSpend)
	}
	if s.faults > 0 {
		fmt.Fprintf(w, "\nFaults (hook failed open): %d, latest: %s\n", s.faults, s.lastFault)
	}
}

func formatCounts[V int | int64](m map[string]V) string {
	keys := sortedKeys(m)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = fmt.Sprintf("%s=%d", k, m[k])
	}
	return strings.Join(parts, ", ")
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// percentile returns the nearest-rank percentile of v (unsorted input).
func percentile(v []int, p int) int {
	if len(v) == 0 {
		return 0
	}
	s := append([]int(nil), v...)
	sort.Ints(s)
	idx := (p*len(s)+99)/100 - 1
	return s[max(0, min(idx, len(s)-1))]
}

func orUnset(s string) string {
	if s == "" {
		return "(unset)"
	}
	return s
}

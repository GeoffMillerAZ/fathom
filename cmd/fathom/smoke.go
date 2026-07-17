package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/fathom/internal/config"
)

// smoke.go — CF-6 (docs/plans/config-experience.md): `fathom smoke` is
// SPEND, not health — one tiny real Complete call per selected profile,
// reporting model/latency/tokens/cost. Contrast with `fathom providers`
// (CF-5), which never spends a token. Because this command spends real
// money against real accounts, --profile or --all is REQUIRED — there is no
// implicit "smoke everything" default the way providers has one.
//
// Cost is estimated using the same flat per-KTok rates cmd/fathom already
// uses for every compile run (compileViaStore/runServeHTTP's
// c.WithLLM(..., 0.001, 0.005) — see smokeInRatePerKTok/smokeOutRatePerKTok
// below), so a smoke run's reported cost is directly comparable to a
// compile run's cost_usd.
const (
	smokeInRatePerKTok  = 0.001
	smokeOutRatePerKTok = 0.005
	smokePrompt         = "Reply with exactly: pong"
	smokeMaxTokens      = 8
)

// cmdSmoke implements "smoke": run smokeProfiles against the selected
// profile(s) using time.Now as the wall-clock source, and render the
// result. Exit 0 only when every selected profile completed successfully.
func cmdSmoke(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("smoke", stderr)
	profileFlag := fs.String("profile", "", "smoke-test this named profile")
	all := fs.Bool("all", false, "smoke-test every configured profile")
	jsonOut := fs.Bool("json", false, "emit JSON instead of a human table")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *profileFlag == "" && !*all {
		fmt.Fprintln(stderr, "error: smoke spends tokens on a real completion — pass --profile <name> or --all explicitly")
		fs.Usage()
		return 1
	}

	loaded, err := config.Load(config.DefaultUserPath(), config.DefaultProjectPath())
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	selector := *profileFlag
	if *all {
		selector = ""
	}
	names, err := selectProfileNames(loaded.Config, selector)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	rows := smokeProfiles(loaded, names, time.Now)

	failed := false
	for _, r := range rows {
		if !r.OK {
			failed = true
		}
	}

	if *jsonOut {
		printSmokeJSON(stdout, rows)
	} else {
		printSmokeTable(stdout, rows)
	}
	if failed {
		return 1
	}
	return 0
}

// smokeRow is one profile's smoke-test outcome.
type smokeRow struct {
	Name      string
	Provider  string
	Model     string
	OK        bool
	LatencyMS int64
	TokensIn  int
	TokensOut int
	CostUSD   float64
	Detail    string
}

// smokeProfiles resolves, constructs, and runs one tiny Complete call per
// name, in order. now is the injected wall-clock source (repo convention:
// no bare time.Now() in logic) — production passes time.Now, tests pass a
// fixed/stepped fake so latency assertions stay deterministic.
func smokeProfiles(loaded config.Loaded, names []string, now func() time.Time) []smokeRow {
	rows := make([]smokeRow, 0, len(names))
	for _, name := range names {
		row := smokeRow{Name: displayProfileName(name)}

		p, _, err := config.ResolveProfile(loaded.Config, loaded.Provenance, name)
		if err != nil {
			row.Detail = err.Error()
			rows = append(rows, row)
			continue
		}
		row.Provider = p.Provider

		cl, err := buildLLMClientForProfile(p)
		if err != nil {
			row.Detail = err.Error()
			rows = append(rows, row)
			continue
		}

		model := compileModelForProfile(p)
		start := now()
		resp, err := cl.Complete(context.Background(), llmclient.Request{
			Model:     model,
			Prompt:    smokePrompt,
			MaxTokens: smokeMaxTokens,
		})
		row.LatencyMS = now().Sub(start).Milliseconds()
		if err != nil {
			row.Detail = err.Error()
			rows = append(rows, row)
			continue
		}

		row.Model = model
		row.TokensIn = resp.InputTokens
		row.TokensOut = resp.OutputTokens
		row.CostUSD = float64(resp.InputTokens)/1000*smokeInRatePerKTok + float64(resp.OutputTokens)/1000*smokeOutRatePerKTok
		row.OK = true
		row.Detail = resp.Text
		rows = append(rows, row)
	}
	return rows
}

func printSmokeTable(w io.Writer, rows []smokeRow) {
	fmt.Fprintf(w, "%-24s %-14s %-28s %-6s %-9s %-6s %-6s %-10s %s\n",
		"PROFILE", "PROVIDER", "MODEL", "OK", "LATENCY", "IN", "OUT", "COST_USD", "DETAIL")
	for _, r := range rows {
		fmt.Fprintf(w, "%-24s %-14s %-28s %-6v %-9s %-6d %-6d %-10.6f %s\n",
			r.Name, r.Provider, r.Model, r.OK,
			fmt.Sprintf("%dms", r.LatencyMS), r.TokensIn, r.TokensOut, r.CostUSD, r.Detail)
	}
}

func printSmokeJSON(w io.Writer, rows []smokeRow) {
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]any{
			"profile":    r.Name,
			"provider":   r.Provider,
			"model":      r.Model,
			"ok":         r.OK,
			"latency_ms": r.LatencyMS,
			"tokens_in":  r.TokensIn,
			"tokens_out": r.TokensOut,
			"cost_usd":   r.CostUSD,
			"detail":     r.Detail,
		})
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}

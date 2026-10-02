package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/geoffmilleraz/agentic-kit/foundations/llmclient"
	"github.com/geoffmilleraz/fathom/internal/config"
)

// providers.go — CF-5 (docs/plans/config-experience.md): `fathom providers`
// is AUTH HEALTH, not spend — construct each selected profile's client and
// run one llmclient.ProbeAll pass (zero/near-zero tokens; ProbeAll's own doc
// says every Pinger issues at most a minimal signed request). A profile
// whose adapter does not implement llmclient.Pinger (a type-assertion, not a
// provider-name allowlist — an adapter can gain Ping without this command
// needing an update) is reported "not probeable" and STILL LISTED, never
// silently dropped from the output.
//
// Contrast with `fathom smoke` (CF-6): that command spends a token or two on
// a real completion per profile; this one never does.

// cmdProviders implements "providers": probe every configured profile's
// auth health (or just --profile's, if given). Exit 0 when every PROBEABLE
// lane is healthy; exit 1 if any probed lane failed or any profile could
// not even be constructed. A "not probeable" lane is informational, not a
// failure — an adapter lacking Ping is a capability gap, not a broken
// credential.
func cmdProviders(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("providers", stderr)
	profileFlag := fs.String("profile", "", "probe only this named profile (default: every configured profile, or the bare env-overlay when none are configured)")
	jsonOut := fs.Bool("json", false, "emit JSON instead of a human table")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	loaded, err := config.Load(config.DefaultUserPath(), config.DefaultProjectPath())
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	names, err := selectProfileNames(loaded.Config, *profileFlag)
	if err != nil {
		fmt.Fprintf(stderr, "error: %v\n", err)
		return 1
	}

	rows := probeProfiles(loaded, names, providerClientBuilder)

	failed := false
	for _, r := range rows {
		if r.Status == providerStatusError {
			failed = true
		}
	}

	if *jsonOut {
		printProvidersJSON(stdout, rows)
	} else {
		printProvidersTable(stdout, rows)
	}
	if failed {
		return 1
	}
	return 0
}

// selectProfileNames resolves which named profiles "providers"/"smoke"
// operate over: profileFlag alone when given (validated to exist), else
// every configured profile name sorted, else a single "" entry meaning
// "the bare env-overlay, no named profile at all" (pre-config-file fathom).
func selectProfileNames(cfg config.Config, profileFlag string) ([]string, error) {
	if profileFlag != "" {
		if _, ok := cfg.Profiles[profileFlag]; !ok {
			return nil, fmt.Errorf("config: profile %q not found (known profiles: %s)", profileFlag, joinProfileNames(cfg.Profiles))
		}
		return []string{profileFlag}, nil
	}
	if len(cfg.Profiles) == 0 {
		return []string{""}, nil
	}
	names := make([]string, 0, len(cfg.Profiles))
	for n := range cfg.Profiles {
		names = append(names, n)
	}
	sort.Strings(names)
	return names, nil
}

func joinProfileNames(profiles map[string]config.Profile) string {
	names := make([]string, 0, len(profiles))
	for n := range profiles {
		names = append(names, n)
	}
	sort.Strings(names)
	out := ""
	for i, n := range names {
		if i > 0 {
			out += ", "
		}
		out += n
	}
	return out
}

type providerStatus string

const (
	providerStatusOK           providerStatus = "ok"
	providerStatusError        providerStatus = "error"
	providerStatusNotProbeable providerStatus = "not probeable"
)

// providerRow is one profile's probe/construction outcome.
type providerRow struct {
	Name     string
	Provider string
	Status   providerStatus
	Detail   string
}

// profileClientBuilder is probeProfiles's construction seam: production
// passes buildLLMClientForProfile; tests can pass a stub that returns a
// client deliberately lacking Ping, because every real adapter currently
// implements llmclient.Pinger and the "not probeable" reporting branch
// would otherwise be unreachable in tests (N6).
type profileClientBuilder func(config.Profile) (llmclient.LLMClient, error)

// providerClientBuilder is what cmdProviders actually uses — a var so the
// N6 test can drive the full command (including its exit-code contract)
// through a Ping-less stub client, then restore it.
var providerClientBuilder profileClientBuilder = buildLLMClientForProfile

// probeProfiles resolves and constructs a client for each name, partitions
// them into probeable (Pinger) vs not, runs ONE llmclient.ProbeAll call
// across every probeable lane (per ProbeAll's own doc: all lanes probed
// concurrently, none skipped early), and returns rows in the same name
// order for deterministic output.
func probeProfiles(loaded config.Loaded, names []string, build profileClientBuilder) []providerRow {
	rows := make([]providerRow, 0, len(names))
	rowIndex := map[string]int{}
	lanes := map[string]llmclient.Pinger{}

	for _, name := range names {
		p, _, err := config.ResolveProfile(loaded.Config, loaded.Provenance, name)
		row := providerRow{Name: displayProfileName(name), Provider: p.Provider}
		if err != nil {
			row.Status = providerStatusError
			row.Detail = err.Error()
			rowIndex[name] = len(rows)
			rows = append(rows, row)
			continue
		}
		row.Provider = p.Provider
		cl, err := build(p)
		if err != nil {
			row.Status = providerStatusError
			row.Detail = err.Error()
			rowIndex[name] = len(rows)
			rows = append(rows, row)
			continue
		}
		if pinger, ok := cl.(llmclient.Pinger); ok {
			lanes[name] = pinger
		} else {
			row.Status = providerStatusNotProbeable
			row.Detail = fmt.Sprintf("provider %q's client does not implement Ping", p.Provider)
		}
		rowIndex[name] = len(rows)
		rows = append(rows, row)
	}

	results := llmclient.ProbeAll(context.Background(), lanes)
	for name := range lanes {
		i := rowIndex[name]
		if err, failed := results[name]; failed {
			rows[i].Status = providerStatusError
			rows[i].Detail = err.Error()
		} else {
			rows[i].Status = providerStatusOK
		}
	}
	return rows
}

func displayProfileName(name string) string {
	if name == "" {
		return "(env-overlay)"
	}
	return name
}

func printProvidersTable(w io.Writer, rows []providerRow) {
	fmt.Fprintf(w, "%-24s %-14s %-16s %s\n", "PROFILE", "PROVIDER", "STATUS", "DETAIL")
	for _, r := range rows {
		fmt.Fprintf(w, "%-24s %-14s %-16s %s\n", r.Name, r.Provider, r.Status, r.Detail)
	}
}

func printProvidersJSON(w io.Writer, rows []providerRow) {
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]any{
			"profile":  r.Name,
			"provider": r.Provider,
			"status":   string(r.Status),
			"detail":   r.Detail,
		})
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}

// providersRunner runs `fathom providers` (registered below; EW-45).
type providersRunner struct{}

func (providersRunner) execVerb(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return cmdProviders(args, stdout, stderr)
}

func init() {
	// SR-32: fathom doctor's providers row covers this; stays fully dispatched
	declareVerb(verb{Name: "providers", Group: "operate", Order: 110, Hidden: true, Run: providersRunner{},
		Summary: "Auth-health probe across configured LLM profiles (zero/near-zero tokens)"})
}

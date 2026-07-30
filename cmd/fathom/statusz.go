package main

// statusz.go — Q-25/Q-26 (docs/plans/expert-suite-q.md, docs/design/
// observability-surfaces.md §8 item 6): `GET /statusz` on serve-http —
// daemon health, per-service index freshness, channel liveness, last
// compile per layer. Loopback-only, read-only, gated inside the hosted-
// mode allowlist when `--modules-token-env` is configured
// (servehttp.go's modulesAuthMiddleware, extended there — the
// phase-O-critic-finding-#2 class this closes).

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/pkg/client"
)

// StatuszServiceRow is one indexed service's freshness + last-compile
// summary.
type StatuszServiceRow struct {
	Service       string `json:"service"`
	IndexedCommit string `json:"indexed_commit,omitempty"`
	FreshnessNote string `json:"freshness_note,omitempty"`
	// LastCompilePerLayer maps layer name -> a human-readable summary of
	// its most recent compile_runs row ("ran_at, model, cost").
	LastCompilePerLayer map[string]string `json:"last_compile_per_layer,omitempty"`
}

// StatuszReport is /statusz's payload — both the JSON and HTML renderings
// come from the exact same value.
type StatuszReport struct {
	GeneratedAt  string              `json:"generated_at"`
	UptimeSec    float64             `json:"uptime_seconds"`
	ChannelsNote string              `json:"channels_note"`
	Services     []StatuszServiceRow `json:"services"`
}

// buildStatuszReport is the pure-given-its-client core: it reads whatever
// the already-open client exposes (Services, ServiceMeta, CompileRunsFor)
// — no new store connection, no daemon round trip beyond what the caller
// already has open. Channel liveness reuses fathomChannelsDisabled's OWN
// env read — the SAME honesty caveat checkChannelsEnv already carries:
// this reports the SERVER process's own environment, not the harness's
// (the harness's hooks run in a different process entirely).
func buildStatuszReport(ctx context.Context, c *client.Client, startTime, now time.Time) StatuszReport {
	r := StatuszReport{
		GeneratedAt: now.UTC().Format(time.RFC3339),
		UptimeSec:   now.Sub(startTime).Seconds(),
	}
	if fathomChannelsDisabled() {
		r.ChannelsNote = "FATHOM_CHANNELS=off in this SERVER process's own environment — this does not necessarily reflect the harness's own hook environment"
	} else {
		r.ChannelsNote = "FATHOM_CHANNELS is not \"off\" in this server process's own environment"
	}

	services, err := c.Services(ctx)
	if err != nil {
		return r
	}
	sort.Strings(services)
	for _, svc := range services {
		row := StatuszServiceRow{Service: svc}
		if path, commit, found, merr := c.ServiceMeta(ctx, svc); merr == nil && found {
			row.IndexedCommit = commit
			_ = path
		} else {
			row.FreshnessNote = "not measurable: no ServiceMeta recorded for this service"
		}
		if runs, rerr := c.CompileRunsFor(ctx, svc); rerr == nil && len(runs) > 0 {
			// runs is newest-first (Store.CompileRuns' own contract) —
			// the FIRST run seen per layer is that layer's most recent.
			seen := map[string]bool{}
			perLayer := map[string]string{}
			for _, run := range runs {
				if seen[run.Layer] {
					continue
				}
				seen[run.Layer] = true
				perLayer[run.Layer] = fmt.Sprintf("ran_at=%s model=%s cost=$%.4f",
					run.RanAt.UTC().Format(time.RFC3339), run.Model, run.CostUSD)
			}
			row.LastCompilePerLayer = perLayer
		}
		r.Services = append(r.Services, row)
	}
	return r
}

// statuszHandler serves buildStatuszReport as JSON (Accept: application/json
// or ?format=json) or a minimal, dependency-free HTML page otherwise — no
// template engine, no new import.
func statuszHandler(c *client.Client, startTime time.Time) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		report := buildStatuszReport(r.Context(), c, startTime, time.Now())

		wantJSON := r.URL.Query().Get("format") == "json" || strings.Contains(r.Header.Get("Accept"), "application/json")
		if wantJSON {
			w.Header().Set("Content-Type", "application/json")
			enc := json.NewEncoder(w)
			enc.SetIndent("", "  ")
			_ = enc.Encode(report)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, "<!doctype html><html><head><title>fathom statusz</title></head><body>")
		fmt.Fprintf(w, "<h1>fathom statusz</h1>")
		fmt.Fprintf(w, "<p>generated_at: %s</p>", htmlEscape(report.GeneratedAt))
		fmt.Fprintf(w, "<p>uptime_seconds: %.1f</p>", report.UptimeSec)
		fmt.Fprintf(w, "<p>channels: %s</p>", htmlEscape(report.ChannelsNote))
		fmt.Fprintf(w, "<h2>services</h2><ul>")
		for _, s := range report.Services {
			fmt.Fprintf(w, "<li><b>%s</b> — indexed_commit=%s", htmlEscape(s.Service), htmlEscape(s.IndexedCommit))
			if s.FreshnessNote != "" {
				fmt.Fprintf(w, " (%s)", htmlEscape(s.FreshnessNote))
			}
			if len(s.LastCompilePerLayer) > 0 {
				layers := make([]string, 0, len(s.LastCompilePerLayer))
				for layer := range s.LastCompilePerLayer {
					layers = append(layers, layer)
				}
				sort.Strings(layers)
				fmt.Fprintf(w, "<ul>")
				for _, layer := range layers {
					fmt.Fprintf(w, "<li>%s: %s</li>", htmlEscape(layer), htmlEscape(s.LastCompilePerLayer[layer]))
				}
				fmt.Fprintf(w, "</ul>")
			}
			fmt.Fprintf(w, "</li>")
		}
		fmt.Fprintf(w, "</ul></body></html>")
	}
}

// htmlEscape is a minimal, dependency-free HTML text-escaper — statusz's
// values are all fathom-controlled (service names, models, notes), but
// escaping costs nothing and closes the door on any future value that
// isn't.
func htmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return r.Replace(s)
}

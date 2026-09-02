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

	"github.com/geoffmilleraz/fathom/internal/buildinfo"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// statuszSchemaVersion identifies the /statusz payload contract. "2" adds
// the daemon's own build stamp (binary_commit, binary_dirty, built_at —
// PC-1, vision 26 §1.1) so a CLI can tell which binary built the store it
// is about to trust; v1 carried no schema_version at all. Additive: every
// v1 field keeps its name and meaning.
const statuszSchemaVersion = 2

// StatuszBuildInfo is the daemon binary's own identity as /statusz reports
// it — the daemon's stamp, never the CLI's or the repo's HEAD (O5).
type StatuszBuildInfo struct {
	BinaryCommit string `json:"binary_commit"`
	BinaryDirty  bool   `json:"binary_dirty"`
	BuiltAt      string `json:"built_at,omitempty"`
}

// statuszBuildInfo reads the running daemon binary's VCS stamp from
// internal/buildinfo. An unstamped binary (built without -buildvcs) yields
// an empty commit, which readers treat as "unknown", never as fresh.
func statuszBuildInfo() StatuszBuildInfo {
	st := buildinfo.Read()
	return StatuszBuildInfo{BinaryCommit: st.Revision, BinaryDirty: st.Dirty, BuiltAt: st.BuiltAt}
}

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
	SchemaVersion int    `json:"schema_version"`
	GeneratedAt   string `json:"generated_at"`
	StatuszBuildInfo
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
		SchemaVersion:    statuszSchemaVersion,
		GeneratedAt:      now.UTC().Format(time.RFC3339),
		StatuszBuildInfo: statuszBuildInfo(),
		UptimeSec:        now.Sub(startTime).Seconds(),
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
		fmt.Fprintf(w, "<!doctype html><html><head><meta charset=\"utf-8\"><title>fathom statusz</title>%s</head><body>", statuszCSS)
		fmt.Fprintf(w, "<header><h1>fathom <span class=\"sub\">statusz</span></h1>")
		fmt.Fprintf(w, "<dl class=\"meta\"><dt>generated</dt><dd>%s</dd><dt>uptime</dt><dd>%.1fs</dd><dt>binary</dt><dd>%s</dd><dt>channels</dt><dd>%s</dd></dl></header>",
			htmlEscape(report.GeneratedAt), report.UptimeSec, htmlEscape(statuszBinaryLabel(report.StatuszBuildInfo)), htmlEscape(report.ChannelsNote))
		fmt.Fprintf(w, "<h2>services <span class=\"count\">%d</span></h2><div class=\"grid\">", len(report.Services))
		for _, s := range report.Services {
			fmt.Fprintf(w, "<section class=\"svc\"><h3>%s</h3>", htmlEscape(s.Service))
			fmt.Fprintf(w, "<p class=\"commit\">%s</p>", htmlEscape(s.IndexedCommit))
			if s.FreshnessNote != "" {
				fmt.Fprintf(w, "<p class=\"note\">%s</p>", htmlEscape(s.FreshnessNote))
			}
			if len(s.LastCompilePerLayer) > 0 {
				layers := make([]string, 0, len(s.LastCompilePerLayer))
				for layer := range s.LastCompilePerLayer {
					layers = append(layers, layer)
				}
				sort.Strings(layers)
				fmt.Fprintf(w, "<ul class=\"layers\">")
				for _, layer := range layers {
					fmt.Fprintf(w, "<li><b>%s</b> %s</li>", htmlEscape(layer), htmlEscape(s.LastCompilePerLayer[layer]))
				}
				fmt.Fprintf(w, "</ul>")
			} else {
				fmt.Fprintf(w, "<p class=\"none\">no compiled layers</p>")
			}
			fmt.Fprintf(w, "</section>")
		}
		fmt.Fprintf(w, "</div><footer>read-only diagnostic · JSON at this same path with <code>Accept: application/json</code></footer></body></html>")
	}
}

// statuszBinaryLabel renders the build stamp for the HTML view: the commit
// (short), "-dirty" when built from a modified tree, or "unstamped".
func statuszBinaryLabel(b StatuszBuildInfo) string {
	if b.BinaryCommit == "" {
		return "unstamped (built without -buildvcs)"
	}
	label := b.BinaryCommit
	if len(label) > 12 {
		label = label[:12]
	}
	if b.BinaryDirty {
		label += "-dirty"
	}
	if b.BuiltAt != "" {
		label += " built " + b.BuiltAt
	}
	return label
}

// statuszCSS is deliberately hand-written and inline.
//
// The fleet convention for real UI is the licensed corpus (TailwindPlus /
// MatDash via the appkit MCP) rendered through app-kit's pure-Go ui
// components — and that is exactly what fathom-studio's web dashboard does
// (docs/design/observability-surfaces.md §5). It is the WRONG trade here:
// /statusz is served by the fathom daemon binary, whose doctrine is one
// pure-Go binary with no asset pipeline. Adopting the corpus would mean an
// app-kit dependency plus a Tailwind build step inside the daemon, to style
// a read-only diagnostic whose actual contract is the JSON at the same path.
// So: ~40 inline lines here, and the corpus where it earns its keep.
const statuszCSS = `<style>
:root{color-scheme:light dark;--bg:#fbfcfd;--fg:#101418;--dim:#5b6773;--line:#e3e8ec;--card:#fff;--accent:#0b6b7a}
@media(prefers-color-scheme:dark){:root{--bg:#0d1216;--fg:#e8edf1;--dim:#8b9aa6;--line:#1e262d;--card:#131a20;--accent:#4fc3d4}}
*{box-sizing:border-box}
body{margin:0;padding:32px 24px 48px;background:var(--bg);color:var(--fg);
font:14px/1.55 system-ui,-apple-system,"Segoe UI",sans-serif}
header{max-width:1100px;margin:0 auto 28px}
h1{margin:0 0 12px;font-size:22px;font-weight:600;letter-spacing:-.01em}
h1 .sub{color:var(--dim);font-weight:400}
h2{max-width:1100px;margin:0 auto 14px;font-size:13px;font-weight:600;
text-transform:uppercase;letter-spacing:.08em;color:var(--dim)}
h2 .count{color:var(--accent)}
dl.meta{display:grid;grid-template-columns:auto 1fr;gap:2px 14px;margin:0;
font-size:12.5px}
dl.meta dt{color:var(--dim)}
dl.meta dd{margin:0;overflow-wrap:anywhere}
.grid{max-width:1100px;margin:0 auto;display:grid;gap:12px;
grid-template-columns:repeat(auto-fill,minmax(320px,1fr))}
.svc{background:var(--card);border:1px solid var(--line);border-radius:10px;padding:16px 18px}
.svc h3{margin:0 0 6px;font-size:15px;font-weight:600;overflow-wrap:anywhere}
.commit{margin:0;font-family:ui-monospace,SFMono-Regular,Menlo,monospace;
font-size:11.5px;color:var(--dim);overflow-wrap:anywhere}
.note{margin:8px 0 0;font-size:12.5px;color:var(--accent)}
.none{margin:10px 0 0;font-size:12.5px;color:var(--dim);font-style:italic}
ul.layers{margin:10px 0 0;padding:0;list-style:none;display:grid;gap:5px}
ul.layers li{font-size:12px;color:var(--dim);
font-family:ui-monospace,SFMono-Regular,Menlo,monospace;overflow-wrap:anywhere}
ul.layers b{color:var(--fg);font-weight:600}
footer{max-width:1100px;margin:28px auto 0;font-size:12px;color:var(--dim)}
footer code{font-size:11.5px}
</style>`

// htmlEscape is a minimal, dependency-free HTML text-escaper — statusz's
// values are all fathom-controlled (service names, models, notes), but
// escaping costs nothing and closes the door on any future value that
// isn't.
func htmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return r.Replace(s)
}

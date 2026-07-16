package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/analysis/security"
	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/store"
	"github.com/geoffmilleraz/fathom/pkg/client"
)

// cmdReport implements the "report" subcommand: the deterministic bridge (D3
// FT-5, docs/vision/03-findings-and-experts.md §3): "for 'analyzer results as
// a baseline,' don't persist analyzer structs — add a verb that writes an
// analyzer's output as a markdown finding document into a corpus directory."
// The doc then flows through the ordinary ingest → compile → claims pipeline
// (findings layer, tier verified once FT-7 lands) instead of inventing a
// second persistence path for the Finding{File,Line,Rule,Severity} shape
// every deterministic analyzer already returns.
//
// v0 supports exactly one analyzer — security_rollup — mirroring
// internal/mcp/extra_tools.go's registerSecurityRollup (secret_scan +
// shell_out + http_client_calls over the service's sources; license posture
// is deliberately excluded here: it is not a file:line-anchored finding, so
// it does not fit the Finding shape this bridge unifies).
//
//	fathom report --store DB --service S --analyzer security_rollup --out findings/
//
// Output is exactly ONE file, "<service>-<analyzer>.md", inside --out.
// Generation is a pure function of the current index — no clock reads, no
// random ordering — so re-running it against an unchanged index reproduces
// the identical bytes (asserted by report_test.go's double-run compare); the
// findings compiler's own digest gate depends on that determinism to avoid
// spurious recompiles.
func cmdReport(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("report", stderr)
	storePath := fs.String("store", "", "path to the DuckDB store (required)")
	service := fs.String("service", "", "service ID (required)")
	analyzer := fs.String("analyzer", "security_rollup", "analyzer whose output to bridge (v0: security_rollup)")
	out := fs.String("out", "findings", "output directory (created if missing)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *storePath == "" || *service == "" {
		fmt.Fprintln(stderr, "error: report requires --store and --service")
		fs.Usage()
		return 1
	}
	if *analyzer != "security_rollup" {
		fmt.Fprintf(stderr, "error: unsupported --analyzer %q (v0 supports: security_rollup)\n", *analyzer)
		return 1
	}

	st, err := store.Open(*storePath)
	if err != nil {
		fmt.Fprintf(stderr, "error: open store %q: %v\n", *storePath, err)
		return 1
	}
	defer st.Close()

	c := client.New(connector.NewLocalConnector(), st)
	defer c.Close()

	ctx := context.Background()
	files, err := c.Sources(ctx, *service)
	if err != nil {
		fmt.Fprintf(stderr, "error: fetch sources for %q: %v\n", *service, err)
		return 1
	}

	doc := renderSecurityRollupFindings(*service, files)

	if err := os.MkdirAll(*out, 0o755); err != nil {
		fmt.Fprintf(stderr, "error: mkdir %q: %v\n", *out, err)
		return 1
	}
	outPath := filepath.Join(*out, *service+"-"+*analyzer+".md")
	if err := os.WriteFile(outPath, []byte(doc), 0o644); err != nil {
		fmt.Fprintf(stderr, "error: write %q: %v\n", outPath, err)
		return 1
	}

	fmt.Fprintf(stdout, "wrote %s (%d bytes)\n", outPath, len(doc))
	return 0
}

// securityRollupSection is one finding category the security_rollup bridge
// emits as a single markdown section (a "finding" in the review-findings
// module's sense, docs/design/expert-modules.md §3.2 — severity, area,
// affected paths, recommendation — not one section per raw scanner hit,
// which would make an 80-hit scan an 80-section document).
type securityRollupSection struct {
	id       string // stable finding ID, e.g. "SEC-001"
	heading  string
	remedy   string // the actionable statement — kept <= 28 words for the findings compiler's rule budget
	findings []security.Finding
}

// renderSecurityRollupFindings renders the security_rollup bridge document
// deterministically: category order is fixed (declared below, not derived
// from map iteration), and each category's findings are sorted by
// (File, Line, Rule) before being listed.
func renderSecurityRollupFindings(service string, files []core.SourceFile) string {
	var secrets, shellOuts, httpCalls []security.Finding
	for _, f := range files {
		secrets = append(secrets, security.SecretScan(f.Path, f.Content)...)
		shellOuts = append(shellOuts, security.ShellOut(f.Path, f.Content)...)
		httpCalls = append(httpCalls, security.HTTPClientCalls(f.Path, f.Content)...)
	}

	sections := []securityRollupSection{
		{
			id:       "SEC-001",
			heading:  "Hardcoded secrets",
			remedy:   "Remove hardcoded credentials from source and rotate any exposed secret before the next release.",
			findings: secrets,
		},
		{
			id:       "SEC-002",
			heading:  "Unchecked shell-outs",
			remedy:   "Replace unchecked shell-out calls with a validated, allow-listed command path.",
			findings: shellOuts,
		},
		{
			id:       "SEC-003",
			heading:  "Unsafe HTTP client usage",
			remedy:   "Audit unsafe HTTP client calls for TLS verification and explicit timeout configuration.",
			findings: httpCalls,
		},
	}

	var b strings.Builder
	fmt.Fprintf(&b, "# security_rollup findings — %s\n\n", service)
	b.WriteString("Generated by `fathom report --analyzer security_rollup`. Do not edit by hand — re-run the report to refresh from the current index.\n")

	for _, sec := range sections {
		if len(sec.findings) == 0 {
			continue
		}
		found := append([]security.Finding(nil), sec.findings...)
		sort.Slice(found, func(i, j int) bool {
			if found[i].File != found[j].File {
				return found[i].File < found[j].File
			}
			if found[i].Line != found[j].Line {
				return found[i].Line < found[j].Line
			}
			return found[i].Rule < found[j].Rule
		})

		fmt.Fprintf(&b, "\n## %s: %s\n\n", sec.id, sec.heading)
		fmt.Fprintf(&b, "Severity: high. Area: security.\n%s\n\n", sec.remedy)
		b.WriteString("Affected:\n")
		for _, find := range found {
			fmt.Fprintf(&b, "- `%s:%d` (%s) — %s\n", find.File, find.Line, find.Rule, find.Message)
		}
	}
	return b.String()
}

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
// v0 supports three analyzers:
//
//   - security_rollup (default) — mirrors internal/mcp/extra_tools.go's
//     registerSecurityRollup (secret_scan + shell_out + http_client_calls
//     over the service's sources; license posture is deliberately excluded
//     here: it is not a file:line-anchored finding, so it does not fit the
//     Finding shape this bridge unifies).
//
//   - authorship — E2-3 (docs/plans/new-use-cases.md): AI co-authorship
//     share × staleness × complexity, and the only analyzer that also
//     persists a claims ledger (see report_authorship.go).
//
//   - reverse-drift — E6-1 (docs/plans/new-use-cases.md): public API
//     surface minus spec-bindings coverage = ungoverned surface, per
//     service (see report_reversedrift.go). Read-only, like security_rollup.
//
//     fathom report --store DB --service S --analyzer security_rollup --out findings/
//
// Output is exactly ONE file, "<service>-<analyzer>.md", inside --out.
// Generation is a pure function of the corpus working tree at run time
// (Sources() re-reads the corpus from disk, not the index) — no clock reads,
// no random ordering — so re-running it against an unchanged tree reproduces
// the identical bytes (asserted by report_test.go's double-run compare); the
// findings compiler's own digest gate depends on that determinism to avoid
// spurious recompiles.
//
// Self-scan exclusion: --out normally points INSIDE the corpus (that is the
// point — the doc joins the findings layer's corpus), so source files under
// the resolved --out directory are excluded from the scan. Without this the
// second run scans the first run's doc (whose shell-out/http finding messages
// embed the matched snippets), gains findings anchored in the report itself,
// and every run mutates the doc — shifting line numbers, defeating the
// findings compiler's digest gate, and flipping compiled claims stale on
// every refresh.
func cmdReport(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("report", stderr)
	storePath := fs.String("store", "", "path to the store, .duckdb or .sqlite/.db (required)")
	service := fs.String("service", "", "service ID (required)")
	analyzer := fs.String("analyzer", "security_rollup", "analyzer whose output to bridge (security_rollup, authorship, reverse-drift)")
	out := fs.String("out", "findings", "output directory (created if missing)")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if *storePath == "" || *service == "" {
		fmt.Fprintln(stderr, "error: report requires --store and --service")
		fs.Usage()
		return 1
	}
	if *analyzer != "security_rollup" && *analyzer != "authorship" && *analyzer != "reverse-drift" {
		fmt.Fprintf(stderr, "error: unsupported --analyzer %q (supports: security_rollup, authorship, reverse-drift)\n", *analyzer)
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

	var doc string
	switch *analyzer {
	case "authorship":
		doc, err = runAuthorshipReport(ctx, st, *service)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
	case "reverse-drift":
		doc, err = runReverseDriftReport(ctx, st, *service)
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
	default: // security_rollup
		files, ferr := c.Sources(ctx, *service)
		if ferr != nil {
			fmt.Fprintf(stderr, "error: fetch sources for %q: %v\n", *service, ferr)
			return 1
		}
		// Self-scan exclusion (see the command doc above): drop every source
		// file under the resolved --out directory before scanning. Sources()
		// paths are repo-relative (ADR-017), so resolve --out against the
		// service root recorded at ingest and compare repo-relative prefixes.
		if meta, found, merr := st.GetServiceMeta(ctx, *service); merr == nil && found && meta.Path != "" {
			files = excludeUnderOut(files, meta.Path, *out)
		}
		doc = renderSecurityRollupFindings(*service, files)
	}

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

// excludeUnderOut returns files minus everything under the resolved --out
// directory. root is the service's recorded corpus root; out is the --out
// flag as given (absolute or CWD-relative). When out resolves outside root
// there is nothing to exclude.
func excludeUnderOut(files []core.SourceFile, root, out string) []core.SourceFile {
	rootAbs, err1 := filepath.Abs(root)
	outAbs, err2 := filepath.Abs(out)
	if err1 != nil || err2 != nil {
		return files
	}
	rel, err := filepath.Rel(rootAbs, outAbs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return files // --out is outside the corpus: nothing to exclude
	}
	prefix := filepath.ToSlash(rel)
	if prefix == "." || prefix == "" {
		// --out IS the corpus root: exclude only prior report docs would need
		// per-file knowledge; scanning the whole corpus away would render the
		// report useless. Keep everything — the supported deployment is a
		// subdirectory (e.g. findings/).
		return files
	}
	kept := files[:0]
	for _, f := range files {
		if f.Path == prefix || strings.HasPrefix(f.Path, prefix+"/") {
			continue
		}
		kept = append(kept, f)
	}
	return kept
}

// securityRollupSection is one finding category the security_rollup bridge
// emits as a single markdown section (a "finding" in the review-findings
// module's sense, docs/design/expert-modules.md §3.2 — severity, area,
// affected paths, recommendation — not one section per raw scanner hit,
// which would make an 80-hit scan an 80-section document).
type securityRollupSection struct {
	id      string // stable finding ID, e.g. "SEC-001"
	heading string
	remedy  string // the actionable statement — kept <= 28 words for the findings compiler's rule budget
	// redactMessages drops the scanner Message from every listed hit: the
	// secret scanners embed the matched source line verbatim (e.g.
	// `credential pattern "password = \"...\"" found`), and a committable
	// finding document must never exfiltrate the credential it reports.
	// Redaction lives here in the bridge, not in the analyzer — interactive
	// consumers (the security_rollup MCP tool) legitimately need the context.
	redactMessages bool
	findings       []security.Finding
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
			id:             "SEC-001",
			heading:        "Hardcoded secrets",
			remedy:         "Remove hardcoded credentials from source and rotate any exposed secret before the next release.",
			redactMessages: true, // secret messages embed the matched line — never write the credential into the doc
			findings:       secrets,
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
	b.WriteString("Generated by `fathom report --analyzer security_rollup`. Do not edit by hand — re-run the report to refresh from the current working tree.\n")

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
			if sec.redactMessages {
				fmt.Fprintf(&b, "- `%s:%d` (%s)\n", find.File, find.Line, find.Rule)
			} else {
				fmt.Fprintf(&b, "- `%s:%d` (%s) — %s\n", find.File, find.Line, find.Rule, find.Message)
			}
		}
	}
	return b.String()
}

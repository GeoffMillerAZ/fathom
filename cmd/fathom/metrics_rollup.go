package main

// metrics_rollup.go — Q-21 (docs/plans/expert-suite-q.md, docs/design/
// observability-surfaces.md §4): `fathom metrics rollup` builds one date's
// daily rollup file; `fathom metrics rotate` is the SEPARATE, explicit
// call-log retention verb ("never delete raw data implicitly" — a
// dry-run report by default, --apply required to actually archive).

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/metrics"
)

// defaultMetricsRollupDir is where `fathom metrics rollup` writes and
// `fathom metrics series` reads (repo-scoped, under the workspace/
// convention).
const defaultMetricsRollupDir = "workspace/artifacts/metrics/daily"

func cmdMetricsRollupBuild(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("metrics rollup", stderr)
	dateFlag := fs.String("date", "", "date to roll up, YYYY-MM-DD (default: today, UTC)")
	root := fs.String("root", ".", "project root — --out and layer-pack budget lookups resolve under here")
	out := fs.String("out", "", "output directory for the rollup file; default: --root/"+defaultMetricsRollupDir)
	storePath := fs.String("store", "", "local store path (optional — omit to skip the spend/review-superseded rows)")
	service := fs.String("service", "", "service ID (used only with --store)")
	module := fs.String("module", "", "module/layer filter (used only with --store)")
	callLogFlag := fs.String("call-log", "", "MCP tool-call log (JSONL); default: [daemon].call_log in config.toml, else "+defaultCallLogPath())
	hooksLogFlag := fs.String("hooks-log", "", "hooks beat log (JSONL); default: $FATHOM_HOOKS_LOG or ~/.fathom/hooks.jsonl")
	jsonOut := fs.Bool("json", false, "emit JSON instead of a human summary")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	now := time.Now().UTC()
	date := now
	if *dateFlag != "" {
		parsed, err := time.Parse("2006-01-02", *dateFlag)
		if err != nil {
			fmt.Fprintf(stderr, "error: --date must be YYYY-MM-DD: %v\n", err)
			return 1
		}
		date = parsed
	}
	sinceT := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
	untilT := sinceT.Add(24*time.Hour - time.Nanosecond)

	runs, rErr := readCompileRunsFor(*storePath, *service, *module)
	if rErr != nil {
		fmt.Fprintf(stderr, "error: %v\n", rErr)
		return 1
	}
	spend := metrics.AggregateSpend(runs, spendBudgetLookup(*root), metrics.SpendOptions{Since: sinceT, Until: untilT, GroupBy: metrics.SpendGroupByLayer})

	// OC-2: the one call-log resolution (console_calllog.go) — the daemon's
	// configured log when a daemon record names one, never a log relative
	// to this command's working directory.
	resolved := resolveConsoleCallLog(*callLogFlag, "")
	callLogPath := resolved.Path
	entries, verdicts := readCallDataFor(callLogPath, *service, sinceT, untilT)
	traffic := metrics.AggregateTraffic(entries, metrics.TrafficOptions{Since: sinceT, Until: untilT, GroupBy: metrics.TrafficGroupBySurface})

	hooksLogPath := *hooksLogFlag
	if hooksLogPath == "" {
		hooksLogPath = beatLogPath()
	}
	beats := readBeatsFor(hooksLogPath)
	channels := metrics.AggregateChannels(beats, sinceT, untilT)

	claimEvents, claimEventsQueried := readClaimEventsFor(*storePath, sinceT, untilT, stderr)
	interventions := metrics.AggregateInterventions(verdicts, beats, claimEvents, claimEventsQueried, sinceT, untilT)

	rollup := metrics.BuildDailyRollup(date, spend, traffic, channels, interventions, now)

	outDir := *out
	if outDir == "" {
		outDir = filepath.Join(*root, filepath.FromSlash(defaultMetricsRollupDir))
	}
	if err := metrics.WriteDailyRollup(outDir, rollup); err != nil {
		fmt.Fprintf(stderr, "error: writing rollup: %v\n", err)
		return 1
	}

	if *jsonOut {
		jsonPrint(stdout, withCallLog(rollup, resolved))
	} else {
		fmt.Fprintf(stdout, "wrote %s\n", filepath.Join(outDir, rollup.Date+".json"))
		fmt.Fprintln(stdout, callLogLine(resolved))
	}
	return 0
}

// cmdMetricsRotate is the explicit, dry-run-by-default call-log retention
// verb (design §4: "never delete raw data implicitly"). Without --apply it
// only REPORTS how many lines are older than --keep-days; --apply moves
// them to a dated archive sibling file, never deletes, never touches the
// archive on a re-run for a DIFFERENT cutoff (each cutoff gets its own
// archive file, named by that run's date — an existing archive is never
// overwritten or merged).
func cmdMetricsRotate(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("metrics rotate", stderr)
	callLogFlag := fs.String("call-log", "", "MCP tool-call log (JSONL); default: the one resolution every metrics verb shares — the daemon's recorded log, then [daemon].call_log, then "+defaultCallLogPath())
	keepDays := fs.Int("keep-days", 90, "lines older than this many days (by their own ts field) are candidates for archiving")
	apply := fs.Bool("apply", false, "actually archive the matched lines (default: dry-run report only)")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	// OC-2: the retention verb inspects (and with --apply rewrites) the log
	// every other metrics verb and page reads — the daemon's record first —
	// never a path relative to this command's working directory.
	resolved := resolveConsoleCallLog(*callLogFlag, "")
	callLogPath := resolved.Path
	fmt.Fprintln(stdout, callLogLine(resolved))
	if callLogPath == "" {
		fmt.Fprintln(stdout, "no call log is configured — nothing to rotate")
		return 0
	}
	cutoff := time.Now().UTC().AddDate(0, 0, -*keepDays)

	f, err := os.Open(callLogPath)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Fprintf(stdout, "no call log at %s — nothing to rotate\n", callLogPath)
			return 0
		}
		fmt.Fprintf(stderr, "error: call log %s is unreadable: %v\n", callLogPath, err)
		return 1
	}
	var keep, archive []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		var e adoptionCallEntry
		if jerr := json.Unmarshal([]byte(trimmed), &e); jerr != nil {
			keep = append(keep, line) // malformed line: kept, never silently dropped
			continue
		}
		ts, ok := parseCallLogTS(e.TS)
		if !ok || !ts.Before(cutoff) {
			keep = append(keep, line)
			continue
		}
		archive = append(archive, line)
	}
	f.Close()
	if serr := sc.Err(); serr != nil {
		fmt.Fprintf(stderr, "error: scanning %s: %v\n", callLogPath, serr)
		return 1
	}

	if len(archive) == 0 {
		fmt.Fprintf(stdout, "0 lines older than %s (keep-days=%d) — nothing to rotate\n", cutoff.Format("2006-01-02"), *keepDays)
		return 0
	}
	if !*apply {
		fmt.Fprintf(stdout, "%d line(s) older than %s (keep-days=%d) would be archived (dry run — pass --apply to actually rotate)\n", len(archive), cutoff.Format("2006-01-02"), *keepDays)
		return 0
	}

	archivePath := callLogPath + ".archive-" + time.Now().UTC().Format("2006-01-02")
	if _, err := os.Stat(archivePath); err == nil {
		fmt.Fprintf(stderr, "error: archive path %s already exists — refusing to overwrite (run again tomorrow, or move it aside)\n", archivePath)
		return 1
	}
	if err := os.WriteFile(archivePath, []byte(strings.Join(archive, "\n")+"\n"), 0o644); err != nil {
		fmt.Fprintf(stderr, "error: writing archive %s: %v\n", archivePath, err)
		return 1
	}
	keptContent := ""
	if len(keep) > 0 {
		keptContent = strings.Join(keep, "\n") + "\n"
	}
	if err := os.WriteFile(callLogPath, []byte(keptContent), 0o644); err != nil {
		fmt.Fprintf(stderr, "error: rewriting %s: %v (archive at %s is intact)\n", callLogPath, err, archivePath)
		return 1
	}
	fmt.Fprintf(stdout, "archived %d line(s) to %s; %d line(s) kept in %s\n", len(archive), archivePath, len(keep), callLogPath)
	return 0
}

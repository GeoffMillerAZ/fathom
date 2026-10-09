package main

// decidelog.go — `fathom decide log`: the decision port's provider and
// sites, its ruling log and the latest calibration table (OC-45, vision 35
// §8; ADR-042). The console's /decide page renders the same projection
// (console.DecideView) from the same sources.
//
// The ruling log is decisions.jsonl beside the resolved call log
// (internal/decide.DefaultRecorderPath) — where the port the daemon opened
// records every ruling. The calibration is the newest
// docs/bench/readings/decision-port-calibration-*.md under a registered
// service's root (or --readings), parsed, never recomputed: the log carries
// no gold labels to grade a ruling with.

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/config"
	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/decide"
)

// decideConfigOf is the [decide] block as the page shows it.
func decideConfigOf(loaded config.Loaded, loadErr error) console.DecideConfig {
	d := loaded.Config.Decide
	c := console.DecideConfig{Provider: d.Provider, Model: d.Model, Endpoint: d.EffectiveBaseURL(), Sites: append([]string{}, d.Sites...), RouteConfidence: d.RouteConfidence}
	if c.Provider == "" {
		c.Provider = "none"
	}
	var from []string
	if loaded.UserFound {
		from = append(from, loaded.UserPath)
	}
	if loaded.ProjectFound {
		from = append(from, loaded.ProjectPath)
	}
	switch {
	case loadErr != nil:
		c.Source = "config did not load (" + loadErr.Error() + ") — the built-in defaults"
	case len(from) == 0:
		c.Source = "the built-in defaults (no config file found)"
	default:
		c.Source = strings.Join(from, " + ")
	}
	return c
}

// calibrationReadingGlob names the readings the calibration comes from.
const calibrationReadingGlob = "decision-port-calibration-*.md"

// findCalibrationReading is the newest calibration reading: under dir when
// given, else under docs/bench/readings of every registered root (the
// newest by its dated file name, then by root). reason names the absence.
func findCalibrationReading(dir string, roots []string) (path, reason string) {
	var candidates []string
	if dir != "" {
		candidates, _ = filepath.Glob(filepath.Join(dir, calibrationReadingGlob))
	} else {
		sorted := append([]string{}, roots...)
		sort.Strings(sorted)
		for _, r := range sorted {
			if r == "" {
				continue
			}
			m, _ := filepath.Glob(filepath.Join(r, "docs", "bench", "readings", calibrationReadingGlob))
			candidates = append(candidates, m...)
		}
	}
	if len(candidates) == 0 {
		where := "under docs/bench/readings of any registered service"
		if dir != "" {
			where = "in " + dir
		}
		return "", "no decision-port calibration reading (" + calibrationReadingGlob + ") " + where + " — calibration is not measured here, never assumed"
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		bi, bj := filepath.Base(candidates[i]), filepath.Base(candidates[j])
		if bi != bj {
			return bi > bj
		}
		return candidates[i] < candidates[j]
	})
	return candidates[0], ""
}

// decideSources is what DecideView is built from.
type decideSources struct {
	cfg         console.DecideConfig
	logPath     string
	log         []byte
	logErr      error
	readingPath string
	reading     []byte
	readingErr  error
	readingNote string
}

// readDecideSources reads the ruling log beside the resolved call log and
// the newest calibration reading.
func readDecideSources(resolved console.ResolvedCallLog, readingsDir string, roots []string) decideSources {
	loaded, err := config.Load(config.DefaultUserPath(), config.DefaultProjectPath())
	s := decideSources{cfg: decideConfigOf(loaded, err), logPath: decide.DefaultRecorderPath(resolved.Path)}
	if s.logPath != "" {
		s.log, s.logErr = os.ReadFile(s.logPath)
	}
	s.readingPath, s.readingNote = findCalibrationReading(readingsDir, roots)
	if s.readingPath != "" {
		s.reading, s.readingErr = os.ReadFile(s.readingPath)
	}
	return s
}

// decideInputsFrom adapts the sources to DecideView's inputs.
func decideInputsFrom(s decideSources) console.DecideInputs {
	in := console.DecideInputs{Config: s.cfg, AllSites: decide.AllSites, LogPath: s.logPath, Log: s.log, ReadingReason: s.readingNote}
	if s.logPath == "" {
		in.LogReason = "no call log is resolved, so no decisions log beside it — rulings are not measured"
	} else {
		in.LogReason, in.LogError = fileState("decisions log", s.logPath, s.logErr)
		if in.LogReason != "" && !in.LogError {
			in.LogReason = "no decisions log at " + s.logPath + " — the port has recorded no ruling there (it writes only once a provider answers); not zero"
		}
	}
	switch {
	case s.readingPath == "":
	case s.readingErr != nil:
		in.ReadingReason = "calibration reading unreadable: " + s.readingErr.Error()
	default:
		if t, ok := console.ParseCalibrationReading(s.readingPath, s.reading); ok {
			in.Calibration = &t
		} else {
			in.ReadingReason = "the newest calibration reading (" + s.readingPath + ") carries no confidence-bucket table"
		}
	}
	return in
}

func cmdDecide(args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 && args[0] == "log" {
		return cmdDecideLog(args[1:], stdout, stderr)
	}
	fmt.Fprintln(stderr, "usage: fathom decide log [--site S] [--page N] [--per-page N] [--readings DIR] [--call-log PATH] [--url U] [--json]")
	return 1
}

func cmdDecideLog(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("decide log", stderr)
	callLogFlag := fs.String("call-log", "", "the call log whose directory holds decisions.jsonl; default: the daemon's configured log (its record)")
	url := fs.String("url", defaultFathomURL(), "the daemon whose record names the call log and whose registry roots are searched for the calibration reading")
	readings := fs.String("readings", "", "directory holding "+calibrationReadingGlob+" (default: docs/bench/readings under every registered service root)")
	site := fs.String("site", "", "keep one site's rulings")
	page := fs.Int("page", 1, "page of rulings (newest first)")
	perPage := fs.Int("per-page", 50, "rulings per page")
	jsonOut := fs.Bool("json", false, "emit JSON")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	resolved := resolveConsoleCallLogFor(callLogQuery{Flag: *callLogFlag, BaseURL: *url})
	var roots []string
	if *readings == "" {
		if svcs, err := fetchServices(&http.Client{Timeout: 5 * time.Second}, *url); err == nil {
			for _, info := range svcs {
				roots = append(roots, info.path)
			}
		}
	}
	m := console.DecideView(decideInputsFrom(readDecideSources(resolved, *readings, roots)), console.DecideFilter{Site: *site}, *page, *perPage)
	if *jsonOut {
		jsonPrint(stdout, withCallLog(m, resolved))
		return 0
	}
	fmt.Fprintf(stdout, "decision port — provider %s (%s)\n", m.Config.Provider, m.Config.Source)
	for _, s := range m.Sites {
		state := "off"
		if s.On {
			state = "on"
		}
		fmt.Fprintf(stdout, "  site %-20s %-3s rulings %s\n", s.Site, state, valuePlain(s.Rulings))
	}
	fmt.Fprintf(stdout, "log %s: %s rulings, %s applied, %s errors, median latency %s ms\n", m.LogPath, valuePlain(m.Rulings), valuePlain(m.Applied), valuePlain(m.Errors), valuePlain(m.MedianLatencyMS))
	for _, r := range m.Rows {
		fmt.Fprintf(stdout, "  %s  %-18s %-8s %-18s conf %.2f  %5d ms  applied=%v %s\n", r.TS, r.Site, r.Kind, r.Choice, r.Confidence, r.LatencyMS, r.Applied, r.Error)
	}
	if m.Calibration != nil {
		fmt.Fprintf(stdout, "calibration (%s):\n", m.Calibration.Reading)
		for _, b := range m.Calibration.Buckets {
			fmt.Fprintf(stdout, "  %-24s %d/%d = %.3f\n", b.Bucket, b.Hits, b.N, b.Fraction)
		}
	} else {
		fmt.Fprintln(stdout, "calibration: "+m.CalibrationNote)
	}
	return 0
}

// decideRunner runs `fathom decide` (registered below; EW-45).
type decideRunner struct{}

func (decideRunner) execVerb(args []string, _ io.Reader, stdout, stderr io.Writer) int {
	return cmdDecide(args, stdout, stderr)
}

func init() {
	declareVerb(verb{Name: "decide", Group: "govern", Order: 130, Run: decideRunner{},
		Summary: "The decision port (ADR-042): log [--site S] [--json] — the provider, its sites, the ruling log beside the call log and the latest calibration reading's table"})
}

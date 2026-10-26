package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

type EnvelopeResult struct {
	Version           string `json:"version"`
	Status            string `json:"status"`
	Reason            string `json:"reason"`
	Next              string `json:"next"`
	NoActionAuthority bool   `json:"noActionAuthority"`
	Data              any    `json:"data,omitempty"`
}

func main() { os.Exit(run(os.Args[1:], os.Stdout)) }

func run(args []string, w io.Writer) int {
	if len(args) == 0 {
		return emit(w, "text", EnvelopeResult{Version: "decision-lab.result/1", Status: "error", Reason: "usage", Next: "use prepare, check or study", NoActionAuthority: true}, 2)
	}
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	packPath := fs.String("pack", "", "trusted pack JSON")
	inputPath := fs.String("input", "", "input JSON")
	capturePath := fs.String("capture", "", "captured result JSON")
	manifestPath := fs.String("manifest", "", "expected manifest JSON")
	capturesPath := fs.String("captures", "", "capture array JSON")
	outPath := fs.String("out", "", "prepared envelope path; - writes state to stdout")
	format := fs.String("format", "text", "text or json")
	if err := fs.Parse(args[1:]); err != nil || fs.NArg() != 0 {
		return emit(w, "text", fail("usage", "check command flags"), 2)
	}
	if *format != "text" && *format != "json" {
		return emit(w, "text", fail("format", "use --format text or json"), 2)
	}
	if *packPath == "" {
		return emit(w, *format, fail("pack_required", "supply --pack PATH"), 2)
	}
	var p Pack
	packRaw, err := readJSON(*packPath, &p)
	if err != nil {
		return emit(w, *format, fail("invalid_pack", "inspect trusted pack JSON"), 2)
	}
	if err = p.validate(); err != nil {
		return emit(w, *format, fail("invalid_pack", "repair trusted pack contract"), 2)
	}
	switch args[0] {
	case "prepare":
		if *inputPath == "" {
			return emit(w, *format, fail("input_required", "supply --input PATH"), 2)
		}
		var in Input
		inputRaw, err := readJSON(*inputPath, &in)
		if err != nil {
			return emit(w, *format, fail("invalid_input", "repair input JSON"), 2)
		}
		pr, err := prepared(p, in)
		if err != nil {
			return emit(w, *format, fail("invalid_input", "repair input identity or state"), 2)
		}
		pr.PackFileDigest = packRaw
		pr.InputFileDigest = inputRaw
		if *outPath != "" && *outPath != "-" {
			f, err := os.OpenFile(*outPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
			if err != nil {
				return emit(w, *format, fail("output_failure", "choose a new writable --out PATH"), 2)
			}
			enc := json.NewEncoder(f)
			enc.SetIndent("", "  ")
			err = enc.Encode(pr)
			closeErr := f.Close()
			if err != nil || closeErr != nil {
				os.Remove(*outPath)
				return emit(w, *format, fail("output_failure", "check output filesystem"), 2)
			}
		}
		data := map[string]any{"caseId": in.CaseID, "packFileDigest": packRaw, "inputFileDigest": inputRaw, "packDigest": pr.PackDigest, "inputDigest": pr.InputDigest, "requestKey": pr.RequestKey, "outputPath": *outPath}
		if *outPath == "-" {
			if err := json.NewEncoder(w).Encode(pr); err != nil {
				return 2
			}
			return 0
		}
		return emit(w, *format, EnvelopeResult{Version: "decision-lab.result/1", Status: "prepared", Reason: "ok", Next: "capture the decision port response, then run check", NoActionAuthority: true, Data: data}, 0)
	case "check":
		if *inputPath == "" || *capturePath == "" {
			return emit(w, *format, fail("input_required", "supply --input and --capture"), 2)
		}
		var in Input
		inputRaw, err := readJSON(*inputPath, &in)
		if err != nil {
			return emit(w, *format, fail("invalid_input", "repair input JSON"), 2)
		}
		var c Capture
		captureRaw, err := readJSON(*capturePath, &c)
		if err != nil {
			return emit(w, *format, fail("invalid_capture", "repair capture JSON"), 2)
		}
		cr, err := check(p, in, c)
		if err != nil {
			return emit(w, *format, fail("invalid_capture", "compare capture bindings and port outcomes"), 2)
		}
		data := map[string]any{"report": cr, "packFileDigest": packRaw, "inputFileDigest": inputRaw, "captureFileDigest": captureRaw}
		code := 2
		if cr.Status == "usable" {
			code = 0
		}
		return emit(w, *format, EnvelopeResult{Version: "decision-lab.result/1", Status: cr.Status, Reason: cr.Reason, Next: "review diagnostic outcomes; no effect is authorized", NoActionAuthority: true, Data: data}, code)
	case "study":
		if *manifestPath == "" || *capturesPath == "" {
			return emit(w, *format, fail("study_files_required", "supply --manifest and --captures"), 2)
		}
		var m Manifest
		manifestRaw, err := readJSON(*manifestPath, &m)
		if err != nil {
			return emit(w, *format, fail("invalid_manifest", "repair independently authored manifest JSON"), 2)
		}
		var captures []Capture
		capturesRaw, err := readJSON(*capturesPath, &captures)
		if err != nil {
			return emit(w, *format, fail("invalid_captures", "repair capture array JSON"), 2)
		}
		report, err := study(p, m, captures)
		report.RawFileDigests = map[string]string{"pack": packRaw, "manifest": manifestRaw, "captures": capturesRaw}
		if err != nil {
			next := "repair manifest, captures and exact case coverage"
			if report.Reason == "duplicate_exposure" {
				next = "remove repeated request exposure; inspect case inputs independently"
			}
			if report.Reason == "missing_cases" {
				next = "supply one capture for every listed expected case"
			}
			return emit(w, *format, EnvelopeResult{Version: "decision-lab.result/1", Status: "incomplete", Reason: report.Reason, Next: next, NoActionAuthority: true, Data: report}, 2)
		}
		return emit(w, *format, EnvelopeResult{Version: "decision-lab.result/1", Status: "complete", Reason: "ok", Next: "review per-class diagnostics against baseline and kill criterion", NoActionAuthority: true, Data: report}, 0)
	default:
		return emit(w, *format, fail("usage", "use prepare, check or study"), 2)
	}
}

func fail(reason, next string) EnvelopeResult {
	return EnvelopeResult{Version: "decision-lab.result/1", Status: "error", Reason: reason, Next: next, NoActionAuthority: true}
}

func emit(w io.Writer, format string, r EnvelopeResult, code int) int {
	var err error
	if format == "json" {
		err = json.NewEncoder(w).Encode(r)
	} else {
		_, err = fmt.Fprintf(w, "%s: %s (%s). %s. no action authority\n", r.Status, r.Reason, r.Version, r.Next)
		if err == nil {
			switch d := r.Data.(type) {
			case StudyReport:
				_, err = fmt.Fprintf(w, "cases %d/%d; answers %d/%d; unknown %d; declined %d; invalid %d; failed %d; answer cache hits %d; declared attempts %d; failed attempts %d; usage unreported captures %d; false favorable %d; false unfavorable %d; abstained expected favorable %d\n", d.CapturedCases, d.ExpectedCases, d.Answered, d.ExpectedAnswers, d.Unknown, d.Declined, d.Invalid, d.Failed, d.AnswerCacheHits, d.DeclaredAttempts, d.FailedAttempts, d.UsageUnreportedCaptures, d.FalseFavorable, d.FalseUnfavorable, d.AbstainedExpectedFavorable)
				if err == nil {
					for _, cause := range []string{"no-provider", "timeout", "not-authorized", "budget-exhausted", "transport-error"} {
						if _, err = fmt.Fprintf(w, "%s: %d\n", cause, d.CauseCounts[cause]); err != nil {
							break
						}
					}
				}
				if err == nil && len(d.MissingCaseIDs) > 0 {
					_, err = fmt.Fprintf(w, "missing cases: %s\n", strings.Join(d.MissingCaseIDs, ", "))
				}
				if err == nil && len(d.DuplicateExposureCaseIDs) > 0 {
					_, err = fmt.Fprintf(w, "duplicate exposure cases: %s\n", strings.Join(d.DuplicateExposureCaseIDs, ", "))
				}
			case map[string]any:
				if c, ok := d["report"].(CheckResult); ok {
					for _, id := range sortedKeys(c.Outcomes) {
						if answer, ok := c.ObservedAnswers[id]; ok {
							_, err = fmt.Fprintf(w, "%s: %s; choice %s; confidence %s; answer cache hit %t\n", id, c.Outcomes[id], answer.Choice, strconv.FormatFloat(answer.Confidence, 'g', 6, 64), answer.AnswerCacheHit)
						} else {
							_, err = fmt.Fprintf(w, "%s: %s\n", id, c.Outcomes[id])
						}
						if err != nil {
							break
						}
					}
				} else if key, ok := d["requestKey"].(string); ok {
					_, err = fmt.Fprintf(w, "requestKey: %s\n", key)
				}
			}
		}
	}
	if err != nil {
		return 2
	}
	return code
}

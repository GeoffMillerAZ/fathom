package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/geoffmilleraz/fathom/internal/console"
	"github.com/geoffmilleraz/fathom/internal/verify"
)

// cmdPlansWeightAccept implements plans weight accept <row-id> [--plan file].
// It reads optional local index evidence, never a daemon or provider.
func cmdPlansWeightAccept(args []string, stdout, stderr io.Writer) int {
	fs := newFlagSet("plans weight accept", stderr)
	planFile := fs.String("plan", "", "plan containing the row (default: search active plans in this repository)")
	storePath := fs.String("store", "", "existing local index store for the estimate (optional)")
	service := fs.String("service", "", "service ID in the local index (required with --store)")
	dryRun := fs.Bool("dry-run", false, "print the authored and estimated weights without writing")
	if len(args) > 0 && args[0] != "accept" && args[0] != "-h" && args[0] != "--help" && args[0] != "-help" && args[0] != "help" {
		fmt.Fprintf(stderr, "error: unknown plans weight subcommand %q (want accept)\n", args[0])
		return 1
	}

	if len(args) < 2 {
		if err := fs.Parse(args); err != nil {
			return 1
		}
		fmt.Fprintln(stderr, "Usage: fathom plans weight accept <row-id> [--plan docs/plans/x.md] [--store DB --service S] [--dry-run]")
		return 1
	}
	if len(args) == 2 && args[0] == "accept" {
		switch args[1] {
		case "-h", "--help", "-help", "help":
			if err := fs.Parse(args[1:]); err != nil {
				return 0
			}
		}
	}
	if args[0] != "accept" || strings.HasPrefix(args[1], "-") {
		fmt.Fprintln(stderr, "Usage: fathom plans weight accept <row-id> [--plan docs/plans/x.md] [--store DB --service S] [--dry-run]")
		return 1
	}
	id := args[1]
	if err := fs.Parse(args[2:]); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 1
	}
	if *storePath != "" && *service == "" {
		fmt.Fprintln(stderr, "error: --service is required with --store")
		return 1
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "error: unexpected argument after row id")
		return 1
	}
	files := []string{*planFile}
	if *planFile == "" {
		var err error
		files, err = activePlanFiles(verifyRepoRoot())
		if err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
	}
	var file string
	var raw []byte
	var task verify.Task
	found := false
	for _, path := range files {
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(stderr, "error: reading plan %q: %v\n", path, err)
			return 1
		}
		for _, row := range verify.ParseMarkdownPlan("", data).Tasks {
			if row.ID != id {
				continue
			}
			if found {
				fmt.Fprintf(stderr, "error: row %q is ambiguous; use --plan with a unique row id\n", id)
				return 1
			}
			file, raw, task, found = path, data, row, true
		}
	}
	if !found {
		fmt.Fprintf(stderr, "error: row %q not found in the selected plans\n", id)
		return 1
	}

	// Preserve every byte outside the chosen row's annotation, including CRLF,
	// wrapped contracts, trailing spaces and a missing final newline.
	lines := strings.SplitAfter(string(raw), "\n")
	text := strings.Join(lines[task.Line-1:task.EndLine], "")
	ann, _, suspect := verify.ParseAnnotation(text)
	if suspect != "" || ann["id"] != id {
		fmt.Fprintf(stderr, "error: row %q requires an unambiguous authored id annotation\n", id)
		return 1
	}
	if task.WeightInvalid != "" {
		fmt.Fprintf(stderr, "error: row %q has malformed authored weight %s\n", id, task.WeightInvalid)
		return 1
	}
	authoredTaskWeight(&task, raw)
	in, err := rowWeightPreflightInput(context.Background(), task, file, *service, *storePath, time.Now().UTC())
	if *storePath != "" && err != nil {
		fmt.Fprintf(stderr, "error: reading estimate store: %v\n", err)
		return 1
	}
	in.ExcludeWeight = true
	weight := estimatePlanTask(task, console.DifficultyProfile(in))
	replacement, ok := upsertWeightPreservingSpace(text, strconv.Itoa(weight.EstimatedWeight))
	if !ok {
		fmt.Fprintf(stderr, "error: row %q has an unsafe or duplicate weight annotation\n", id)
		return 1
	}
	updated := []byte(strings.Join(lines[:task.Line-1], "") + replacement + strings.Join(lines[task.EndLine:], ""))
	if !*dryRun && !bytes.Equal(raw, updated) {
		if err := os.WriteFile(file, updated, 0); err != nil {
			fmt.Fprintf(stderr, "error: writing plan %q: %v\n", file, err)
			return 1
		}
	}
	before := "absent"
	if _, ok := ann["weight"]; ok {
		before = strconv.Itoa(task.Weight)
	}
	fmt.Fprintf(stdout, "%s (%s): weight: %s -> %d; %s\n", id, file, before, weight.EstimatedWeight, weight.Label)
	return 0
}

// estimatePlanTask is shared by preview, acceptance, context and the row card.
func estimatePlanTask(task verify.Task, profile console.Profile) console.RowWeightEstimate {
	fact := planTaskFact(task, nil)
	files := append([]string{task.OraclePath}, task.GonePaths...)
	return console.EstimateRowWeight(console.RowWeightInput{ContractKind: console.ContractOf(fact).Kind, Files: files, Needs: len(task.Needs), Profile: profile, AuthoredWeight: task.Weight})
}

// upsertWeightPreservingSpace leaves the grammar's safety/duplicate check in
// charge. A marker identifies the exact pair it selected, even beside quoted
// commas/braces or earlier code literals. Only that pair's surrounding space
// is restored; every unrelated byte comes from the original text.
func upsertWeightPreservingSpace(text, value string) (string, bool) {
	const marker = "__weight_replacement__"
	marked, ok := verify.UpsertAnnotationField(text, "weight", marker)
	if !ok {
		return text, false
	}
	ann, _, _ := verify.ParseAnnotation(text)
	if _, exists := ann["weight"]; !exists {
		return verify.UpsertAnnotationField(text, "weight", value)
	}
	pos := strings.Index(marked, "weight: "+marker)
	if pos < 0 {
		return text, false
	}
	// Upsert preserves all pairs before and after the selected pair. Recover
	// its original span from those identical prefix/suffix bytes.
	prefix := strings.TrimRightFunc(marked[:pos], unicode.IsSpace)
	suffix := marked[pos+len("weight: "+marker):]
	if !strings.HasPrefix(text, prefix) || !strings.HasSuffix(text, suffix) {
		return text, false
	}
	pair := text[len(prefix) : len(text)-len(suffix)]
	left := pair[:len(pair)-len(strings.TrimLeftFunc(pair, unicode.IsSpace))]
	right := pair[len(strings.TrimRightFunc(pair, unicode.IsSpace)):]
	return prefix + left + "weight: " + value + right + suffix, true
}

// authoredTaskWeight distinguishes the grammar's default 1 from an authored 1.
func authoredTaskWeight(task *verify.Task, raw []byte) {
	lines := strings.SplitAfter(string(raw), "\n")
	if task.Line < 1 || task.EndLine < task.Line || task.EndLine > len(lines) {
		return
	}
	ann, _, _ := verify.ParseAnnotation(strings.Join(lines[task.Line-1:task.EndLine], ""))
	if _, ok := ann["weight"]; !ok {
		task.Weight = 0
	}
}

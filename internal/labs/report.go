package labs

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// LabResultRow is the part of one ledger row the adoption report shows.
// It decodes its own keys rather than labResultKey, so the writer's
// duplicate key can change without moving the report.
type LabResultRow struct {
	RunID   string `json:"run_id"`
	TaskID  string `json:"task_id"`
	Arm     string `json:"arm"`
	Lab     string `json:"lab"`
	Verdict string `json:"verdict"`
}

// NewestLabResults reads the append-only ledger at path and keeps the last
// row in file order for each lab, sorted by lab. A missing file is an empty
// ledger. A line that does not decode or names no lab is skipped and named
// with its one-based line number; the readable rows are still returned.
func NewestLabResults(path string) ([]LabResultRow, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return []LabResultRow{}, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	newest := map[string]LabResultRow{}
	var problems []error
	name := filepath.ToSlash(path)
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for n := 1; sc.Scan(); n++ {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var raw struct {
			LabResultRow
			Evaluation struct {
				Verdict string `json:"verdict"`
			} `json:"evaluation"`
		}
		if err := json.Unmarshal(line, &raw); err != nil {
			problems = append(problems, fmt.Errorf("%s:%d: %v", name, n, err))
			continue
		}
		if raw.Lab == "" {
			problems = append(problems, fmt.Errorf("%s:%d: no lab", name, n))
			continue
		}
		row := raw.LabResultRow
		row.Verdict = raw.Evaluation.Verdict
		newest[row.Lab] = row
	}
	if err := sc.Err(); err != nil {
		problems = append(problems, fmt.Errorf("%s: %w", name, err))
	}
	out := make([]LabResultRow, 0, len(newest))
	for _, row := range newest {
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Lab < out[j].Lab })
	return out, errors.Join(problems...)
}

// LabCaseCount is one lab's committed cases by split.
type LabCaseCount struct {
	Lab     string `json:"lab"`
	Train   int    `json:"train"`
	Select  int    `json:"select"`
	Confirm int    `json:"confirm"`
}

// CountLabCases counts dir/<lab>/cases/*.json by split, reading only the
// split key. It is the report's stand-in until LAB-18's case loader
// validates cases against #Case. A missing dir or cases directory is empty.
// A case that does not decode, or names no known split, is skipped and
// named relative to dir with a one-based line number. Sorted by lab; labs
// with no counted case are left out.
func CountLabCases(dir string) ([]LabCaseCount, error) {
	labDirs, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return []LabCaseCount{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := []LabCaseCount{}
	var problems []error
	for _, labDir := range labDirs {
		if !labDir.IsDir() {
			continue
		}
		casesRel := filepath.Join(labDir.Name(), "cases")
		files, err := os.ReadDir(filepath.Join(dir, casesRel))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			problems = append(problems, fmt.Errorf("%s: %w", filepath.ToSlash(casesRel), err))
			continue
		}
		count := LabCaseCount{Lab: labDir.Name()}
		for _, file := range files {
			if file.IsDir() || !strings.HasSuffix(file.Name(), ".json") {
				continue
			}
			rel := filepath.ToSlash(filepath.Join(casesRel, file.Name()))
			data, err := os.ReadFile(filepath.Join(dir, casesRel, file.Name()))
			if err != nil {
				problems = append(problems, fmt.Errorf("%s: %w", rel, err))
				continue
			}
			var c struct {
				Split string `json:"split"`
			}
			if err := json.Unmarshal(data, &c); err != nil {
				line := 1
				var syntax *json.SyntaxError
				if errors.As(err, &syntax) {
					line += bytes.Count(data[:min(int(syntax.Offset), len(data))], []byte("\n"))
				}
				problems = append(problems, fmt.Errorf("%s:%d: %v", rel, line, err))
				continue
			}
			line := 1
			if at := bytes.Index(data, []byte(`"split"`)); at >= 0 {
				line += bytes.Count(data[:at], []byte("\n"))
			}
			switch c.Split {
			case "train":
				count.Train++
			case "select":
				count.Select++
			case "confirm":
				count.Confirm++
			case "":
				problems = append(problems, fmt.Errorf("%s:%d: no split", rel, line))
			default:
				problems = append(problems, fmt.Errorf("%s:%d: unknown split %q", rel, line, c.Split))
			}
		}
		if count.Train+count.Select+count.Confirm > 0 {
			out = append(out, count)
		}
	}
	return out, errors.Join(problems...)
}

// FixProofNotMeasured is every fixed observation's fix-proof stage. No
// fix-proof format exists, so nothing reads a proof file or invents its key;
// LAB-24 replaces this with a reader of docs/bench/readings/labs/fix-proofs.jsonl.
const FixProofNotMeasured = "not measured (LAB-24 unbuilt)"

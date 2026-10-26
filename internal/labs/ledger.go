package labs

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/errors"
)

type labResultKey struct {
	RunID  string `json:"run_id"`
	TaskID string `json:"task_id"`
	Arm    string `json:"arm"`
}

// AppendLabResult validates row against #Result and appends one JSONL row to
// path (normally docs/bench/labs-ledger.jsonl), creating parent directories.
// The oracle's keys precede the sorted remaining fields. The row's stages
// retain the funnel, and evaluation.criterion_id names the single criterion
// the run was preregistered to decide. Verdicts are pass, fail or indeterminate.
// An exclusive lock on the ledger serializes duplicate checks and appends
// across writers. Past rows require valid JSON and their nonempty key fields,
// so later #Result changes cannot invalidate historical non-key payloads.
// Future callers fathom labs fire --write and labbench do not exist yet.
func AppendLabResult(path string, row any) error {
	data, err := json.Marshal(row)
	if err != nil {
		return fmt.Errorf("encode lab result: %w", err)
	}
	def := labDefinition("#Result")
	if err := def.Unify(def.Context().CompileBytes(data)).Validate(cue.Concrete(true)); err != nil {
		return fmt.Errorf("lab result: %s", errors.Details(err, nil))
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	var key labResultKey
	if err := json.Unmarshal(data, &key); err != nil {
		return err
	}
	line, err := orderedLabResult(fields)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("lock labs ledger: %w", err)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	reader := bufio.NewReader(f)
	needsNewline := false
	for n := 1; ; n++ {
		b, err := reader.ReadBytes('\n')
		if err != nil && err != io.EOF {
			return fmt.Errorf("read labs ledger: %w", err)
		}
		if len(b) > 0 {
			needsNewline = b[len(b)-1] != '\n'
		}
		if len(bytes.TrimSpace(b)) > 0 {
			var existing labResultKey
			if e := json.Unmarshal(b, &existing); e != nil {
				return fmt.Errorf("labs ledger line %d: %w", n, e)
			}
			if strings.TrimSpace(existing.RunID) == "" || strings.TrimSpace(existing.TaskID) == "" || strings.TrimSpace(existing.Arm) == "" {
				return fmt.Errorf("labs ledger line %d: missing run_id, task_id or arm", n)
			}
			if existing == key {
				return fmt.Errorf("duplicate lab result (%s, %s, %s)", key.RunID, key.TaskID, key.Arm)
			}
		}
		if err == io.EOF {
			break
		}
	}
	if needsNewline {
		line = append([]byte{'\n'}, line...)
	}
	line = append(line, '\n')
	if n, err := f.Write(line); err != nil {
		return err
	} else if n != len(line) {
		return io.ErrShortWrite
	}
	return f.Sync()
}

func orderedLabResult(fields map[string]json.RawMessage) ([]byte, error) {
	keys := []string{"run_id", "task_id", "arm", "trials", "evaluation", "surface_unused", "instrument_defect"}
	rest := make([]string, 0, len(fields))
	first := surfaceSet(keys)
	for key := range fields {
		if !first[key] {
			rest = append(rest, key)
		}
	}
	sort.Strings(rest)
	keys = append(keys, rest...)
	var out bytes.Buffer
	out.WriteByte('{')
	for _, key := range keys {
		value, ok := fields[key]
		if !ok {
			continue
		}
		if out.Len() > 1 {
			out.WriteByte(',')
		}
		name, _ := json.Marshal(key)
		out.Write(name)
		out.WriteByte(':')
		var compact bytes.Buffer
		if err := json.Compact(&compact, value); err != nil {
			return nil, err
		}
		out.Write(compact.Bytes())
	}
	out.WriteByte('}')
	return out.Bytes(), nil
}

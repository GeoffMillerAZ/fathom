package benchledger

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

const Schema = "fathom.bench-cell/v1"

type CellRow struct {
	Schema    string        `json:"schema"`
	Key       string        `json:"key"`
	Inputs    CellInputs    `json:"inputs"`
	Outcome   ScoredCell    `json:"outcome"`
	Preflight PairPreflight `json:"preflight"`
}

func validateRow(row CellRow, seen map[string]bool) error {
	if row.Schema != Schema {
		return refuse("unknown cell schema %q", row.Schema)
	}
	key, err := CellKey(row.Inputs)
	if err != nil {
		return err
	}
	if row.Key != key {
		return refuse("stored key does not match inputs")
	}
	if seen[key] {
		return refuse("duplicate cell key %s", key)
	}
	if err := validateScore(row.Outcome); err != nil {
		return err
	}
	state, err := pairState(row.Preflight)
	if err != nil {
		return err
	}
	// The persisted shape always carries an explicit typed disposition, even when
	// ProjectCell accepted a missing pair as an unknown transient observation.
	if row.Preflight.Disposition == "" {
		return refuse("missing preflight disposition")
	}
	if row.Outcome.State != state {
		return refuse("outcome state differs from pair disposition")
	}
	if row.Outcome.TaskID != row.Inputs.Base.TaskID || row.Outcome.AssignedArm != row.Inputs.Base.Arm || row.Outcome.Trial != row.Inputs.Trial {
		return refuse("outcome identity differs from cell inputs")
	}
	seen[key] = true
	return nil
}

// AppendCell validates the complete supplied history before writing one row.
// The caller owns serialization/locking and supplies all existing rows.
func AppendCell(dst io.Writer, existing []CellRow, row CellRow) error {
	seen := make(map[string]bool, len(existing)+1)
	for i, r := range existing {
		if err := validateRow(r, seen); err != nil {
			return fmt.Errorf("existing line %d: %w", i+1, err)
		}
	}
	if err := validateRow(row, seen); err != nil {
		return err
	}
	raw, err := json.Marshal(row)
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	n, err := dst.Write(raw)
	if err != nil {
		return err
	}
	if n != len(raw) {
		return io.ErrShortWrite
	}
	return nil
}

// ReadCells reads JSONL without hidden filesystem state. Invalid rows include
// their physical line number, and a failure never returns a partial ledger.
func ReadCells(src io.Reader) ([]CellRow, error) {
	rows := make([]CellRow, 0)
	seen := make(map[string]bool)
	reader := bufio.NewReader(src)
	for line := 1; ; line++ {
		raw, err := reader.ReadBytes('\n')
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		if len(raw) == 0 && err == io.EOF {
			return rows, nil
		}
		var row CellRow
		if bytes.ContainsRune(raw, '\r') {
			return nil, refuse("line %d: CR is not canonical LF JSON", line)
		}
		if e := validateWire(raw); e != nil {
			return nil, refuse("line %d: %v", line, e)
		}
		if e := decodeStrict(raw, &row); e != nil {
			return nil, refuse("line %d: invalid cell JSON: %v", line, e)
		}
		if e := validateRow(row, seen); e != nil {
			return nil, fmt.Errorf("line %d: %w", line, e)
		}
		rows = append(rows, row)
		if err == io.EOF {
			return rows, nil
		}
	}
}

// Require every v1 observation explicitly, including false/zero values. Decode
// object keys ourselves because encoding/json otherwise accepts duplicate keys
// and case-insensitive matches, which can hide an incomplete or ambiguous row.
func objectFields(raw []byte, names string) (map[string]json.RawMessage, error) {
	d := json.NewDecoder(bytes.NewReader(raw))
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		return nil, refuse("expected JSON object")
	}
	fields := make(map[string]json.RawMessage)
	allowed := make(map[string]bool)
	for _, name := range strings.Fields(names) {
		allowed[name] = true
	}
	for d.More() {
		token, err := d.Token()
		if err != nil {
			return nil, err
		}
		name, ok := token.(string)
		if !ok || !allowed[name] {
			return nil, refuse("unknown field %q", name)
		}
		if _, ok := fields[name]; ok {
			return nil, refuse("duplicate field %q", name)
		}
		var value json.RawMessage
		if err := d.Decode(&value); err != nil {
			return nil, err
		}
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return nil, refuse("null field %q", name)
		}
		fields[name] = value
	}
	if _, err := d.Token(); err != nil {
		return nil, err
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return nil, refuse("trailing JSON data")
	}
	if len(fields) != len(allowed) {
		return nil, refuse("incomplete v1 object")
	}
	return fields, nil
}
func validateWire(raw []byte) error {
	row, err := objectFields(raw, "schema key inputs outcome preflight")
	if err != nil {
		return err
	}
	inputs, err := objectFields(row["inputs"], "base subject_commit tool_schema_sha256 profile_sha256 family criterion_id trial")
	if err != nil {
		return err
	}
	if _, err = objectFields(inputs["base"], "corpus_sha256 task_id arm model_id harness_version scoring prompt_subject arm_surface"); err != nil {
		return err
	}
	if _, err = objectFields(row["outcome"], "task_id assigned_arm trial oracle_passed fail_to_pass_ok pass_to_pass_ok success surface_calls total_tokens state"); err != nil {
		return err
	}
	_, err = objectFields(row["preflight"], "pair_id treatment_session control_session before_agent valid disposition reason")
	return err
}

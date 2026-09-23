package testevidence

// envelopes.go — LoadEnvelopes (TC-2): reads one attested #TestResult
// envelope file, or every *.json file in a directory. Each file may hold
// a single JSON object, an NDJSON stream (one object per line — what
// `surveyor run --json` emits), or a JSON array; every top-level value is
// decoded independently. A value may be a bare #TestResult object or a
// DSSE envelope wrapping an in-toto Statement whose predicate IS the
// #TestResult (design doc §2, ADR-043 decision 5) — fathom decodes the
// payload and records its digest as the run's identity, but never
// attempts signature verification itself (that is test-kit's job; the
// DSSE envelope's own `tier` field is carried through as a fact).

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// dsseEnvelope is the DSSE wrapper (schema/testkit's mirrored #DSSE would
// be the CUE twin, but envelopes are attested JSON, never CUE-validated
// here — LoadManifests is the CUE-validated surface, not this one).
type dsseEnvelope struct {
	PayloadType string          `json:"payloadType"`
	Payload     string          `json:"payload"`
	Signatures  json.RawMessage `json:"signatures"`
	Tier        string          `json:"tier"`
}

// inTotoStatement is the #Statement a DSSE payload decodes to.
type inTotoStatement struct {
	Type          string          `json:"_type"`
	Subject       json.RawMessage `json:"subject"`
	PredicateType string          `json:"predicateType"`
	Predicate     json.RawMessage `json:"predicate"`
}

// envelopeWire is the JSON wire shape of a bare #TestResult — Run plus
// the timing block Run itself does not expose as a JSON field.
type envelopeWire struct {
	Suite     SuiteRef        `json:"suite"`
	Key       string          `json:"key"`
	Verdict   string          `json:"verdict"`
	Cases     []Case          `json:"cases"`
	Totals    Totals          `json:"totals"`
	Coverage  *CoverageReport `json:"coverage,omitempty"`
	Artifacts []ArtifactRef   `json:"artifacts,omitempty"`
	Timing    timingView      `json:"timing"`
}

func (w envelopeWire) toRun() Run {
	return Run{
		Suite: w.Suite, Key: w.Key, Verdict: w.Verdict, Cases: w.Cases,
		Totals: w.Totals, Coverage: w.Coverage, Artifacts: w.Artifacts,
		StartedAt: w.Timing.StartedAt,
	}
}

// LoadEnvelopes reads attested #TestResult envelopes from path: a single
// file, or every *.json file in a directory (sorted). Returns one Run per
// decoded envelope object, in file order.
func LoadEnvelopes(path string) ([]Run, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", path, err)
	}

	var files []string
	if info.IsDir() {
		entries, err := os.ReadDir(path)
		if err != nil {
			return nil, fmt.Errorf("read dir %s: %w", path, err)
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			files = append(files, filepath.Join(path, e.Name()))
		}
		sort.Strings(files)
	} else {
		files = []string{path}
	}

	var runs []Run
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", f, err)
		}
		fileRuns, err := parseEnvelopeFile(data)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", f, err)
		}
		for i := range fileRuns {
			fileRuns[i].SourceFile = f
		}
		runs = append(runs, fileRuns...)
	}
	return runs, nil
}

// parseEnvelopeFile decodes every top-level JSON value in data: a JSON
// array is unmarshalled as a list of values; otherwise a streaming
// decoder loop handles a single object OR an NDJSON stream identically
// (json.Decoder.Decode reads one value per call regardless of whether
// values are newline- or whitespace-separated).
func parseEnvelopeFile(data []byte) ([]Run, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, nil
	}

	var rawObjs []json.RawMessage
	if trimmed[0] == '[' {
		if err := json.Unmarshal(trimmed, &rawObjs); err != nil {
			return nil, fmt.Errorf("decode JSON array: %w", err)
		}
	} else {
		dec := json.NewDecoder(bytes.NewReader(trimmed))
		for {
			var raw json.RawMessage
			if err := dec.Decode(&raw); err != nil {
				if err == io.EOF {
					break
				}
				return nil, fmt.Errorf("decode JSON stream: %w", err)
			}
			rawObjs = append(rawObjs, raw)
		}
	}

	runs := make([]Run, 0, len(rawObjs))
	for i, raw := range rawObjs {
		run, err := decodeOneEnvelope(raw)
		if err != nil {
			return nil, fmt.Errorf("envelope %d: %w", i, err)
		}
		runs = append(runs, run)
	}
	return runs, nil
}

// decodeOneEnvelope decodes one top-level JSON value into a Run, unwrapping
// a DSSE envelope when the value carries both "payloadType" and "payload"
// keys (the two fields no bare #TestResult has).
func decodeOneEnvelope(raw json.RawMessage) (Run, error) {
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		return Run{}, fmt.Errorf("decode as JSON object: %w", err)
	}
	_, hasType := probe["payloadType"]
	_, hasPayload := probe["payload"]
	if hasType && hasPayload {
		return decodeDSSEEnvelope(raw)
	}

	var wire envelopeWire
	if err := json.Unmarshal(raw, &wire); err != nil {
		return Run{}, fmt.Errorf("decode #TestResult: %w", err)
	}
	run := wire.toRun()
	sum := sha256.Sum256(raw)
	run.Digest = "sha256:" + hex.EncodeToString(sum[:])
	return run, nil
}

func decodeDSSEEnvelope(raw json.RawMessage) (Run, error) {
	var dsse dsseEnvelope
	if err := json.Unmarshal(raw, &dsse); err != nil {
		return Run{}, fmt.Errorf("decode DSSE envelope: %w", err)
	}
	payload, err := base64.StdEncoding.DecodeString(dsse.Payload)
	if err != nil {
		return Run{}, fmt.Errorf("decode DSSE payload base64: %w", err)
	}
	var stmt inTotoStatement
	if err := json.Unmarshal(payload, &stmt); err != nil {
		return Run{}, fmt.Errorf("decode in-toto statement: %w", err)
	}
	if len(stmt.Predicate) == 0 {
		return Run{}, fmt.Errorf("in-toto statement carries no predicate")
	}
	var wire envelopeWire
	if err := json.Unmarshal(stmt.Predicate, &wire); err != nil {
		return Run{}, fmt.Errorf("decode #TestResult predicate: %w", err)
	}
	run := wire.toRun()
	sum := sha256.Sum256(payload)
	run.Digest = "sha256:" + hex.EncodeToString(sum[:])
	run.SignerTier = dsse.Tier
	return run, nil
}

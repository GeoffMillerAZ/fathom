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

// inTotoSubject is one entry of an in-toto Statement's `subject` array —
// just the two fields runCommitFromEnvelope (CV-1) needs to recognise a
// subject named "gitCommit" (design doc §1.1: "An in-toto subject named
// gitCommit is accepted as the same fact").
type inTotoSubject struct {
	Name   string            `json:"name"`
	Digest map[string]string `json:"digest"`
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
	Env       envInfoWire     `json:"env"`
}

// envInfoWire is the subset of #EnvInfo (schema/testkit/result.cue)
// runCommitFromEnvelope reads — CV-1's own commit/dirty/race facts
// (design doc §1.1, O114). The other #EnvInfo fields (toolchainDigest,
// runnerClass, arch, os) are not fathom's concern and are left for the
// decoder to discard.
type envInfoWire struct {
	Commit string `json:"commit,omitempty"`
	Dirty  bool   `json:"dirty,omitempty"`
	Race   bool   `json:"race,omitempty"`
}

// runCommitFromEnvelope (CV-1, design doc §1.1) reads the commit/dirty/
// race facts a run was attested under: from #EnvInfo's own env.commit/
// env.dirty/env.race fields when commit is present, or — for a
// DSSE-wrapped envelope whose in-toto Statement carries a subject named
// "gitCommit" — from that subject's digest, accepted as the same commit
// fact. subjects is nil for a bare (non-DSSE) envelope, which has no
// subjects to fall back to.
//
// Returns "" / false / false when neither source states a commit: an
// envelope from before this field existed, or from an adapter that was
// never told, leaves the run unbound — exactly what design doc §1.1's
// "unbound runs leave it unproven" expects, never a fabricated commit.
//
// The in-toto fallback carries only the commit identity, never dirty or
// race: a Statement's subject is a bare {name, digest} pair with no room
// for either fact, so a run bound this way reads dirty=false, race=false
// — a documented limitation (an actually-dirty tree attested only via a
// gitCommit subject is indistinguishable from a clean one here), not a
// silent bug.
func runCommitFromEnvelope(env envInfoWire, subjects []inTotoSubject) (commit string, dirty, race bool) {
	if env.Commit != "" {
		return env.Commit, env.Dirty, env.Race
	}
	for _, s := range subjects {
		if s.Name != "gitCommit" {
			continue
		}
		// git's own hash algorithm is sha1 (sha256 repositories are rare
		// and would still name their algorithm the same way); preferred
		// explicitly, with a sorted-key fallback so a differently-keyed
		// digest map never resolves by Go's randomized map iteration
		// order (internal/CLAUDE.md: map iteration order must never
		// reach a result).
		if sha1 := s.Digest["sha1"]; sha1 != "" {
			return sha1, false, false
		}
		keys := make([]string, 0, len(s.Digest))
		for k := range s.Digest {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if s.Digest[k] != "" {
				return s.Digest[k], false, false
			}
		}
	}
	return "", false, false
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
	run.Commit, run.Dirty, run.Race = runCommitFromEnvelope(wire.Env, nil)
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
	// subjects is decoded best-effort: an absent or malformed `subject`
	// array leaves it nil, which runCommitFromEnvelope treats exactly
	// like "no in-toto fallback available" rather than failing the whole
	// envelope over a field it does not otherwise need.
	var subjects []inTotoSubject
	_ = json.Unmarshal(stmt.Subject, &subjects)
	run := wire.toRun()
	run.Commit, run.Dirty, run.Race = runCommitFromEnvelope(wire.Env, subjects)
	sum := sha256.Sum256(payload)
	run.Digest = "sha256:" + hex.EncodeToString(sum[:])
	run.SignerTier = dsse.Tier
	return run, nil
}

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"time"

	fitness "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/quality/fitness-function-registry"
	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/internal/jsonwire"
)

type documentAdjudication struct {
	Version      string `json:"version"`
	PacketDigest string `json:"packet_digest"`
	Reviewer     struct {
		ID           string `json:"id"`
		Kind         string `json:"kind"`
		Relationship string `json:"relationship"`
	} `json:"reviewer"`
	Observations []fitness.Observation `json:"observations"`
}

// reviewPrivateBytes reads one existing private file without following a final symlink.
func reviewPrivateBytes(root *os.Root, name string, max int64) ([]byte, error) {
	st, err := root.Lstat(name)
	if err != nil || !st.Mode().IsRegular() || (st.Mode().Perm() != 0400 && st.Mode().Perm() != 0600) || st.Size() > max {
		return nil, errors.New("private file refused")
	}
	f, err := root.OpenFile(name, os.O_RDONLY|documentReviewNoFollow, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(st, opened) {
		return nil, errors.New("private file changed")
	}
	b, err := io.ReadAll(io.LimitReader(f, max+1))
	if err != nil || int64(len(b)) > max {
		return nil, errors.New("private file bound")
	}
	return b, nil
}

func reviewAt(raw string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil || t.IsZero() || t.Year() < 1 || t.Year() > 9999 || t.Location() != time.UTC || t.Format(time.RFC3339Nano) != raw || raw[len(raw)-1:] != "Z" {
		return time.Time{}, errors.New("noncanonical evaluation time")
	}
	return t, nil
}

func loadDocumentAdjudication(path, pin, packet string) (documentAdjudication, error) {
	var a documentAdjudication
	if !documentDigestPattern.MatchString(pin) || !documentDigestPattern.MatchString(packet) {
		return a, errors.New("invalid pin")
	}
	b, err := readDocumentAdjudicationBytes(path)
	if err != nil || hashBytes(b) != pin {
		return a, errors.New("adjudication pin mismatch")
	}
	if err = exactObject(b, "version", "packet_digest", "reviewer", "observations"); err != nil {
		return a, err
	}
	m, err := jsonwire.Object(b)
	if err != nil {
		return a, err
	}
	if err = exactObject(m["reviewer"], "id", "kind", "relationship"); err != nil {
		return a, err
	}
	var rows []json.RawMessage
	if err = json.Unmarshal(m["observations"], &rows); err != nil || rows == nil || len(rows) > 32 {
		return a, errors.New("invalid observations")
	}
	for _, row := range rows {
		if err = exactObject(row, "criterion_id", "definition_digest", "subject_digest", "outcome", "method", "reason", "artifact_digests", "observed_at", "valid_until"); err != nil {
			return a, err
		}
		fields, _ := jsonwire.Object(row)
		var digests []string
		if err = json.Unmarshal(fields["artifact_digests"], &digests); err != nil || digests == nil {
			return a, errors.New("invalid artifact digests")
		}
		if bytes.Equal(bytes.TrimSpace(fields["reason"]), []byte("null")) {
			return a, errors.New("null reason")
		}
	}
	if err = decodeDocumentReviewArtifact(b, &a); err != nil {
		return a, err
	}
	if a.Version != "prose-lab.document-adjudication/1" || a.PacketDigest != packet || !validLabID(a.Reviewer.ID) || (a.Reviewer.Kind != "human" && a.Reviewer.Kind != "agent") || (a.Reviewer.Relationship != "independent" && a.Reviewer.Relationship != "producer" && a.Reviewer.Relationship != "unknown") {
		return a, errors.New("adjudication header refused")
	}
	for _, o := range a.Observations {
		if err = o.Validate(); err != nil || (a.Reviewer.Kind == "agent" && o.Method == fitness.EvidenceMethodHuman) {
			return a, errors.New("observation refused")
		}
	}
	return a, nil
}

func readDocumentAdjudicationBytes(path string) ([]byte, error) {
	st, err := os.Lstat(path)
	if err != nil || !st.Mode().IsRegular() || (st.Mode().Perm() != 0400 && st.Mode().Perm() != 0600) || st.Size() > 1<<20 {
		return nil, errors.New("adjudication permissions")
	}
	f, err := os.OpenFile(path, os.O_RDONLY|documentReviewNoFollow, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(st, opened) {
		return nil, errors.New("adjudication changed")
	}
	b, err := io.ReadAll(io.LimitReader(f, (1<<20)+1))
	if err != nil || len(b) > 1<<20 {
		return nil, errors.New("adjudication byte bound")
	}
	return b, nil
}

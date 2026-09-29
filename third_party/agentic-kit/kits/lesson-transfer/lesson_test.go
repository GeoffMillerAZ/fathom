package lessontransfer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var lessonEpoch = time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)

func digestFill(ch string) string { return strings.Repeat(ch, 64) }

const (
	originSnapshot = "9955737c1df3ea26e823325c89c67c700428f4638703a828506e310393093af5"
	originIncident = "4255c32f60fa12116995505ec0bdfaafaf700ca557175a3479a3507a95b1391e"
	originScenario = "405ea170f114a38bf3029db03c7470801690a50d4091617fce1700a1ec182f13"
	remedyDigest   = "66248ea85be18c455a6a4440b2eade13b0a9d8bc70878f074bb26b2e1c954c4e"
	regressionTest = "405ea170f114a38bf3029db03c7470801690a50d4091617fce1700a1ec182f13"
)

func fixtureBytes(t *testing.T) ([]byte, []byte, []byte) {
	t.Helper()
	reproducer := []byte(`{"schema":"lesson-reproducer/v1","version":1,"input":{"case":"unknown-family"},"expected":"unknown","invariant":"unknown evidence cannot pass"}`)
	payload := Payload{
		Version: Version, ID: "unknown-evidence", Revision: 1, Status: "reviewed", Classification: "public-synthetic",
		Origin:        Origin{Workload: "kits/agent-harness", SnapshotDigest: originSnapshot, IncidentDigest: originIncident, ScenarioDigest: originScenario},
		Applicability: Applicability{Workloads: []string{"resume_as_code"}, ConfigDigests: []string{}},
		Exclusions:    []string{},
		Reproducer:    ReproducerRef{Schema: "lesson-reproducer/v1", Version: 1, File: "reproducer.json", RawDigest: digest(reproducerDigestDomain, reproducer), Expected: "unknown", MaxBytes: maxReproducerBytes},
		Remedies:      []RemedyRef{{ID: "coverage-gate", Revision: 1, Digest: remedyDigest, TestDigest: regressionTest}},
		Supersedes:    []string{},
		Revocation:    Revocation{Status: "active", Recheck: true},
	}
	lesson, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	att, err := json.Marshal(Attestations{PayloadDigest: digest(payloadDigestDomain, lesson), ReproducerDigest: digest(reproducerDigestDomain, reproducer), Reviewer: "fixture-review", ReviewedAt: lessonEpoch.Format(time.RFC3339), Scope: "synthetic only"})
	if err != nil {
		t.Fatal(err)
	}
	return lesson, reproducer, att
}

func fixtureBundle(t *testing.T) Bundle {
	t.Helper()
	lesson, reproducer, att := fixtureBytes(t)
	b, err := ValidateBundle(lesson, reproducer, att)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func alteredBundle(t *testing.T, change func(*Payload), changeAtt func(*Attestations)) Bundle {
	t.Helper()
	lesson, reproducer, attestation := fixtureBytes(t)
	var payload Payload
	var att Attestations
	if err := json.Unmarshal(lesson, &payload); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(attestation, &att); err != nil {
		t.Fatal(err)
	}
	change(&payload)
	if changeAtt != nil {
		changeAtt(&att)
	}
	var err error
	lesson, err = json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	att.PayloadDigest, att.ReproducerDigest = digest(payloadDigestDomain, lesson), digest(reproducerDigestDomain, reproducer)
	attestation, err = json.Marshal(att)
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := ValidateBundle(lesson, reproducer, attestation)
	if err != nil {
		t.Fatal(err)
	}
	return bundle
}

func receiver() ReceiverSnapshot {
	return ReceiverSnapshot{Workload: "resume_as_code", Revision: digestFill("1"), ConfigDigest: digestFill("d")}
}

func activeCurrent(_ context.Context, want ReceiverSnapshot) (ReceiverSnapshot, error) {
	return want, nil
}
func activeRevocation(context.Context, Bundle) (RevocationStatus, error) {
	return RevocationStatus{Status: "active"}, nil
}
func accept(context.Context, Bundle, ReceiverSnapshot) error { return nil }
func unknown(context.Context, Bundle, ReceiverSnapshot) (ReproductionOutcome, error) {
	return ReproductionOutcome{Classification: "unknown", Evidence: digestFill("9"), Detail: "fixture"}, nil
}

func TestValidateBundleBindsRawBytesWithActualNUL(t *testing.T) {
	t.Parallel()
	lesson, reproducer, att := fixtureBytes(t)
	b, err := ValidateBundle(lesson, reproducer, att)
	if err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(append(append([]byte(payloadDigestDomain), 0), lesson...))
	if b.PayloadDigest != hex.EncodeToString(h[:]) || b.PayloadDigest == digest(payloadDigestDomain+`\x00`, lesson) {
		t.Fatal("payload digest did not use an actual NUL domain separator")
	}
	if b.ReproducerDigest != digest(reproducerDigestDomain, reproducer) {
		t.Fatal("raw reproducer digest")
	}
}

func TestPublicSyntheticFixtureIsRawBoundAndInert(t *testing.T) {
	t.Parallel()
	base := filepath.Join("testdata", "public-synthetic")
	lesson, err := os.ReadFile(filepath.Join(base, "lesson.json"))
	if err != nil {
		t.Fatal(err)
	}
	reproducer, err := os.ReadFile(filepath.Join(base, "reproducer.json"))
	if err != nil {
		t.Fatal(err)
	}
	attestations, err := os.ReadFile(filepath.Join(base, "attestations.json"))
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := ValidateBundle(lesson, reproducer, attestations)
	if err != nil || bundle.Reproducer.Expected != "unknown" || strings.Contains(string(reproducer), "command") {
		t.Fatalf("bundle=%+v err=%v", bundle, err)
	}
	marshaled, err := json.Marshal(bundle.Payload)
	if err != nil || bytes.Contains(marshaled, []byte(`:null`)) {
		t.Fatalf("validated payload must preserve required arrays: %s err=%v", marshaled, err)
	}
}

func TestValidateBundleRejectsMalformedBindingsAndUnsafeShape(t *testing.T) {
	t.Parallel()
	lesson, reproducer, att := fixtureBytes(t)
	for _, tc := range []struct {
		name               string
		lesson, repro, att []byte
	}{
		{"unknown", append(lesson[:len(lesson)-1], []byte(`,"unknown":true}`)...), reproducer, att},
		{"duplicate", []byte(`{"version":"lesson-transfer/v1","version":"lesson-transfer/v1"}`), reproducer, att},
		{"trailing", append(append([]byte(nil), lesson...), []byte(` {}`)...), reproducer, att},
		{"attestation mismatch", lesson, reproducer, []byte(`{"payload_digest":"` + digestFill("0") + `","reproducer_digest":"` + digestFill("0") + `","reviewer":"x","reviewed_at":"now","scope":"x"}`)},
		{"command reproducer", lesson, []byte(`{"schema":"lesson-reproducer/v1","version":1,"command":"bad","input":{},"expected":"unknown","invariant":"x"}`), att},
		{"closed synthetic input", lesson, []byte(`{"schema":"lesson-reproducer/v1","version":1,"input":{"case":"unknown-family","api_key":"secret"},"expected":"unknown","invariant":"x"}`), att},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ValidateBundle(tc.lesson, tc.repro, tc.att); !errors.Is(err, ErrInvalid) {
				t.Fatalf("%v", err)
			}
		})
	}
}

func TestCheckAdoptionRequiresIndependentReceiverBoundaries(t *testing.T) {
	t.Parallel()
	b := fixtureBundle(t)
	for _, tc := range []struct {
		name       string
		accept     ReviewAcceptance
		current    CurrentReceiver
		revocation CurrentRevocation
		run        Reproduce
		want       error
	}{
		{"forged review not authority", func(context.Context, Bundle, ReceiverSnapshot) error { return errors.New("not accepted") }, activeCurrent, activeRevocation, unknown, ErrUntrusted},
		{"receiver drift", accept, func(context.Context, ReceiverSnapshot) (ReceiverSnapshot, error) { return ReceiverSnapshot{}, nil }, activeRevocation, unknown, ErrInvalid},
		{"revoked", accept, activeCurrent, func(context.Context, Bundle) (RevocationStatus, error) {
			return RevocationStatus{Status: "revoked"}, nil
		}, unknown, ErrRevoked},
		{"unexpected pass", accept, activeCurrent, activeRevocation, func(context.Context, Bundle, ReceiverSnapshot) (ReproductionOutcome, error) {
			return ReproductionOutcome{Classification: "pass", Evidence: digestFill("9"), Detail: "seeded false pass with otherwise valid evidence"}, nil
		}, ErrInvalid},
		{"missing evidence", accept, activeCurrent, activeRevocation, func(context.Context, Bundle, ReceiverSnapshot) (ReproductionOutcome, error) {
			return ReproductionOutcome{Classification: "unknown"}, nil
		}, ErrInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := CheckAdoptionAt(context.Background(), b, receiver(), tc.accept, tc.current, tc.revocation, tc.run, func() time.Time { return lessonEpoch })
			if !errors.Is(err, tc.want) {
				t.Fatalf("%v", err)
			}
		})
	}
}

func TestValidateReceiverSnapshotRequiresCanonicalDigests(t *testing.T) {
	t.Parallel()
	for _, snapshot := range []ReceiverSnapshot{
		{},
		{Workload: "resume_as_code", Revision: "not-a-digest", ConfigDigest: digestFill("d")},
		{Workload: "resume_as_code", Revision: strings.ToUpper(digestFill("a")), ConfigDigest: digestFill("d")},
	} {
		if err := ValidateReceiverSnapshot(snapshot); !errors.Is(err, ErrInvalid) {
			t.Fatalf("snapshot=%+v err=%v", snapshot, err)
		}
	}
}

func TestLifecycleAndCanonicalBindingsParseBeforeAdoption(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		status string
		revoke string
		want   error
	}{
		{"candidate", "candidate", "active", ErrUntrusted},
		{"superseded", "superseded", "active", ErrRevoked},
		{"revoked", "reviewed", "revoked", ErrRevoked},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bundle := alteredBundle(t, func(payload *Payload) { payload.Status, payload.Revocation.Status = tc.status, tc.revoke }, nil)
			_, err := CheckAdoptionAt(context.Background(), bundle, receiver(), accept, activeCurrent, activeRevocation, unknown, func() time.Time { return lessonEpoch })
			if !errors.Is(err, tc.want) {
				t.Fatalf("err=%v want=%v", err, tc.want)
			}
		})
	}
	for _, tc := range []func(*Payload){
		func(payload *Payload) { payload.Applicability.ConfigDigests = []string{"not-a-digest"} },
		func(payload *Payload) { payload.Remedies = append(payload.Remedies, payload.Remedies[0]) },
		func(payload *Payload) { payload.Exclusions = nil },
	} {
		lesson, reproducer, att := fixtureBytes(t)
		var payload Payload
		if err := json.Unmarshal(lesson, &payload); err != nil {
			t.Fatal(err)
		}
		tc(&payload)
		lesson, _ = json.Marshal(payload)
		var attestation Attestations
		_ = json.Unmarshal(att, &attestation)
		attestation.PayloadDigest, attestation.ReproducerDigest = digest(payloadDigestDomain, lesson), digest(reproducerDigestDomain, reproducer)
		att, _ = json.Marshal(attestation)
		if _, err := ValidateBundle(lesson, reproducer, att); !errors.Is(err, ErrInvalid) {
			t.Fatalf("invalid canonical payload accepted: %v", err)
		}
	}
	lesson, reproducer, att := fixtureBytes(t)
	var malformedAtt Attestations
	if err := json.Unmarshal(att, &malformedAtt); err != nil {
		t.Fatal(err)
	}
	malformedAtt.ReviewedAt = "not-rfc3339"
	att, _ = json.Marshal(malformedAtt)
	if _, err := ValidateBundle(lesson, reproducer, att); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid reviewed_at accepted: %v", err)
	}
}

func TestOutcomeAndClockBoundsFailClosed(t *testing.T) {
	t.Parallel()
	if _, err := ParseReproductionOutcome([]byte(`{"classification":"unknown","evidence":"opaque","detail":"x"}`)); !errors.Is(err, ErrInvalid) {
		t.Fatalf("opaque evidence accepted: %v", err)
	}
	if _, err := CheckAdoptionAt(context.Background(), fixtureBundle(t), receiver(), accept, activeCurrent, activeRevocation, unknown, func() time.Time { return time.Time{} }); !errors.Is(err, ErrInvalid) {
		t.Fatalf("zero clock accepted: %v", err)
	}
}

func TestGoMarshaledReceiptHasConcreteCUEShape(t *testing.T) {
	t.Parallel()
	receipt, err := CheckAdoptionAt(context.Background(), fixtureBundle(t), receiver(), accept, activeCurrent, activeRevocation, unknown, func() time.Time { return lessonEpoch })
	if err != nil {
		t.Fatal(err)
	}
	marshaled, err := json.Marshal(receipt)
	if err != nil || bytes.Contains(marshaled, []byte(`:null`)) || !bytes.Contains(marshaled, []byte(`"checked_at"`)) {
		t.Fatalf("receipt is not a concrete CUE-interchange shape: %s err=%v", marshaled, err)
	}
}

func TestCheckAdoptionCopiesCallbacksAndRechecksFreshness(t *testing.T) {
	t.Parallel()
	b := fixtureBundle(t)
	checks := 0
	receipt, err := CheckAdoptionAt(context.Background(), b, receiver(), func(_ context.Context, got Bundle, _ ReceiverSnapshot) error {
		got.Payload.ID = "mutated"
		got.Payload.Applicability.Workloads[0] = "other"
		return nil
	}, func(_ context.Context, got ReceiverSnapshot) (ReceiverSnapshot, error) {
		checks++
		if checks == 2 {
			return got, nil
		}
		return got, nil
	}, func(_ context.Context, got Bundle) (RevocationStatus, error) {
		got.Payload.Revocation.Status = "revoked"
		return RevocationStatus{Status: "active"}, nil
	}, func(_ context.Context, got Bundle, _ ReceiverSnapshot) (ReproductionOutcome, error) {
		got.Payload.Reproducer.Expected = "fail"
		return ReproductionOutcome{Classification: "unknown", Evidence: digestFill("9"), Detail: "callback mutation test"}, nil
	}, func() time.Time { return lessonEpoch })
	if err != nil || !receipt.Fresh || checks != 2 || b.Payload.ID != "unknown-evidence" || b.Payload.Reproducer.Expected != "unknown" {
		t.Fatalf("receipt=%+v err=%v bundle=%+v", receipt, err, b)
	}
	checks = 0
	_, err = CheckAdoptionAt(context.Background(), b, receiver(), accept, activeCurrent, func(context.Context, Bundle) (RevocationStatus, error) {
		checks++
		if checks == 2 {
			return RevocationStatus{Status: "revoked"}, nil
		}
		return RevocationStatus{Status: "active"}, nil
	}, unknown, func() time.Time { return lessonEpoch })
	if !errors.Is(err, ErrRevoked) {
		t.Fatalf("final revocation passed: %v", err)
	}
}

func TestCheckAdoptionRejectsCallerMutationAndCaseAliases(t *testing.T) {
	t.Parallel()
	b := fixtureBundle(t)
	b.Payload.Reproducer.Expected = "fail"
	b.Payload.Applicability.Workloads[0] = "other"
	b.Reproducer.Expected = "fail"
	b.Reproducer.Input.Case = "unobserved-control"
	b.Attestations.Reviewer = "forged"
	_, err := CheckAdoptionAt(context.Background(), b, receiver(), accept, activeCurrent, activeRevocation, unknown, func() time.Time { return lessonEpoch })
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("caller-mutated bundle accepted: %v", err)
	}
	lesson, reproducer, att := fixtureBytes(t)
	aliased := append(append([]byte(nil), lesson[:len(lesson)-1]...), []byte(`,"Version":"lesson-transfer/v1"}`)...)
	if _, err := ValidateBundle(aliased, reproducer, att); !errors.Is(err, ErrInvalid) {
		t.Fatalf("case duplicate accepted: %v", err)
	}
}

func TestCheckAdoptionCancellationCannotPass(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	_, err := CheckAdoptionAt(ctx, fixtureBundle(t), receiver(), accept, activeCurrent, activeRevocation, func(context.Context, Bundle, ReceiverSnapshot) (ReproductionOutcome, error) {
		cancel()
		return ReproductionOutcome{Classification: "unknown", Evidence: digestFill("9"), Detail: "cancellation test"}, nil
	}, func() time.Time { return lessonEpoch })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("%v", err)
	}
}

package lessontransfer

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
)

func TestLessonBundleRejectsMalformedUnicodeBeforeIdentityBinding(t *testing.T) {
	t.Parallel()
	for _, file := range []string{"lesson", "reproducer", "attestations"} {
		for _, escape := range []string{`\ud800`, `\udfff`, `\ud800\u0041`} {
			t.Run(file+escape, func(t *testing.T) {
				lesson, reproducer, attestations := unicodeBundle(t, file, escape)
				if _, err := ValidateBundle(lesson, reproducer, attestations); !errors.Is(err, ErrInvalid) {
					t.Fatalf("malformed Unicode accepted: %v", err)
				}
			})
		}
	}
}

func TestLessonBundlePreservesValidUnicode(t *testing.T) {
	t.Parallel()
	for _, file := range []string{"lesson", "reproducer", "attestations"} {
		for _, tc := range []struct{ escape, want string }{
			{`\ud834\udd1e`, "𝄞"}, {`\ufffd`, "�"}, {`\\ud800`, `\ud800`},
		} {
			t.Run(file+tc.escape, func(t *testing.T) {
				lesson, reproducer, attestations := unicodeBundle(t, file, tc.escape)
				bundle, err := ValidateBundle(lesson, reproducer, attestations)
				if err != nil {
					t.Fatal(err)
				}
				got := map[string]string{"lesson": bundle.Payload.ID, "reproducer": bundle.Reproducer.Invariant, "attestations": bundle.Attestations.Reviewer}[file]
				if got != tc.want {
					t.Fatalf("decoded identity changed: %q != %q", got, tc.want)
				}
			})
		}
	}
}

// Bind the modified raw bytes correctly, so rejection cannot be an incidental
// digest mismatch masking the decoder defect.
func unicodeBundle(t *testing.T, file, escape string) ([]byte, []byte, []byte) {
	t.Helper()
	lesson, reproducer, attestations := fixtureBytes(t)
	var payload Payload
	var att Attestations
	if err := json.Unmarshal(lesson, &payload); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(attestations, &att); err != nil {
		t.Fatal(err)
	}
	if file == "reproducer" {
		reproducer = bytes.Replace(reproducer, []byte(`"invariant":"unknown evidence cannot pass"`), []byte(`"invariant":"`+escape+`"`), 1)
		payload.Reproducer.RawDigest = digest(reproducerDigestDomain, reproducer)
		var err error
		lesson, err = json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
	}
	if file == "lesson" {
		lesson = bytes.Replace(lesson, []byte(`"id":"unknown-evidence"`), []byte(`"id":"`+escape+`"`), 1)
	}
	att.PayloadDigest, att.ReproducerDigest = digest(payloadDigestDomain, lesson), digest(reproducerDigestDomain, reproducer)
	var err error
	attestations, err = json.Marshal(att)
	if err != nil {
		t.Fatal(err)
	}
	if file == "attestations" {
		attestations = bytes.Replace(attestations, []byte(`"reviewer":"fixture-review"`), []byte(`"reviewer":"`+escape+`"`), 1)
	}
	return lesson, reproducer, attestations
}

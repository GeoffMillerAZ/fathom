package fusion

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func TestFingerprintHashReaderMatchesBytes(t *testing.T) {
	for _, data := range [][]byte{nil, []byte("sample\x00bytes"), bytes.Repeat([]byte("source\n"), 81901)} {
		got, err := FingerprintHashReader(bytes.NewReader(data))
		if err != nil || got != FingerprintHash(data) {
			t.Fatalf("reader=%s, %v; bytes=%s", got, err, FingerprintHash(data))
		}
	}
}
func TestFingerprintAndContentHashMatchesBothDigests(t *testing.T) {
	content := bytes.Repeat([]byte("source\n"), 81901)
	fingerprint, raw := FingerprintAndContentHash(content, ExtractorEpoch)
	sum := sha256.Sum256(content)
	if fingerprint != FingerprintHash(content) || raw != hex.EncodeToString(sum[:]) {
		t.Fatalf("fingerprint=%s raw=%s", fingerprint, raw)
	}
}

func TestFingerprintHashDoesNotAlterBackingBytes(t *testing.T) {
	data := bytes.Repeat([]byte{'x'}, 256)
	before := bytes.Clone(data)
	_ = FingerprintHash(data[:3])
	if !bytes.Equal(data, before) {
		t.Fatal("hash modified caller's backing array")
	}
}
func TestFingerprintHashReaderReturnsReadError(t *testing.T) {
	want := errors.New("read failed")
	if _, err := FingerprintHashReader(fingerprintErrorReader{want}); !errors.Is(err, want) {
		t.Fatalf("error=%v", err)
	}
}

type fingerprintErrorReader struct{ err error }

func (r fingerprintErrorReader) Read([]byte) (int, error) { return 0, r.err }

var _ io.Reader = fingerprintErrorReader{}

func TestFingerprintHashEpochReceiptDoesNotVouchForStat(t *testing.T) {
	run := &ingestRun{refs: []core.SourceRef{{Path: "sample.go"}}, facts: map[string]fileFact{"sample.go": {size: 4, hash: FingerprintHash([]byte("data"))}}}
	_, rows := ingestSummary("", run)
	if len(rows) != 1 || !strings.HasPrefix(rows[0].ContentHash, ExtractorEpoch+":unvouched:") {
		t.Fatalf("missing content epoch: %+v", rows)
	}
	if _, vouches := statRecordFromRow(rows[0]); vouches {
		t.Fatal("content identity vouched for post-read stat")
	}
}

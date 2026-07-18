package docextract

import (
	"errors"
	"strings"
	"testing"
)

func TestExtract_TextFormats(t *testing.T) {
	cases := []struct{ name, mt, body string }{
		{"notes.txt", "", "hello world"},
		{"data.csv", "", "a,b,c\n1,2,3"},
		{"x.md", "", "# Title\n\ntext"},
		{"blob", "text/plain", "plain by media type"},
		{"cfg.json", "application/json", `{"k":1}`},
	}
	for _, c := range cases {
		got, err := Extract(c.name, c.mt, []byte(c.body))
		if err != nil {
			t.Errorf("%s: unexpected error: %v", c.name, err)
		}
		if got != strings.TrimSpace(c.body) {
			t.Errorf("%s: got %q want %q", c.name, got, c.body)
		}
	}
}

func TestExtract_UTF8Fallback(t *testing.T) {
	got, err := Extract("mystery", "application/octet-stream", []byte("still text"))
	if err != nil || got != "still text" {
		t.Errorf("utf8 fallback: got %q err %v", got, err)
	}
}

func TestExtract_UnsupportedBinary(t *testing.T) {
	_, err := Extract("a.bin", "application/octet-stream", []byte{0x00, 0xff, 0x00})
	if err == nil {
		t.Error("expected error on binary payload with no handler")
	}
}

func TestExtract_HandlerWins(t *testing.T) {
	h := Handler{
		Name:    "fake",
		Match:   func(ext, mt string) bool { return ext == ".pdf" },
		Extract: func(data []byte) (string, error) { return "EXTRACTED", nil },
	}
	got, err := Extract("report.pdf", "application/pdf", []byte("binary"), WithHandlers(h))
	if err != nil || got != "EXTRACTED" {
		t.Errorf("handler path: got %q err %v", got, err)
	}
}

func TestExtract_HandlerError(t *testing.T) {
	h := Handler{
		Name:    "boom",
		Match:   func(ext, mt string) bool { return true },
		Extract: func(data []byte) (string, error) { return "", errors.New("nope") },
	}
	if _, err := Extract("x.pdf", "", nil, WithHandlers(h)); err == nil {
		t.Error("expected handler error to propagate")
	}
}

func TestExtract_Clamp(t *testing.T) {
	long := strings.Repeat("a", 100)
	got, err := Extract("big.txt", "", []byte(long), WithMaxChars(10))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got, strings.Repeat("a", 10)) || !strings.Contains(got, "truncated") {
		t.Errorf("clamp: got %q", got)
	}
	// No cap.
	full, _ := Extract("big.txt", "", []byte(long), WithMaxChars(0))
	if full != long {
		t.Errorf("uncapped: len %d want %d", len(full), len(long))
	}
}

func TestClamp_RuneBoundary(t *testing.T) {
	// A multibyte rune straddling the cut point must not be split.
	s := strings.Repeat("é", 20) // 40 bytes
	got := clamp(s, 15)
	if !strings.HasSuffix(strings.TrimSuffix(got, "\n…(truncated)"), "é") {
		t.Errorf("clamp split a rune: %q", got)
	}
}

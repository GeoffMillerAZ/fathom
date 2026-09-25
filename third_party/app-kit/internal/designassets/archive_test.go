package designassets

import (
	"bytes"
	"context"
	"encoding/binary"
	"io"
	"os"
	"strings"
	"testing"
)

type countedReaderAt struct {
	io.ReaderAt
	bytes int
}

func (r *countedReaderAt) ReadAt(p []byte, off int64) (int, error) {
	r.bytes += len(p)
	return r.ReaderAt.ReadAt(p, off)
}

func TestZIPPreflightRejectsWrappedCountBeforeAllocation(t *testing.T) {
	base := zipBytes(t, zipMember{"README.md", "x", 0644})
	cd := bytes.Index(base, []byte{'P', 'K', 1, 2})
	record := base[cd : len(base)-22]
	// Go's ZIP reader accepts EOCD counts modulo 65536, then allocates one
	// File for every actual record. Forge that exact discrepancy cheaply.
	body := append([]byte{}, base[:cd]...)
	body = append(body, bytes.Repeat(record, 65536)...)
	trailer := bytes.Clone(base[len(base)-22:])
	binary.LittleEndian.PutUint16(trailer[8:], 0)
	binary.LittleEndian.PutUint16(trailer[10:], 0)
	binary.LittleEndian.PutUint32(trailer[12:], uint32(len(record)*65536))
	body = append(body, trailer...)
	r := &countedReaderAt{ReaderAt: bytes.NewReader(body)}
	if _, err := boundedZIP(r, int64(len(body))); err == nil {
		t.Fatal("wrapped central-directory count accepted")
	}
	if r.bytes > 65557+46 {
		t.Fatalf("preflight read %d bytes before refusing forged count", r.bytes)
	}
	// An understated extent must also fail, even if its count looks bounded.
	binary.LittleEndian.PutUint16(body[len(body)-22+8:], 1)
	binary.LittleEndian.PutUint16(body[len(body)-22+10:], 1)
	binary.LittleEndian.PutUint32(body[len(body)-22+12:], uint32(len(record)))
	if _, err := boundedZIP(bytes.NewReader(body), int64(len(body))); err == nil {
		t.Fatal("understated central-directory extent accepted")
	}
}

func TestZIPPreflightRejectsNestedFooterBeforeAllocation(t *testing.T) {
	base := zipBytes(t, zipMember{"README.md", "x", 0644})
	cd := bytes.Index(base, []byte{'P', 'K', 1, 2})
	record := base[cd : len(base)-22]
	// The outer footer describes one bounded record. Its comment contains a
	// later footer pointing at 65,537 records (count wraps to one), followed
	// by padding. A parser that skips the later footer because of that padding
	// checks the outer directory, while archive/zip chooses the inner footer.
	body := append([]byte{}, base[:cd]...)
	body = append(body, bytes.Repeat(record, 65536)...)
	outerOffset := len(body)
	body = append(body, record...)
	outer := bytes.Clone(base[len(base)-22:])
	binary.LittleEndian.PutUint32(outer[16:], uint32(outerOffset))
	binary.LittleEndian.PutUint16(outer[20:], 22+8)
	body = append(body, outer...)
	inner := bytes.Clone(base[len(base)-22:])
	binary.LittleEndian.PutUint32(inner[12:], uint32(len(record)*65537))
	body = append(body, inner...)
	body = append(body, []byte("padding!")...)
	for _, truncated := range []bool{false, true} {
		fixture := bytes.Clone(body)
		if truncated {
			binary.LittleEndian.PutUint16(fixture[len(fixture)-8-2:], 500)
		}
		r := &countedReaderAt{ReaderAt: bytes.NewReader(fixture)}
		if _, err := boundedZIP(r, int64(len(fixture))); err == nil {
			t.Fatalf("nested footer accepted (truncated=%t)", truncated)
		}
		if r.bytes > 65557 {
			t.Fatalf("nested footer reached unchecked directory reads: bytes=%d truncated=%t", r.bytes, truncated)
		}
	}
	// A normal vendor comment remains supported; only ambiguity is refused.
	commented := bytes.Clone(base)
	comment := []byte("ordinary archive comment")
	binary.LittleEndian.PutUint16(commented[len(commented)-2:], uint16(len(comment)))
	commented = append(commented, comment...)
	if _, err := boundedZIP(bytes.NewReader(commented), int64(len(commented))); err != nil {
		t.Fatalf("ordinary ZIP comment refused: %v", err)
	}
}

func TestZIPPreflightRejectsDirectorySizeZIP64Sentinel(t *testing.T) {
	base := zipBytes(t, zipMember{"README.md", "x", 0644})
	cd := bytes.Index(base, []byte{'P', 'K', 1, 2})
	record := bytes.Clone(base[cd : len(base)-22])
	padding := 0xffff - len(record)
	binary.LittleEndian.PutUint16(record[32:], uint16(padding))
	body := append([]byte{}, base[:cd]...)
	body = append(body, record...)
	body = append(body, make([]byte, padding)...)
	footer := bytes.Clone(base[len(base)-22:])
	binary.LittleEndian.PutUint32(footer[12:], 0xffff)
	body = append(body, footer...)
	r := &countedReaderAt{ReaderAt: bytes.NewReader(body)}
	if _, err := boundedZIP(r, int64(len(body))); err == nil {
		t.Fatal("directory-size sentinel allowed archive/zip to probe another ZIP64 footer")
	}
	if r.bytes > 65557 {
		t.Fatalf("ZIP64 sentinel reached directory reads: %d bytes", r.bytes)
	}
}

func TestDirectoryLocalHeadersAndCRCValidated(t *testing.T) {
	base := zipBytes(t, zipMember{"project/", "", os.ModeDir | 0755}, zipMember{"README.md", "x", 0644})
	cd := bytes.Index(base, []byte{'P', 'K', 1, 2})
	for _, field := range []string{"crc", "offset", "local-signature"} {
		t.Run(field, func(t *testing.T) {
			body := bytes.Clone(base)
			switch field {
			case "crc":
				binary.LittleEndian.PutUint32(body[cd+16:], 1)
			case "offset":
				binary.LittleEndian.PutUint32(body[cd+42:], uint32(len(body)+100))
			case "local-signature":
				body[0] = 'X'
			}
			if _, _, err := inspect(context.Background(), bytes.NewReader(body), int64(len(body)), "studio.zip", digest(body)); err == nil {
				t.Fatal("invalid directory accepted")
			}
		})
	}
}

func TestDeclaredExpandedAndProjectBounds(t *testing.T) {
	members := make([]zipMember, 17)
	for i := range members {
		members[i] = zipMember{strings.Repeat("a", i+1) + ".txt", "x", 0644}
	}
	body := zipBytes(t, members...)
	for offset := 0; ; {
		i := bytes.Index(body[offset:], []byte{'P', 'K', 1, 2})
		if i < 0 {
			break
		}
		i += offset
		binary.LittleEndian.PutUint32(body[i+24:], uint32(MaxMemberBytes))
		offset = i + 46
	}
	if _, _, err := inspect(context.Background(), bytes.NewReader(body), int64(len(body)), "studio.zip", digest(body)); err == nil || !strings.Contains(err.Error(), "expanded archive") {
		t.Fatalf("expanded bound not enforced before reads: %v", err)
	}
	body = zipBytes(t, zipMember{"package.json", "{}", 0644})
	i := bytes.Index(body, []byte{'P', 'K', 1, 2})
	binary.LittleEndian.PutUint32(body[i+24:], uint32(MaxProjectBytes+1))
	if _, _, err := inspect(context.Background(), bytes.NewReader(body), int64(len(body)), "studio.zip", digest(body)); err == nil || !strings.Contains(err.Error(), "project metadata") {
		t.Fatalf("project bound not enforced before reads: %v", err)
	}
}

func TestClassificationsKeepOriginalRoles(t *testing.T) {
	cases := map[string]string{"README.md": "readme", "LICENSE.txt": "license", "src/theme.css": "theme", "src/app/page.tsx": "source", "public/font.woff2": "asset", "docs/setup.md": "documentation", "package.json": "manifest", "package-lock.json": "manifest", "next.config.ts": "source", ".env": "sensitive", ".env.example": "sensitive", "keys/client.pem": "sensitive", "unknown.blob": "other"}
	for name, want := range cases {
		if got := classify(name); got != want {
			t.Errorf("%s classified %s, want %s", name, got, want)
		}
	}
}

func TestCredentialDisclosureGuard(t *testing.T) {
	for _, text := range []string{"-----BEGIN PRIVATE KEY-----", "https://user:password@example.com/private", "https://example.com/api?access_token=value", "ghp_" + strings.Repeat("a", 36), "AKIA" + strings.Repeat("A", 16)} {
		if safeText(text) {
			t.Errorf("credential-shaped text accepted: %q", text)
		}
	}
	if !safeText("export const button = '<button>Example</button>'\n") {
		t.Fatal("ordinary source refused")
	}
}

package jsonwire

import (
	"encoding/json"
	"fmt"
	"testing"
	"unicode/utf8"
)

func TestValidUnicode(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		raw  string
		want bool
	}{
		{`{"id":"\ud800"}`, false}, {`{"id":"\udfff"}`, false},
		{`{"id":"\ud800\ud800"}`, false}, {`{"id":"\ud800\u0041"}`, false},
		{`{"id":"\udc00\ud800"}`, false}, {`{"\udfff":0}`, false},
		{`["\ud800","\udc00"]`, false}, {`"\ud800\\udc00"`, false},
		{`"\ud800" "\udc00"`, false}, {`"\ud800\"\udc00"`, false},
		{`"\ud800\uZZZZ"`, false}, {`"\u000g"`, false},
		{`"\u"`, false}, {`"\ud800\u"`, false}, {`"\`, false}, {`"`, false},
		{"\"\xff\"", false}, {"\"\xed\xa0\x80\"", false},
		{`{"id":"\ud800\udc00"}`, true}, {`"\uDBFF\uDFFF"`, true},
		{`"\ud834\udd1e\ud834\udd1e"`, true}, {`"\ud834\udd1e\udc00"`, false},
		{`"\ud7ff\ue000"`, true}, {`"\ufffd"`, true}, {`"�"`, true},
		{`"\\ud800"`, true}, {`"\u005cud800"`, true},
		{`"quote:\";backslash:\\;slash:\/"`, true}, {`["𝄞","Café"]`, true},
		// This helper is not a JSON grammar check; callers still use a decoder.
		{`not JSON`, true},
	} {
		t.Run(tc.raw, func(t *testing.T) {
			if got := ValidUnicode([]byte(tc.raw)); got != tc.want {
				t.Fatalf("got=%v want=%v", got, tc.want)
			}
		})
	}
}

func TestEveryEscapedUTF16CodeUnit(t *testing.T) {
	t.Parallel()
	for unit := 0; unit <= 0xffff; unit++ {
		raw := fmt.Appendf(nil, `"\u%04x"`, unit)
		want := unit < 0xd800 || unit > 0xdfff
		if ValidUnicode(raw) != want {
			t.Fatalf("standalone code unit U+%04X", unit)
		}
	}
}

func FuzzValidUnicodeRoundTrip(f *testing.F) {
	for _, value := range []string{"", "𝄞", "�", `\ud800`, "a\"b\\c", "\x00", "Café"} {
		f.Add(value)
	}
	f.Fuzz(func(t *testing.T, value string) {
		if len(value) > 64<<10 || !utf8.ValidString(value) {
			t.Skip()
		}
		raw, err := json.Marshal(value)
		if err != nil || !ValidUnicode(raw) {
			t.Fatalf("valid string rejected: %q: %v", raw, err)
		}
		var decoded string
		if err := json.Unmarshal(raw, &decoded); err != nil || decoded != value {
			t.Fatal("valid string lost exact bytes")
		}
		// Always inject an unpaired high surrogate immediately before the closing
		// quote, regardless of the arbitrary preceding escaped string content.
		malformed := append(append([]byte(nil), raw[:len(raw)-1]...), []byte(`\ud800"`)...)
		if ValidUnicode(malformed) {
			t.Fatalf("injected malformed scalar accepted: %q", malformed)
		}
	})
}

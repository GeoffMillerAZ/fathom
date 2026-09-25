package gallery

import (
	"net/url"
	"strings"
	"testing"
)

func TestConversionSourcePaneExactWindows(t *testing.T) {
	for _, content := range []string{"", "one", "one\n", "\n", "\n\n", "one\r\ntwo\r\nthree", "one\ntwo\nthree\n", "日本語\n<unsafe>\nlast"} {
		for _, limit := range []int{1, 2, 100, 400} {
			var combined strings.Builder
			start := 1
			for {
				p, err := conversionSourcePane(content, start, limit, "source_line", "conversion-original", "/conversions/test", url.Values{"artifact_line": {"3"}, "theme": {"dark"}})
				if err != nil || p.TooLarge {
					t.Fatal("valid bounded source window failed", err)
				}
				combined.WriteString(p.Content)
				if p.Next == "" {
					if p.End != p.Total {
						t.Fatal("premature source continuation end")
					}
					break
				}
				u, err := url.Parse(p.Next)
				if err != nil || u.Query().Get("artifact_line") != "3" || u.Query().Get("theme") != "dark" || u.Fragment != "conversion-original" {
					t.Fatal("independent pane context changed")
				}
				start = p.End + 1
			}
			if combined.String() != content {
				t.Fatalf("line pages changed source bytes for limit %d", limit)
			}
		}
	}
}

func TestConversionSourcePaneBoundsAndRecovery(t *testing.T) {
	for _, window := range [][2]int{{0, 1}, {3, 1}, {1, 0}, {1, 401}} {
		if _, err := conversionSourcePane("a\nb\n", window[0], window[1], "source_line", "conversion-original", "/conversions/test", url.Values{}); err == nil {
			t.Fatal("invalid line window accepted", window)
		}
	}
	content := strings.Repeat("x", 32<<10) + "\nshort\n"
	p, err := conversionSourcePane(content, 1, 1, "source_line", "conversion-original", "/conversions/test", url.Values{})
	if err != nil || !p.TooLarge || p.Content != "" || p.Bytes != (32<<10)+1 || p.Next == "" {
		t.Fatal("oversized window was truncated or lost recovery", err)
	}
	p, err = conversionSourcePane(content, 2, 1, "source_line", "conversion-original", "/conversions/test", url.Values{})
	if err != nil || p.TooLarge || p.Content != "short\n" || p.Previous == "" {
		t.Fatal("bounded later line could not be recovered", err)
	}
	p, err = conversionSourcePane(strings.Repeat("x", 32<<10), 1, 1, "source_line", "conversion-original", "/conversions/test", url.Values{})
	if err != nil || p.TooLarge || len(p.Content) != 32<<10 {
		t.Fatal("exact byte limit was refused", err)
	}
}

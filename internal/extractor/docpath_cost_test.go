package extractor

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// These are selectors, not document contents. The ASCII space class is
// deliberately narrower than Unicode whitespace: keys may contain NBSP,
// em-space, vertical tab and invalid UTF-8 bytes inside the trimmed selector.
func TestDocPathSelectorTokenGrammar(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want []docStep
	}{
		{"root", " $ ", nil},
		{"relative", ".worker", []docStep{{kind: stepKey, key: "worker"}}},
		{"index", "$.items[001].value", []docStep{{kind: stepKey, key: "items"}, {kind: stepIndex, index: 1}, {kind: stepKey, key: "value"}}},
		{"wildcards", "$.*[*]..key", []docStep{{kind: stepAllValues}, {kind: stepAllItems}, {kind: stepDescend, key: "key"}}},
		{"where", "$..{kind=task}.name", []docStep{{kind: stepDescendWhere, key: "kind", want: "task"}, {kind: stepKey, key: "name"}}},
		{"unicode", "$.café.ключ.中文", []docStep{{kind: stepKey, key: "café"}, {kind: stepKey, key: "ключ"}, {kind: stepKey, key: "中文"}}},
		{"non_ascii_space", "$.a\u00a0b.\u2003child", []docStep{{kind: stepKey, key: "a\u00a0b"}, {kind: stepKey, key: "\u2003child"}}},
		{"vertical_tab", "$.a\vb", []docStep{{kind: stepKey, key: "a\vb"}}},
		{"invalid_utf8", "$.a\xffb.\xc0\x80tail", []docStep{{kind: stepKey, key: "a\xffb"}, {kind: stepKey, key: "\xc0\x80tail"}}},
		{"punctuation", `$.a:b\c@d`, []docStep{{kind: stepKey, key: `a:b\c@d`}}},
		{"repeated_indices", "$[1][2]..b", []docStep{{kind: stepIndex, index: 1}, {kind: stepIndex, index: 2}, {kind: stepDescend, key: "b"}}},
		{"long_zero_index", "$[" + strings.Repeat("0", 40) + "]", []docStep{{kind: stepIndex}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseDocPath(tc.src)
			if err != nil || !reflect.DeepEqual(got.steps, tc.want) {
				t.Fatalf("parseDocPath(%q) = %+v, %v; want %+v", tc.src, got.steps, err, tc.want)
			}
		})
	}
}

func TestDocPathSelectorTokenRefusals(t *testing.T) {
	unexpected := func(tail string) string {
		return fmt.Sprintf("selector: unexpected %q (the subset is $ .key .* [*] [N] ..key)", tail)
	}
	overflow := "[" + strings.Repeat("9", 40) + "]"
	cases := []struct{ src, want string }{
		{"", "empty selector"},
		{" \t\r\n", "empty selector"},
		{"items", `selector "items" must start with $ or .`},
		{"$$.a", unexpected("$.a")},
		{"$.", `selector: bad key at "."`},
		{"$..", "selector: `..` needs a key at \"..\""},
		{"$..*", "selector: `..` needs a key at \"..*\""},
		{"$[1", unexpected("[1")},
		{"$[]", unexpected("[]")},
		{"$[-1]", unexpected("[-1]")},
		{"$[+1]", unexpected("[+1]")},
		{"$[ 1]", unexpected("[ 1]")},
		{"$[1x]", unexpected("[1x]")},
		{"$[١]", unexpected("[١]")},
		{"$[1.0]", unexpected("[1.0]")},
		{"$[0]suffix", unexpected("suffix")},
		{"$" + overflow, fmt.Sprintf("selector: bad index at %q", overflow)},
		{"$..{key=value", "selector: unclosed `..{` at \"..{key=value\""},
		{"$..{key=}", "selector: `..{key=value}` needs a key and a value at \"..{key=}\""},
		{"$..{=value}", "selector: `..{key=value}` needs a key and a value at \"..{=value}\""},
		{"$.key|other", unexpected("|other")},
	}
	for _, space := range []string{" ", "\t", "\n", "\f", "\r"} {
		cases = append(cases, struct{ src, want string }{"$.key" + space + "next", unexpected(space + "next")})
	}
	for _, tc := range cases {
		got, err := parseDocPath(tc.src)
		if err == nil || err.Error() != tc.want || len(got.steps) != 0 {
			t.Errorf("parseDocPath(%q) = %+v, %v; want empty path and %q", tc.src, got.steps, err, tc.want)
		}
	}
}

// All byte values exercise the original selector grammar's boundary, including
// malformed UTF-8. Its excluded class is ASCII . [ ] | * and [\t\n\f\r ].
// A broader unicode.IsSpace test or rune replacement would change valid keys.
func TestDocPathKeyByteBoundaries(t *testing.T) {
	for i := 0; i < 256; i++ {
		c := byte(i)
		key := "a" + string([]byte{c}) + "b"
		got, err := parseDocPath("$." + key)
		switch {
		case c == '.':
			want := []docStep{{kind: stepKey, key: "a"}, {kind: stepKey, key: "b"}}
			if err != nil || !reflect.DeepEqual(got.steps, want) {
				t.Fatalf("dot boundary = %+v, %v; want %+v", got.steps, err, want)
			}
		case strings.ContainsRune("[]|* \t\n\f\r", rune(c)):
			if err == nil {
				t.Fatalf("reserved byte 0x%02x accepted in key: %+v", c, got.steps)
			}
		default:
			want := []docStep{{kind: stepKey, key: key}}
			if err != nil || !reflect.DeepEqual(got.steps, want) {
				t.Fatalf("key byte 0x%02x = %+v, %v; want %+v", c, got.steps, err, want)
			}
		}
	}
}

// Selector storage may grow with the number of steps; token recognition must
// not allocate regex capture/scratch buffers for every step. A 16x larger
// selector gets ample room for geometric output-slice growth, independent of
// wall time or CPU speed. The old [N] submatch alone allocates once per index,
// even before race-instrumented regexp pool churn.
func TestDocPathTokenAllocationDoesNotGrowPerIndex(t *testing.T) {
	measure := func(pairs int) float64 {
		src := "$" + strings.Repeat(".items[0]", pairs)
		var got docPath
		var err error
		allocs := testing.AllocsPerRun(100, func() {
			got, err = parseDocPath(src)
		})
		if err != nil || len(got.steps) != pairs*2 {
			t.Fatalf("allocation fixture lost steps: %d, %v; want %d", len(got.steps), err, pairs*2)
		}
		return allocs
	}
	small, large := measure(4), measure(64)
	t.Logf("selector allocations: 8 steps %.0f, 128 steps %.0f", small, large)
	if large > small+8 {
		t.Fatalf("selector token allocations grow per index: %.0f for 8 steps, %.0f for 128; want at most 8 additional allocations for output growth", small, large)
	}
}

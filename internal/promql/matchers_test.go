package promql

import (
	"reflect"
	"testing"
)

// TestParseMatchers (wave c3, LB-F361): an Alertmanager matcher list is read
// by the selector scanner's own brace grammar, with the one thing
// Alertmanager adds: an unquoted value, to the next comma or the closing
// brace. The braces are optional; quoted names and values keep the
// scanner's escapes.
func TestParseMatchers(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{`alertname = Watchdog`, []string{"alertname=Watchdog"}},
		{`severity =~ warning|info`, []string{"severity=~warning|info"}},
		{`severity="critical"`, []string{"severity=critical"}},
		{`{team="a", service!~"x|y"}`, []string{"team=a", "service!~x|y"}},
		{`foo=bar,baz!=qux`, []string{"foo=bar", "baz!=qux"}},
		{`env = "prod, eu"`, []string{"env=prod, eu"}},
		{`msg="a \"quoted\" word"`, []string{`msg=a "quoted" word`}},
		{`"service.name" = cart`, []string{"service.name=cart"}},
		{`instance=`, []string{"instance="}},
		{`{}`, nil},
		{`  `, nil},
	}
	for _, c := range cases {
		got, err := ParseMatchers(c.in)
		if err != nil {
			t.Errorf("ParseMatchers(%q): %v", c.in, err)
			continue
		}
		var s []string
		for _, m := range got {
			s = append(s, m.Name+m.Op+m.Value)
		}
		if !reflect.DeepEqual(s, c.want) {
			t.Errorf("ParseMatchers(%q) = %q, want %q", c.in, s, c.want)
		}
	}
	for _, bad := range []string{`foo`, `"name alone"`, `=bar`, `foo=b"ar`, `{foo="bar"`, `foo=="bar"x`} {
		if got, err := ParseMatchers(bad); err == nil {
			t.Errorf("ParseMatchers(%q) = %v, want an error", bad, got)
		}
	}
}

// TestParseMatchersLeavesSelectorsStrict: the lenient value is
// ParseMatchers' alone; an expression's selector still needs a quoted
// value (LB-F316 unchanged).
func TestParseMatchersLeavesSelectorsStrict(t *testing.T) {
	if _, err := Selectors(`up{job=api}`); err == nil {
		t.Fatal("Selectors accepted an unquoted matcher value")
	}
}

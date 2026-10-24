package fusion

// pgbouncerdispatch_test.go — LB-Q-pgbouncer: a pgbouncer.ini is claimed by the
// gateway hub by a manifest basename or glob, or, for any other .ini, by the
// dialect's content sniff, AND the doc spec's content sniff; an .ini of another
// tool stays unclaimed.

import "testing"

func TestPgBouncerDispatchClaims(t *testing.T) {
	const pgb = "[databases]\napp = host=db dbname=app\n\n[pgbouncer]\nlisten_port = 6432\n"
	cases := []struct {
		path, content string
		want          ToolKind
	}{
		{"etc/pgbouncer.ini", pgb, ToolPgBouncer},
		{"deploy/pgbouncer-minimal.ini", pgb, ToolPgBouncer},
		{"deploy/pgbouncer/main.ini", pgb, ToolPgBouncer},
		{"test/test.ini", pgb, ToolPgBouncer},
		{"test/ctest.ini", "\n; comment\n[pgbouncer]\nlisten_port = 7000\n", ToolPgBouncer},
		{"app/settings.ini", "[core]\nname = x\n", ""},
		{"app/tox.ini", "[tox]\nenvlist = py3\n", ""},
		{"etc/pgbouncer.ini", "name = not an ini\n", ""},
	}
	for _, c := range cases {
		e, tk, ok := gatewayDispatch(c.path, []byte(c.content))
		if c.want == "" {
			if ok || e != nil {
				t.Errorf("gatewayDispatch(%q) claimed it as %q; want unclaimed", c.path, tk)
			}
			continue
		}
		if !ok || tk != c.want || e == nil {
			t.Errorf("gatewayDispatch(%q) = (%v, %q, %v), want %q", c.path, e, tk, ok, c.want)
		}
	}
}

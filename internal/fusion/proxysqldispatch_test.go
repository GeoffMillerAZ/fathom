package fusion

// proxysqldispatch_test.go — LB-Q-proxysql: a proxysql.cnf is claimed by the
// gateway hub by a manifest basename or glob AND the doc spec's content sniff; a
// file of that name that is not a libconfig ProxySQL configuration stays unread.

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/langroute"
)

func TestProxySQLDispatchClaims(t *testing.T) {
	const cnf = "datadir=\"/var/lib/proxysql\"\n\nmysql_servers =\n(\n\t{ address=\"db\" , port=3306 , hostgroup=0 }\n)\n"
	cases := []struct {
		path, content string
		want          ToolKind
	}{
		{"etc/proxysql.cnf", cnf, ToolProxySQL},
		{"src/proxysql.cfg", cnf, ToolProxySQL},
		{"test/cluster/confs/proxysql01.cfg", "admin_variables=\n{\n\tadmin_credentials=\"a:b\"\n}\n", ToolProxySQL},
		{"docker/proxysql/ci.cnf", "mysql_variables=\n{\n\tthreads=4\n}\n", ToolProxySQL},
		{"etc/proxysql.cnf", "[mysqld]\nport = 3306\n", ""},
		{"etc/my.cnf", cnf, ""},
		{"conf/haproxy.cfg", cnf, ""},
	}
	for _, c := range cases {
		e, tk, ok := langroute.GatewayDispatch(c.path, []byte(c.content))
		if c.want == "" {
			if ok || e != nil {
				t.Errorf("langroute.GatewayDispatch(%q) claimed it as %q; want unclaimed", c.path, tk)
			}
			continue
		}
		if !ok || tk != c.want || e == nil {
			t.Errorf("langroute.GatewayDispatch(%q) = (%v, %q, %v), want %q", c.path, e, tk, ok, c.want)
		}
	}
}

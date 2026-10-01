package main

import "testing"

func TestHostOf(t *testing.T) {
	for in, want := range map[string]string{
		"auth":                       "auth",
		"auth:9999":                  "auth",
		"http://auth:9999/verify":    "auth",
		"https://u:p@host.example/x": "host.example",
		"http://[::1]:80/":           "::1",
		"10.0.0.5:8000":              "10.0.0.5",
		"":                           "",
	} {
		if got := hostOf(in); got != want {
			t.Errorf("hostOf(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestClassify(t *testing.T) {
	d := deployed{"auth": {"docker-compose.yml"}, "db": {"a.yml", "b.yml"}}
	for _, c := range []struct{ raw, toPath, to, reason string }{
		{"http://auth:9999/v1", "docker-compose.yml", "auth", ""},
		{"auth:80", "docker-compose.yml", "auth", ""},
		{"db:5432", "", "", "ambiguous"},
		{"10.0.0.5:80", "", "", "ip-literal"},
		{"http://[::1]:80", "", "", "ip-literal"},
		{"api.example.com", "", "", "external"},
		{"${UPSTREAM}:80", "", "", "variable"},
		{"{{ .Host }}", "", "", "variable"},
		{"unix:/var/run/x.sock", "", "", "unix-socket"},
		{"cache", "", "", "undeclared"},
	} {
		tp, to, reason := d.classify(c.raw)
		if tp != c.toPath || to != c.to || reason != c.reason {
			t.Errorf("classify(%q) = %q, %q, %q; want %q, %q, %q", c.raw, tp, to, reason, c.toPath, c.to, c.reason)
		}
	}
}

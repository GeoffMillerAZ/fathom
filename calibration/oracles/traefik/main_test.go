package main

import "testing"

func TestTOMLLocLines(t *testing.T) {
	src := []byte("[entryPoints.web]\n  address = \":80\"\n\n[http.routers.site]\n  rule = \"x\"\n\n[http.services.site.loadBalancer]\n  [[http.services.site.loadBalancer.servers]]\n    url = \"u\"\n")
	l, err := newTOMLLoc(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		keys      []string
		line, end int
	}{
		{[]string{"entryPoints", "web"}, 1, 2},
		{[]string{"http", "routers", "site"}, 4, 5},
		{[]string{"http", "services", "site"}, 7, 9},
	} {
		if line, end := l.find(c.keys...); line != c.line || end != c.end {
			t.Errorf("find(%v) = %d-%d, want %d-%d", c.keys, line, end, c.line, c.end)
		}
	}
	if l.rootLine() != 1 {
		t.Errorf("rootLine = %d", l.rootLine())
	}
}

func TestStaticCopyKeepsFileProviderAndMovesEntryPoints(t *testing.T) {
	fi, ok := decodeFile("/tmp/s.yml", []byte("entryPoints:\n  web:\n    address: ':80'\nproviders:\n  docker: {}\n  file:\n    filename: dyn.yml\n"))
	if !ok {
		t.Fatal("decode")
	}
	out := string(staticCopy(fi, "/srv/cfg", 1234))
	for _, want := range []string{"127.0.0.1:1234", "/srv/cfg/dyn.yml", "insecure: true"} {
		if !contains(out, want) {
			t.Errorf("static copy lacks %q:\n%s", want, out)
		}
	}
	if contains(out, "docker") || contains(out, ":80") {
		t.Errorf("static copy keeps what it must drop:\n%s", out)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

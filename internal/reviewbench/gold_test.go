package reviewbench

import (
	"testing"
)

func strp(s string) *string { return &s }

func TestExtractGold_HunkHeaderAndFullDecl(t *testing.T) {
	pr := PRBRef{
		FilesChanged: []PRFile{
			{
				Filename: "cmd/commandfuncs.go",
				Patch: strp("@@ -735,7 +735,7 @@ func DetermineAdminAPIAddress(address, configFile, configAdapter string) (string, error) {\n" +
					" \t\t// unchanged context line\n" +
					"-\t\t// get the address of the admin listener\n" +
					"+\t\t// get the address of the admin listener if set\n"),
			},
			{
				Filename: "caddyhttp/proxy/proxy.go",
				Patch: strp("@@ -247,12 +247,28 @@ func createUpstreamRequest(r *http.Request) *http.Request {\n" +
					"+func newHelper() {}\n" +
					"+\treturn nil\n"),
			},
			{
				// A method declaration's hunk header carries a receiver
				// clause — the captured name must be the method, not the
				// receiver type.
				Filename: "caddyhttp/proxy/reverseproxy.go",
				Patch:    strp("@@ -10,3 +10,4 @@ func (rp *ReverseProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {\n+\t_ = w\n"),
			},
			{
				// No patch (binary/omitted file) contributes nothing.
				Filename: "assets/logo.png",
				Patch:    nil,
			},
		},
	}

	got := ExtractGold(pr)
	want := map[GoldEntry]bool{
		{File: "cmd/commandfuncs.go", Func: "DetermineAdminAPIAddress"}:   true,
		{File: "caddyhttp/proxy/proxy.go", Func: "createUpstreamRequest"}: true,
		{File: "caddyhttp/proxy/proxy.go", Func: "newHelper"}:             true,
		{File: "caddyhttp/proxy/reverseproxy.go", Func: "ServeHTTP"}:      true,
	}
	if len(got) != len(want) {
		t.Fatalf("ExtractGold returned %d entries, want %d: %+v", len(got), len(want), got)
	}
	for _, g := range got {
		if !want[g] {
			t.Errorf("unexpected gold entry %+v", g)
		}
	}
}

func TestExtractGold_Deduplicates(t *testing.T) {
	pr := PRBRef{
		FilesChanged: []PRFile{
			{Filename: "a.go", Patch: strp("@@ -1,2 +1,2 @@ func Foo() {\n-old\n+new\n")},
			{Filename: "a.go", Patch: strp("@@ -1,2 +1,2 @@ func Foo() {\n-old2\n+new2\n")},
		},
	}
	got := ExtractGold(pr)
	if len(got) != 1 {
		t.Fatalf("ExtractGold = %+v, want exactly 1 deduplicated entry", got)
	}
}

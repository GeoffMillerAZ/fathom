package deps

import (
	"context"
	"encoding/json"
	"io/fs"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
)

func TestDependencyDoor(t *testing.T) {
	repo := fstest.MapFS{
		"go.mod":     &fstest.MapFile{Data: []byte("module example.com/control\n")},
		"lib/lib.go": &fstest.MapFile{Data: []byte("package lib\n")},
	}
	src := []byte("package app\nimport \"example.com/control/lib\"\n")
	r, err := New()
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.ReadFile(context.Background(), repo, "app.go", "go", src)
	if err != nil || got.Complete || got.Measured || got.Trusted || !hasUnknown(got, UnmeasuredReader) {
		t.Fatalf("default evidence: %+v, %v", got, err)
	}
	if !hasEdge(got, Import, Directory, "lib") || !hasEdge(got, ConfigRef, File, "go.mod") {
		t.Fatalf("local import or module input missing: %+v", got)
	}

	r, err = New(TrustUnmeasured("go"), TrustUnmeasured("unimplemented"))
	if err != nil {
		t.Fatal(err)
	}
	got, err = r.ReadFile(context.Background(), repo, "app.go", "go", src)
	if err != nil || !got.Complete || got.Measured || !got.Trusted || got.Unknowns == nil {
		t.Fatalf("explicit trust: %+v, %v", got, err)
	}
	for _, c := range []struct {
		dialect string
		source  []byte
	}{
		{"go", nil},
		{"go", []byte("package \nprivate-sentinel")},
		{"unimplemented", src},
	} {
		got, err = r.ReadFile(context.Background(), repo, "app.go", c.dialect, c.source)
		data, _ := json.Marshal(got)
		if err != nil || got.Complete || len(got.Unknowns) == 0 || got.Edges == nil ||
			strings.Contains(string(data), "private-sentinel") {
			t.Fatalf("insufficient evidence: %s, %v", data, err)
		}
	}
	if _, err := r.ReadFile(context.Background(), repo, "/private-sentinel/app.go", "go", src); err == nil ||
		strings.Contains(err.Error(), "private-sentinel") {
		t.Fatalf("unsafe path error: %v", err)
	}
	if got, err := r.ReadFiles(context.Background(), repo, nil); err == nil || got == nil || len(got) != 0 {
		t.Fatalf("empty batch: %v, %v", got, err)
	}
}

type resolverFunc func(fs.FS, string, string) ([]Edge, *Unknown)

func (f resolverFunc) Resolve(repo fs.FS, from, raw string) ([]Edge, *Unknown) {
	return f(repo, from, raw)
}

func TestDependencyExtensionValidation(t *testing.T) {
	repo := fstest.MapFS{"go.mod": &fstest.MapFile{Data: []byte("module example.com/control\n")}}
	src := []byte("package app\nimport \"example.org/lib\"\n")
	r, err := New(TrustUnmeasured("go"), WithResolver("go", resolverFunc(func(view fs.FS, from, raw string) ([]Edge, *Unknown) {
		if from != "main.go" || raw != "example.org/lib" {
			t.Fatalf("resolver operands: %s %s", from, raw)
		}
		return []Edge{
			{Kind: Import, Resolution: External, To: "go:example.org/lib", Version: "v1.2.3"},
			{Kind: Import, Resolution: File, To: "/private-sentinel"},
		}, &Unknown{Code: "custom-sentinel", Raw: "https://user:private-sentinel@example.org", DecidableBy: []DecidableBy{"custom-sentinel"}}
	})), WithDeclaredEdges(Edge{From: "main.go", Kind: BuildInput, Resolution: File, To: "settings.json"}))
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.ReadFile(context.Background(), repo, "main.go", "go", src)
	if err != nil || got.Complete || !hasUnknown(got, UnsupportedConstruct) ||
		!hasEdge(got, BuildInput, File, "settings.json") ||
		!hasEdge(got, Import, External, "go:example.org/lib") {
		t.Fatalf("extension evidence: %+v, %v", got, err)
	}
	for _, e := range got.Edges {
		if e.To == "go:example.org/lib" && (e.Provenance != Extension || e.Version != "v1.2.3" || e.Line != 2) {
			t.Fatalf("extension provenance: %+v", e)
		}
	}
	encoded, _ := json.Marshal(got)
	if strings.Contains(string(encoded), "sentinel") || strings.Contains(string(encoded), `"decidable_by":null`) {
		t.Fatalf("unsafe extension evidence: %s", encoded)
	}
	for _, e := range []Edge{
		{From: "main.go", Kind: Import, Resolution: External, To: "go:https://example.org"},
		{From: "../main.go", Kind: Include, Resolution: File, To: "lib"},
		{From: "main.go", Kind: "invented", Resolution: File, To: "lib"},
	} {
		if _, err := New(WithDeclaredEdges(e)); err == nil {
			t.Fatalf("accepted invalid declaration: %+v", e)
		}
	}
}

func TestGoDependencyDisclosure(t *testing.T) {
	repo := fstest.MapFS{
		"go.mod":       &fstest.MapFile{Data: []byte("module example.com/control\nrequire example.org/lib v1.2.3\n")},
		"payload.txt":  &fstest.MapFile{Data: []byte("contents")},
		"companion.go": &fstest.MapFile{Data: []byte("package app\n")},
	}
	r, err := New(TrustUnmeasured("go"))
	if err != nil {
		t.Fatal(err)
	}
	src := []byte("package app\nimport (\n_ \"embed\"\n_ \"example.org/lib/sub\"\n_ \"relative/path\"\n)\n//go:embed payload.txt *.cfg\nvar payload string\n")
	got, err := r.ReadFile(context.Background(), repo, "main.go", "go", src)
	if err != nil || got.Complete || !hasEdge(got, Embed, File, "payload.txt") ||
		!hasEdge(got, BuildInput, File, "companion.go") || !hasUnknown(got, ExternalUnpinned) ||
		!hasUnknown(got, UnboundedGlob) {
		t.Fatalf("unresolved Go constructs: %+v, %v", got, err)
	}
	for _, e := range got.Edges {
		if e.To == "go:example.org/lib/sub" && e.Version == "" {
			return
		}
	}
	t.Fatalf("minimum module requirement was treated as a selected pin: %+v", got)
}

func TestGoDependencyContext(t *testing.T) {
	for _, c := range []struct{ name, file, data string }{
		{"workspace", "go.work", "go 1.26\nuse .\n"},
		{"vendor", "vendor/modules.txt", "module list"},
		{"replacement", "go.mod", "module example.com/control\nreplace example.org/other => ../other\n"},
		{"tagged companion", "tagged.go", "//go:build custom\n\npackage app\n"},
		{"platform companion", "companion_linux.go", "package app\n"},
		{"native companion", "companion.s", "TEXT"},
		{"nested module", "lib/go.mod", "module example.org/other\n"},
		{"broken companion", "broken.go", "package app\nfunc {"},
	} {
		t.Run(c.name, func(t *testing.T) {
			repo := fstest.MapFS{
				"go.mod":     &fstest.MapFile{Data: []byte("module example.com/control\n")},
				"lib/lib.go": &fstest.MapFile{Data: []byte("package lib\n")},
				c.file:       &fstest.MapFile{Data: []byte(c.data)},
			}
			r, err := New(TrustUnmeasured("go"))
			if err != nil {
				t.Fatal(err)
			}
			got, err := r.ReadFile(context.Background(), repo, "app.go", "go", []byte("package app\nimport _ \"example.com/control/lib\"\n"))
			if err != nil || got.Complete || len(got.Unknowns) == 0 {
				t.Fatalf("unresolved Go context: %+v, %v", got, err)
			}
		})
	}
}

func hasEdge(got FileDeps, kind Kind, resolution Resolution, target string) bool {
	for _, e := range got.Edges {
		if e.Kind == kind && e.Resolution == resolution && e.To == target {
			return true
		}
	}
	return false
}

func hasUnknown(got FileDeps, code UnknownCode) bool {
	for _, u := range got.Unknowns {
		if u.Code == code {
			return true
		}
	}
	return false
}

func TestDependencyAdmission(t *testing.T) {
	for _, tc := range []struct {
		state                       admission
		trust, known, bad           bool
		complete, measured, trusted bool
	}{
		{state: unmeasured, known: true},
		{state: unmeasured, known: true, trust: true, complete: true, trusted: true},
		{state: unmeasured, known: true, trust: true, bad: true, trusted: true},
		{state: unmeasured, trust: true},
		{state: measured, known: true, complete: true, measured: true},
		{state: detectOnly, known: true, trust: true},
	} {
		f := newResult("file.go", "go")
		if tc.bad {
			f.addUnknown(Unknown{Code: MissingConfig})
		}
		f.finish(tc.state, tc.known, tc.trust)
		if f.Complete != tc.complete || f.Measured != tc.measured || f.Trusted != tc.trusted {
			t.Errorf("%+v produced %+v", tc, f)
		}
	}
}

func TestDependencyReadCapAndCancellation(t *testing.T) {
	repo := fstest.MapFS{
		"big.go": &fstest.MapFile{Data: make([]byte, maxReadBytes+1)},
		"go.mod": &fstest.MapFile{Data: []byte("module example.com/control\n")},
	}
	r, err := New(TrustUnmeasured("go"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.ReadFile(context.Background(), repo, "big.go", "go", repo["big.go"].Data)
	if err != nil || got.Complete || !hasUnknown(got, ReadLimit) || len(got.Edges) != 0 {
		t.Fatalf("oversized byte input: %+v, %v", got, err)
	}
	batch, err := r.ReadFiles(context.Background(), repo, []Input{{Path: "big.go"}})
	if err == nil || len(batch) != 1 || !hasUnknown(batch[0], ReadLimit) || batch[0].Complete {
		t.Fatalf("oversized batch input: %+v, %v", batch, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	batch, err = r.ReadFiles(ctx, repo, []Input{{Path: "big.go"}})
	if err != context.Canceled || batch == nil || len(batch) != 0 {
		t.Fatalf("canceled read: %+v, %v", batch, err)
	}
}

func TestDependencyRawRefusal(t *testing.T) {
	r, err := New(WithDeclaredEdges(Edge{From: "main.go", Kind: ConfigRef, To: "config.json",
		Resolution: File, Raw: "file:///outside-sentinel"},
		Edge{From: "main.go", Kind: ConfigRef, To: "other.json",
			Resolution: File, Raw: "password:outside-sentinel"}))
	if err != nil {
		t.Fatal(err)
	}
	repo := fstest.MapFS{"go.mod": &fstest.MapFile{Data: []byte("module example.com/control\n")}}
	got, err := r.ReadFile(context.Background(), repo, "main.go", "go", []byte(
		"package app\nimport \"https://user:outside-sentinel@example.org\"\n//line /outside-sentinel:300\nvar x = 1\n"))
	if err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(got)
	if got.Complete || !hasUnknown(got, UnsupportedConstruct) || strings.Contains(string(encoded), "outside-sentinel") {
		t.Fatalf("unsafe operand escaped: %s", encoded)
	}
}

func TestDependencyExtensionMissingTarget(t *testing.T) {
	repo := fstest.MapFS{"go.mod": &fstest.MapFile{Data: []byte("module example.com/control\n")}}
	src := []byte("package app\nimport _ \"example.org/lib\"\n")
	r, err := New(TrustUnmeasured("go"), WithResolver("go", resolverFunc(func(fs.FS, string, string) ([]Edge, *Unknown) {
		return []Edge{{Kind: Import, Resolution: File, To: "missing.go"}}, nil
	})))
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.ReadFile(context.Background(), repo, "main.go", "go", src)
	if err != nil || got.Complete || !hasUnknown(got, UnresolvedAlias) || hasEdge(got, Import, File, "missing.go") {
		t.Fatalf("missing extension target: %+v, %v", got, err)
	}
}

func TestGoDependencyEmbedWithMissingImport(t *testing.T) {
	repo := fstest.MapFS{
		"go.mod":      &fstest.MapFile{Data: []byte("module example.com/control\n")},
		"payload.txt": &fstest.MapFile{Data: []byte("data")},
	}
	r, err := New(TrustUnmeasured("go"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.ReadFile(context.Background(), repo, "main.go", "go", []byte(
		"package app\n//go:embed payload.txt\nvar payload string\n"))
	if err != nil || got.Complete || !hasUnknown(got, UnsupportedConstruct) {
		t.Fatalf("embed without required import: %+v, %v", got, err)
	}
}

func TestGoDependencyPackageEntryCap(t *testing.T) {
	repo := fstest.MapFS{"go.mod": &fstest.MapFile{Data: []byte("module example.com/control\n")}}
	for i := range maxPackageEntries {
		repo["p"+strconv.Itoa(i)+".go"] = &fstest.MapFile{Data: []byte("package app\n")}
	}
	r, err := New(TrustUnmeasured("go"))
	if err != nil {
		t.Fatal(err)
	}
	got, err := r.ReadFile(context.Background(), repo, "main.go", "go", []byte("package app\n"))
	if err != nil || got.Complete || !hasUnknown(got, ReadLimit) || len(got.Edges) != 1 {
		t.Fatalf("directory over entry cap leaked a partial member list: %+v, %v", got, err)
	}
}

package adoption

import (
	"encoding/json"
	"reflect"
	"runtime"
	"runtime/debug"
	"strings"
	"testing"
)

func TestBuildEvidenceAbsent(t *testing.T) {
	for _, info := range []*debug.BuildInfo{nil, {}} {
		got := buildEvidence(info, "go1.26.test", "fixture-os", "fixture-arch")
		want := BuildEvidence{GoVersion: "go1.26.test", GOOS: "fixture-os", GOARCH: "fixture-arch"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("absent metadata = %#v, want %#v", got, want)
		}
		body, err := json.Marshal(got)
		if err != nil {
			t.Fatal(err)
		}
		const wantJSON = `{"main_module":"","main_version":"","go_version":"go1.26.test","goos":"fixture-os","goarch":"fixture-arch","vcs_revision":"","vcs_modified":null}`
		if string(body) != wantJSON {
			t.Fatalf("absent metadata must retain explicit unknown fields: %s", body)
		}
	}
}

func TestBuildEvidenceVCSFlags(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value string
		known bool
		dirty bool
	}{
		{name: "dirty", value: "true", known: true, dirty: true},
		{name: "clean", value: "false", known: true},
		{name: "empty"},
		{name: "numeric", value: "1"},
		{name: "uppercase", value: "TRUE"},
		{name: "padded", value: "false "},
		{name: "invalid", value: "unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			info := &debug.BuildInfo{
				Main: debug.Module{Path: "example.test/checker", Version: "(devel)"},
				Settings: []debug.BuildSetting{
					{Key: "vcs.revision", Value: "0123456789abcdef"},
					{Key: "vcs.modified", Value: tc.value},
				},
			}
			got := buildEvidence(info, "go1.26.test", "linux", "arm64")
			if got.MainModule != "example.test/checker" || got.MainVersion != "(devel)" || got.VCSRevision != "0123456789abcdef" {
				t.Fatalf("main checker identity lost: %#v", got)
			}
			if (got.VCSModified != nil) != tc.known {
				t.Fatalf("modified known = %v, want %v", got.VCSModified != nil, tc.known)
			}
			if tc.known && *got.VCSModified != tc.dirty {
				t.Errorf("modified = %v, want %v", *got.VCSModified, tc.dirty)
			}
			if again := buildEvidence(info, "go1.26.test", "linux", "arm64"); !reflect.DeepEqual(got, again) {
				t.Fatal("identical inputs produced different evidence")
			}
		})
	}
}

func TestBuildEvidenceAllowlistDoesNotLeakBuildEnvironment(t *testing.T) {
	info := &debug.BuildInfo{
		GoVersion: "private-info-go-version",
		Path:      "/private/command/path",
		Main: debug.Module{
			Path: "example.test/checker", Version: "v1.2.3", Sum: "private-main-sum",
			Replace: &debug.Module{Path: "/private/main/replacement", Version: "private-replacement-version"},
		},
		Deps: []*debug.Module{{Path: "private-dependency", Version: "v9.9.9"}},
		Settings: []debug.BuildSetting{
			{Key: "vcs", Value: "private-vcs-type"},
			{Key: "vcs.time", Value: "private-build-time"},
			{Key: "GOOS", Value: "private-setting-os"},
			{Key: "GOARCH", Value: "private-setting-arch"},
			{Key: "CGO_CFLAGS", Value: "private-compiler-flags"},
			{Key: "-ldflags", Value: "private-linker-flags"},
			{Key: "PWD", Value: "/private/checkout"},
		},
	}
	got := buildEvidence(info, "go1.26.test", "linux", "amd64")
	want := BuildEvidence{MainModule: "example.test/checker", MainVersion: "v1.2.3", GoVersion: "go1.26.test", GOOS: "linux", GOARCH: "amd64"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("allowlisted metadata = %#v, want %#v", got, want)
	}
	body, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(body), "private") {
		t.Fatalf("nonallowlisted metadata leaked: %s", body)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		t.Fatal(err)
	}
	if len(fields) != 7 {
		t.Fatalf("build evidence exported unexpected fields: %s", body)
	}
}

func TestCurrentBuildUsesRunningChecker(t *testing.T) {
	info, _ := debug.ReadBuildInfo()
	want := buildEvidence(info, runtime.Version(), runtime.GOOS, runtime.GOARCH)
	if got := CurrentBuild(); !reflect.DeepEqual(got, want) {
		t.Fatalf("running checker metadata = %#v, want %#v", got, want)
	}
}

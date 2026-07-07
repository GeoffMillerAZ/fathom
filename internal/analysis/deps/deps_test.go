package deps_test

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/deps"
)

// -----------------------------------------------------------------
// ParseGoSum
// -----------------------------------------------------------------

const goSumFixture = `
github.com/pkg/errors v0.9.0 h1:FEBLx1zS214owpjy7qsBeixbURkuhQAwrK5UwLGTwt38=
github.com/pkg/errors v0.9.0/go.mod h1:bwawxfHBFNV+L2hUp1rHADufV3IMtnDRdf1r5NINEl0=
golang.org/x/text v0.3.7 h1:olpwvP2KacW1ZWvsR7uQhoyTYvKAupfQrRGBFM352Fd4=
golang.org/x/text v0.3.7/go.mod h1:bEr9sfX3Q8Zfm5fL9x+3itogRgK3+ptLWKqgva+5dAk=
`

func TestParseGoSum(t *testing.T) {
	got := deps.ParseGoSum([]byte(goSumFixture))
	if len(got) != 2 {
		t.Fatalf("expected 2 entries, got %d: %v", len(got), got)
	}
	// sorted: errors before text
	if got[0].Name != "github.com/pkg/errors" || got[0].Version != "v0.9.0" {
		t.Errorf("unexpected first entry: %+v", got[0])
	}
	if got[1].Name != "golang.org/x/text" || got[1].Version != "v0.3.7" {
		t.Errorf("unexpected second entry: %+v", got[1])
	}
	for _, d := range got {
		if d.Ecosystem != "go" {
			t.Errorf("expected ecosystem=go, got %q", d.Ecosystem)
		}
		if d.Direct {
			t.Errorf("go.sum entries should not be Direct")
		}
	}
}

func TestParseGoSumEmpty(t *testing.T) {
	got := deps.ParseGoSum([]byte(""))
	if got != nil && len(got) != 0 {
		t.Fatalf("expected empty slice, got %v", got)
	}
}

func TestParseGoSumDeterminism(t *testing.T) {
	a := deps.ParseGoSum([]byte(goSumFixture))
	b := deps.ParseGoSum([]byte(goSumFixture))
	if len(a) != len(b) {
		t.Fatal("non-deterministic length")
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("non-deterministic at %d: %+v vs %+v", i, a[i], b[i])
		}
	}
}

// -----------------------------------------------------------------
// ParsePackageLock
// -----------------------------------------------------------------

const packageLockFixture = `{
  "name": "myapp",
  "version": "1.0.0",
  "lockfileVersion": 3,
  "packages": {
    "": {
      "name": "myapp",
      "version": "1.0.0"
    },
    "node_modules/express": {
      "version": "4.18.2",
      "dev": false
    },
    "node_modules/lodash": {
      "version": "4.17.21"
    },
    "node_modules/express/node_modules/qs": {
      "version": "6.11.0"
    }
  }
}`

func TestParsePackageLock(t *testing.T) {
	got, err := deps.ParsePackageLock([]byte(packageLockFixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// expect 3 packages: express, lodash, qs (root "" is skipped)
	if len(got) != 3 {
		t.Fatalf("expected 3 entries, got %d: %v", len(got), got)
	}
	byName := make(map[string]deps.Dependency)
	for _, d := range got {
		byName[d.Name] = d
	}

	exp := map[string]struct {
		ver    string
		direct bool
	}{
		"express":                 {"4.18.2", true},
		"lodash":                  {"4.17.21", true},
		"express/node_modules/qs": {"6.11.0", false},
	}
	for name, want := range exp {
		d, ok := byName[name]
		if !ok {
			t.Errorf("missing %q", name)
			continue
		}
		if d.Version != want.ver {
			t.Errorf("%s: version=%q want %q", name, d.Version, want.ver)
		}
		if d.Direct != want.direct {
			t.Errorf("%s: direct=%v want %v", name, d.Direct, want.direct)
		}
		if d.Ecosystem != "npm" {
			t.Errorf("%s: ecosystem=%q want npm", name, d.Ecosystem)
		}
	}
}

func TestParsePackageLockInvalidJSON(t *testing.T) {
	_, err := deps.ParsePackageLock([]byte("{bad json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestParsePackageLockDeterminism(t *testing.T) {
	a, _ := deps.ParsePackageLock([]byte(packageLockFixture))
	b, _ := deps.ParsePackageLock([]byte(packageLockFixture))
	if len(a) != len(b) {
		t.Fatal("non-deterministic length")
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("non-deterministic at %d", i)
		}
	}
}

// -----------------------------------------------------------------
// ParseRequirementsTxt
// -----------------------------------------------------------------

const reqTxtFixture = `
# This is a comment
requests==2.28.1
flask>=2.0.0
numpy~=1.23.0
-r other.txt
boto3==1.26.0  # aws sdk
`

func TestParseRequirementsTxt(t *testing.T) {
	got := deps.ParseRequirementsTxt([]byte(reqTxtFixture))
	if len(got) != 4 {
		t.Fatalf("expected 4 entries, got %d: %v", len(got), got)
	}
	byName := make(map[string]deps.Dependency)
	for _, d := range got {
		byName[d.Name] = d
	}

	exp := map[string]string{
		"requests": "==2.28.1",
		"flask":    ">=2.0.0",
		"numpy":    "~=1.23.0",
		"boto3":    "==1.26.0",
	}
	for name, ver := range exp {
		d, ok := byName[name]
		if !ok {
			t.Errorf("missing %q", name)
			continue
		}
		if d.Version != ver {
			t.Errorf("%s: version=%q want %q", name, d.Version, ver)
		}
		if !d.Direct {
			t.Errorf("%s: expected Direct=true", name)
		}
		if d.Ecosystem != "pypi" {
			t.Errorf("%s: ecosystem=%q want pypi", name, d.Ecosystem)
		}
	}
}

func TestParseRequirementsTxtDeterminism(t *testing.T) {
	a := deps.ParseRequirementsTxt([]byte(reqTxtFixture))
	b := deps.ParseRequirementsTxt([]byte(reqTxtFixture))
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("non-deterministic at %d", i)
		}
	}
}

// -----------------------------------------------------------------
// ParseGemfileLock
// -----------------------------------------------------------------

const gemfileLockFixture = `GEM
  remote: https://rubygems.org/
  specs:
    rack (2.2.4)
    rails (7.0.4)
      rack (>= 2.2.4)
    zeitwerk (2.6.0)

PLATFORMS
  ruby

DEPENDENCIES
  rails (~> 7.0)
  zeitwerk

BUNDLED WITH
   2.3.26
`

func TestParseGemfileLock(t *testing.T) {
	got := deps.ParseGemfileLock([]byte(gemfileLockFixture))
	if len(got) != 3 {
		t.Fatalf("expected 3 entries, got %d: %v", len(got), got)
	}
	byName := make(map[string]deps.Dependency)
	for _, d := range got {
		byName[d.Name] = d
	}

	exp := map[string]struct {
		ver    string
		direct bool
	}{
		"rack":     {"2.2.4", false},
		"rails":    {"7.0.4", true},
		"zeitwerk": {"2.6.0", true},
	}
	for name, want := range exp {
		d, ok := byName[name]
		if !ok {
			t.Errorf("missing %q", name)
			continue
		}
		if d.Version != want.ver {
			t.Errorf("%s: version=%q want %q", name, d.Version, want.ver)
		}
		if d.Direct != want.direct {
			t.Errorf("%s: direct=%v want %v", name, d.Direct, want.direct)
		}
		if d.Ecosystem != "gem" {
			t.Errorf("%s: ecosystem=%q want gem", name, d.Ecosystem)
		}
	}
}

func TestParseGemfileLockDeterminism(t *testing.T) {
	a := deps.ParseGemfileLock([]byte(gemfileLockFixture))
	b := deps.ParseGemfileLock([]byte(gemfileLockFixture))
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("non-deterministic at %d", i)
		}
	}
}

// -----------------------------------------------------------------
// ParseCargoLock
// -----------------------------------------------------------------

const cargoLockFixture = `# This file is automatically @generated by Cargo.
# It is not intended for manual editing.
version = 3

[[package]]
name = "serde"
version = "1.0.152"
source = "registry+https://github.com/rust-lang/crates.io-index"

[[package]]
name = "tokio"
version = "1.25.0"
source = "registry+https://github.com/rust-lang/crates.io-index"
dependencies = [
 "serde",
]

[[package]]
name = "myapp"
version = "0.1.0"
`

func TestParseCargoLock(t *testing.T) {
	got := deps.ParseCargoLock([]byte(cargoLockFixture))
	if len(got) != 3 {
		t.Fatalf("expected 3 entries, got %d: %v", len(got), got)
	}
	byName := make(map[string]deps.Dependency)
	for _, d := range got {
		byName[d.Name] = d
	}

	exp := map[string]string{
		"serde": "1.0.152",
		"tokio": "1.25.0",
		"myapp": "0.1.0",
	}
	for name, ver := range exp {
		d, ok := byName[name]
		if !ok {
			t.Errorf("missing %q", name)
			continue
		}
		if d.Version != ver {
			t.Errorf("%s: version=%q want %q", name, d.Version, ver)
		}
		if d.Direct {
			t.Errorf("%s: Cargo.lock entries should not be Direct", name)
		}
		if d.Ecosystem != "cargo" {
			t.Errorf("%s: ecosystem=%q want cargo", name, d.Ecosystem)
		}
	}
}

func TestParseCargoLockDeterminism(t *testing.T) {
	a := deps.ParseCargoLock([]byte(cargoLockFixture))
	b := deps.ParseCargoLock([]byte(cargoLockFixture))
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("non-deterministic at %d", i)
		}
	}
}

// -----------------------------------------------------------------
// Detect
// -----------------------------------------------------------------

func TestDetect(t *testing.T) {
	tests := []struct {
		filename string
		content  []byte
		wantLen  int
		wantEco  string
		wantErr  bool
	}{
		{"go.sum", []byte(goSumFixture), 2, "go", false},
		{"package-lock.json", []byte(packageLockFixture), 3, "npm", false},
		{"requirements.txt", []byte(reqTxtFixture), 4, "pypi", false},
		{"Gemfile.lock", []byte(gemfileLockFixture), 3, "gem", false},
		{"Cargo.lock", []byte(cargoLockFixture), 3, "cargo", false},
		{"unknown.txt", []byte(""), 0, "", true},
	}
	for _, tc := range tests {
		got, err := deps.Detect(tc.filename, tc.content)
		if tc.wantErr {
			if err == nil {
				t.Errorf("%s: expected error", tc.filename)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: unexpected error: %v", tc.filename, err)
			continue
		}
		if len(got) != tc.wantLen {
			t.Errorf("%s: got %d entries, want %d", tc.filename, len(got), tc.wantLen)
		}
		for _, d := range got {
			if d.Ecosystem != tc.wantEco {
				t.Errorf("%s: ecosystem=%q want %q", tc.filename, d.Ecosystem, tc.wantEco)
			}
		}
	}
}

func TestDetectWithDirectory(t *testing.T) {
	// Detect should strip directory prefix and match on basename.
	got, err := deps.Detect("/path/to/project/go.sum", []byte(goSumFixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2, got %d", len(got))
	}
}

// -----------------------------------------------------------------
// Drift
// -----------------------------------------------------------------

func TestDrift(t *testing.T) {
	current := []deps.Dependency{
		{Name: "foo", Version: "2.0.0"},
		{Name: "bar", Version: "1.0.0"},
		{Name: "baz", Version: "3.0.0"},
	}
	baseline := []deps.Dependency{
		{Name: "foo", Version: "1.0.0"}, // drifted
		{Name: "bar", Version: "1.0.0"}, // same
		{Name: "qux", Version: "9.0.0"}, // only in baseline
	}
	drift := deps.Drift(current, baseline)
	if len(drift) != 1 {
		t.Fatalf("expected 1 drift item, got %d: %v", len(drift), drift)
	}
	if drift[0].Name != "foo" || drift[0].Current != "2.0.0" || drift[0].Baseline != "1.0.0" {
		t.Errorf("unexpected drift item: %+v", drift[0])
	}
}

func TestDriftEmpty(t *testing.T) {
	drift := deps.Drift(nil, nil)
	if len(drift) != 0 {
		t.Fatalf("expected 0 items, got %d", len(drift))
	}
}

func TestDriftDeterminism(t *testing.T) {
	current := []deps.Dependency{
		{Name: "b", Version: "2.0"},
		{Name: "a", Version: "2.0"},
	}
	baseline := []deps.Dependency{
		{Name: "b", Version: "1.0"},
		{Name: "a", Version: "1.0"},
	}
	a := deps.Drift(current, baseline)
	b := deps.Drift(current, baseline)
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("non-deterministic drift at %d", i)
		}
	}
	// Check sorted order
	if a[0].Name != "a" || a[1].Name != "b" {
		t.Errorf("drift not sorted by name: %v", a)
	}
}

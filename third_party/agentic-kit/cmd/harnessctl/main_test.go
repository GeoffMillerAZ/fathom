package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

var fixed = time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)

type fixture struct {
	reg                               registration
	layer                             layer
	private                           ed25519.PrivateKey
	registrationPath, layerPath, root string
}

func toy(t *testing.T) fixture {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(base, "project")
	if e := os.Mkdir(root, 0700); e != nil {
		t.Fatal(e)
	}
	seed := sha256.Sum256([]byte("harnessctl toy test key"))
	private := ed25519.NewKeyFromSeed(seed[:])
	pub := private.Public().(ed25519.PublicKey)
	ref := layerRef{ID: "install", Kind: "installation", TargetID: "host", Path: filepath.Join(base, "layer.json"), KeyID: "issuer", Revision: 1, MaxAgeSeconds: 7200, Priority: 0, ControlIDs: []string{"review"}}
	f := fixture{registrationPath: filepath.Join(base, "registration.json"), layerPath: ref.Path, root: root, private: private}
	f.layer = layer{Version: "harnessctl.layer/1", ID: "install", Kind: "installation", TargetID: "host", Revision: 1, IssuedAt: "2026-10-24T11:00:00Z", ExpiresAt: "2026-10-25T00:00:00Z", Controls: []constraint{{ID: "review", MinSamplePPM: 100000, MaxSamplePPM: 900000, MaxWorkMillis: 1000, OnFault: "hold", AllowMaintenance: false, TunableBy: []string{"install"}}}, Preferences: []preference{{ControlID: "review", SamplePPM: 700000}}}
	f.reg = registration{Version: "harnessctl.registration/1", Epoch: 1, IssuedAt: "2026-10-24T00:00:00Z", ExpiresAt: "2026-10-25T00:00:00Z", HostID: "host", Keys: []key{{ID: "issuer", PublicKey: base64.StdEncoding.EncodeToString(pub), Revoked: false}}, Controls: []control{{ID: "review", Event: "before-tool", Class: "advisory", MinSamplePPM: 0, MaxSamplePPM: 1000000, DefaultSamplePPM: 500000, MaxWorkMillis: 2000, OnFault: "warn", AllowMaintenance: false, TunableBy: []string{"install"}}}, Projects: []project{{ID: "project", Root: root, TeamIDs: []string{}, PrincipalID: "operator", ControlIDs: []string{"review"}, Layers: []layerRef{ref}}}}
	f.save(t)
	return f
}
func (f *fixture) save(t *testing.T) {
	t.Helper()
	payload, e := json.Marshal(f.layer)
	if e != nil {
		t.Fatal(e)
	}
	h := sha256.Sum256(payload)
	if len(f.reg.Projects[0].Layers) > 0 {
		f.reg.Projects[0].Layers[0].PayloadSHA256 = hex.EncodeToString(h[:])
	}
	pae := []byte("DSSEv1 " + strconv.Itoa(len(payloadType)) + " " + payloadType + " " + strconv.Itoa(len(payload)) + " ")
	pae = append(pae, payload...)
	sig := ed25519.Sign(f.private, pae)
	env := envelope{PayloadType: payloadType, Payload: base64.StdEncoding.EncodeToString(payload), Signatures: []signature{{Sig: base64.StdEncoding.EncodeToString(sig)}}}
	b, _ := json.Marshal(env)
	if e = os.WriteFile(f.layerPath, b, 0600); e != nil {
		t.Fatal(e)
	}
	b, _ = json.Marshal(f.reg)
	if e = os.WriteFile(f.registrationPath, b, 0600); e != nil {
		t.Fatal(e)
	}
}
func invoke(f fixture, explain bool, s seams) (report, int) {
	return check(f.registrationPath, f.root, explain, s)
}
func fixedSeams() seams { return seams{now: func() time.Time { return fixed }, read: readSnapshot} }
func TestToyPositive(t *testing.T) {
	f := toy(t)
	r, code := invoke(f, true, fixedSeams())
	if code != 0 || r.Status != "authenticated" || len(r.EffectiveControls) != 1 || r.EffectiveControls[0].SamplePPM != 700000 || r.EffectiveControls[0].TunedBy != "install" {
		t.Fatalf("%+v %d", r, code)
	}
	checkR, code := invoke(f, false, fixedSeams())
	if code != 0 || checkR.SnapshotSHA256 != r.SnapshotSHA256 || len(checkR.EffectiveControls) != 0 {
		t.Fatal(checkR, code)
	}
	var out bytes.Buffer
	if code = run([]string{"config", "explain", "--registration", f.registrationPath, "--project", f.root, "--format", "json"}, &out, fixedSeams()); code != 0 {
		t.Fatal(code, out.String())
	}
	var decoded map[string]any
	if json.Unmarshal(out.Bytes(), &decoded) != nil {
		t.Fatal(out.String())
	}
}

func TestToyTempAlias(t *testing.T) {
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	actual, alias := filepath.Join(base, "actual"), filepath.Join(base, "alias")
	if err := os.Mkdir(actual, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(actual, alias); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", alias)
	// A new T gives TempDir a fresh parent after selecting the aliased TMPDIR.
	t.Run("fixture", func(t *testing.T) {
		f := toy(t)
		if real, err := filepath.EvalSymlinks(f.root); err != nil || real != f.root {
			t.Fatalf("fixture root must be canonical: root=%q real=%q err=%v", f.root, real, err)
		}
		if r, code := invoke(f, true, fixedSeams()); code != 0 || r.Status != "authenticated" {
			t.Fatalf("aliased temp parent changed fixture behavior: %d %+v", code, r)
		}
	})
}

func TestHelpDoesNotReadConfiguration(t *testing.T) {
	s := seams{now: func() time.Time { t.Fatal("help read clock"); return time.Time{} }, read: func(string, int) (snapshot, error) { t.Fatal("help read configuration"); return snapshot{}, errSource }}
	for _, args := range [][]string{{"help"}, {"--help"}, {"-h"}, {"config", "--help"}, {"config", "check", "--help"}, {"config", "explain", "-h"}} {
		var out bytes.Buffer
		if run(args, &out, s) != 0 || !strings.Contains(out.String(), "config check") || !strings.Contains(out.String(), "config explain") {
			t.Fatalf("help %v: %q", args, out.String())
		}
	}
}

func TestUnselectedRegistrationStillRequiresValidShape(t *testing.T) {
	for _, badKind := range []bool{false, true} {
		f := toy(t)
		other := f.reg.Projects[0]
		other.ID = "other"
		other.Root = f.root + "-unselected"
		other.Layers = append([]layerRef(nil), other.Layers...)
		if badKind {
			other.Layers[0].Kind = "unknown-kind"
		} else {
			empty := ""
			other.OrganizationID = &empty
		}
		f.reg.Projects = append(f.reg.Projects, other)
		f.save(t)
		r, code := invoke(f, true, fixedSeams())
		if code != 2 || r.Code != "registration_invalid" || r.ProjectID != "" || r.SnapshotSHA256 != "" || len(r.Sources) != 0 {
			t.Fatalf("accepted invalid unselected shape: code=%d %+v", code, r)
		}
	}
}
func TestSourceFailures(t *testing.T) {
	cases := []struct {
		name   string
		change func(*fixture)
	}{
		{"revoked", func(f *fixture) { f.reg.Keys[0].Revoked = true }},
		{"wrong key", func(f *fixture) { f.reg.Keys[0].PublicKey = base64.StdEncoding.EncodeToString(make([]byte, 32)) }},
		{"target", func(f *fixture) { f.layer.TargetID = "other" }},
		{"revision", func(f *fixture) { f.layer.Revision = 2 }},
		{"missing source", func(f *fixture) { f.reg.Projects[0].Layers = nil }},
		{"foreign control", func(f *fixture) { f.reg.Projects[0].Layers[0].ControlIDs = []string{"foreign"} }},
		{"expired", func(f *fixture) { f.reg.ExpiresAt = "2026-10-24T12:00:00Z" }},
		{"mandatory weakening", func(f *fixture) { f.reg.Controls[0].Class = "mandatory" }},
		{"hidden preference", func(f *fixture) { f.layer.Controls[0].TunableBy = []string{} }},
		{"range conflict", func(f *fixture) { f.layer.Controls[0].MinSamplePPM = 900001 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := toy(t)
			tc.change(&f)
			f.save(t)
			r, code := invoke(f, true, fixedSeams())
			if code == 0 || r.ProjectID != "" || len(r.EffectiveControls) != 0 || r.Category == "" {
				t.Fatalf("%d %+v", code, r)
			}
		})
	}
}
func TestWireAdversarial(t *testing.T) {
	var k key
	for _, raw := range []string{`{"id":"x","id":"y","publicKey":"x","revoked":false}`, `{"Id":"x","publicKey":"x","revoked":false}`, `{"id":"x","publicKey":null,"revoked":false}`, `{"id":"\ud800","publicKey":"x","revoked":false}`, `{"id":"x","publicKey":"x","revoked":false} true`, `{"id":"x","publicKey":"x","revoked":false,"extra":1}`} {
		if strictJSON([]byte(raw), &k) == nil {
			t.Fatal(raw)
		}
	}
	deep := strings.Repeat("[", 25) + "0" + strings.Repeat("]", 25)
	if strictJSON([]byte(deep), &k) == nil {
		t.Fatal("depth")
	}
}
func TestDriftAndOutput(t *testing.T) {
	f := toy(t)
	s := fixedSeams()
	calls := 0
	s.read = func(path string, limit int) (snapshot, error) {
		snap, e := readSnapshot(path, limit)
		calls++
		if calls == 3 && e == nil {
			snap.data = append([]byte{}, snap.data...)
			snap.data[0] = '!'
		}
		return snap, e
	}
	if _, code := invoke(f, true, s); code != 2 {
		t.Fatal(code)
	}
	s = fixedSeams()
	ticks := 0
	s.now = func() time.Time {
		ticks++
		if ticks == 1 {
			return fixed
		}
		return fixed.Add(-time.Second)
	}
	if _, code := invoke(f, false, s); code != 2 {
		t.Fatal(code)
	}
	if code := run([]string{"config", "check", "--registration", f.registrationPath, "--project", f.root}, shortWriter{}, fixedSeams()); code != 2 {
		t.Fatal(code)
	}
}

type shortWriter struct{}

func (shortWriter) Write(b []byte) (int, error) { return 0, io.ErrShortWrite }
func TestSymlink(t *testing.T) {
	f := toy(t)
	link := filepath.Join(filepath.Dir(f.layerPath), "link")
	if e := os.Symlink(f.layerPath, link); e != nil {
		t.Fatal(e)
	}
	f.reg.Projects[0].Layers[0].Path = link
	b, _ := json.Marshal(f.reg)
	if e := os.WriteFile(f.registrationPath, b, 0600); e != nil {
		t.Fatal(e)
	}
	if _, code := invoke(f, false, fixedSeams()); code != 2 {
		t.Fatal(code)
	}
}

func TestEnvelopeProfile(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*envelope)
	}{
		{"hint ignored", func(e *envelope) { e.Signatures[0].KeyID = "someone-else" }},
		{"raw URL base64", func(e *envelope) {
			p, _ := base64.StdEncoding.DecodeString(e.Payload)
			e.Payload = base64.RawURLEncoding.EncodeToString(p)
			s, _ := base64.StdEncoding.DecodeString(e.Signatures[0].Sig)
			e.Signatures[0].Sig = base64.RawURLEncoding.EncodeToString(s)
		}},
		{"wrong signature", func(e *envelope) { e.Signatures[0].Sig = base64.StdEncoding.EncodeToString(make([]byte, 64)) }},
		{"wrong type", func(e *envelope) { e.PayloadType = "application/json" }},
		{"multisig", func(e *envelope) { e.Signatures = append(e.Signatures, e.Signatures[0]) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := toy(t)
			b, e := os.ReadFile(f.layerPath)
			if e != nil {
				t.Fatal(e)
			}
			var env envelope
			if json.Unmarshal(b, &env) != nil {
				t.Fatal("fixture")
			}
			tc.edit(&env)
			b, _ = json.Marshal(env)
			if e = os.WriteFile(f.layerPath, b, 0600); e != nil {
				t.Fatal(e)
			}
			_, code := invoke(f, false, fixedSeams())
			want := 2
			if tc.name == "hint ignored" || tc.name == "raw URL base64" {
				want = 0
			}
			if code != want {
				t.Fatalf("code %d want %d", code, want)
			}
		})
	}
}
func TestDigestAndSharedSource(t *testing.T) {
	f := toy(t)
	first, code := invoke(f, false, fixedSeams())
	if code != 0 {
		t.Fatal(code)
	}
	root2 := filepath.Join(filepath.Dir(f.root), "other")
	if e := os.Mkdir(root2, 0700); e != nil {
		t.Fatal(e)
	}
	p2 := f.reg.Projects[0]
	p2.ID = "other"
	p2.Root = root2
	f.reg.Projects = append(f.reg.Projects, p2)
	f.save(t)
	r, code := invoke(f, false, fixedSeams())
	if code != 0 || r.SnapshotSHA256 == first.SnapshotSHA256 {
		t.Fatal(code, r)
	}
	r2, code := check(f.registrationPath, root2, false, fixedSeams())
	if code != 0 || r2.SnapshotSHA256 == r.SnapshotSHA256 {
		t.Fatal(code, r2)
	}
	f.reg.Projects[0].Layers[0].PayloadSHA256 = strings.Repeat("0", 64)
	b, _ := json.Marshal(f.reg)
	if e := os.WriteFile(f.registrationPath, b, 0600); e != nil {
		t.Fatal(e)
	}
	if _, code := invoke(f, false, fixedSeams()); code != 2 {
		t.Fatal(code)
	}
}
func TestLimitsAndClock(t *testing.T) {
	f := toy(t)
	big := bytes.Repeat([]byte{' '}, (2<<20)+1)
	if e := os.WriteFile(f.registrationPath, big, 0600); e != nil {
		t.Fatal(e)
	}
	if _, code := invoke(f, false, fixedSeams()); code != 2 {
		t.Fatal(code)
	}
	f.save(t)
	s := fixedSeams()
	s.now = func() time.Time { return fixed.Add(24 * time.Hour) }
	if _, code := invoke(f, false, s); code != 2 {
		t.Fatal(code)
	}
}

func TestFailureCodesAndCounts(t *testing.T) {
	f := toy(t)
	s := fixedSeams()
	calls := 0
	s.read = func(path string, limit int) (snapshot, error) {
		snap, e := readSnapshot(path, limit)
		calls++
		if calls == 3 && e == nil {
			snap.data = append([]byte{}, snap.data...)
			snap.data[0] = '!'
		}
		return snap, e
	}
	r, code := invoke(f, true, s)
	if code != 2 || r.Code != "source_changed" || r.ExpectedLayers == nil || *r.ExpectedLayers != 1 || *r.LoadedLayers != 1 || *r.AuthenticatedLayers != 1 || r.SourceOrdinal == nil || *r.SourceOrdinal != 0 || r.ProjectID != "" || r.RegistrationSHA256 != "" || len(r.EffectiveControls) != 0 {
		t.Fatalf("late drift: %d %+v", code, r)
	}
	var out bytes.Buffer
	calls = 0
	if code = run([]string{"config", "explain", "--registration", f.registrationPath, "--project", f.root}, &out, s); code != 2 || strings.Contains(out.String(), "authenticated\"") || strings.Contains(out.String(), "projectId") || strings.Contains(out.String(), "SHA256") {
		t.Fatalf("prefix/leak: %d %s", code, out.String())
	}
	if e := os.Remove(f.layerPath); e != nil {
		t.Fatal(e)
	}
	r, code = invoke(f, false, fixedSeams())
	if code != 2 || r.Code != "source_unavailable" || *r.ExpectedLayers != 1 || *r.LoadedLayers != 0 || *r.AuthenticatedLayers != 0 {
		t.Fatalf("missing selected path: %d %+v", code, r)
	}
	f = toy(t)
	f.layer.Preferences[0].SamplePPM = 950000
	f.save(t)
	r, code = invoke(f, false, fixedSeams())
	if code != 1 || r.Code != "preference_denied" || *r.AuthenticatedLayers != 1 {
		t.Fatalf("preference: %d %+v", code, r)
	}
}
func TestCaseAliasProtection(t *testing.T) {
	f := toy(t)
	base := filepath.Dir(f.root)
	alias := filepath.Join(base, "PROJECT")
	if _, e := os.Stat(alias); e != nil {
		t.Skip("case-sensitive filesystem")
	}
	if e := os.Mkdir(filepath.Join(f.root, "controls"), 0700); e != nil {
		t.Fatal(e)
	}
	inside := filepath.Join(alias, "controls", "registration.json")
	b, _ := json.Marshal(f.reg)
	if e := os.WriteFile(inside, b, 0600); e != nil {
		t.Fatal(e)
	}
	if _, code := check(inside, f.root, false, fixedSeams()); code != 2 {
		t.Fatal("candidate-owned registration accepted")
	}
	p := f.reg.Projects[0]
	p.ID = "alias"
	p.Root = alias
	f.reg.Projects = append(f.reg.Projects, p)
	b, _ = json.Marshal(f.reg)
	if e := os.WriteFile(f.registrationPath, b, 0600); e != nil {
		t.Fatal(e)
	}
	r, code := invoke(f, false, fixedSeams())
	if code != 2 || r.Code != "registration_invalid" {
		t.Fatalf("alias root: %d %+v", code, r)
	}
}

func TestIdentityContainmentAndGhostTuner(t *testing.T) {
	f := toy(t)
	inside := filepath.Join(f.root, "nested")
	if e := os.Mkdir(inside, 0700); e != nil {
		t.Fatal(e)
	}
	file := filepath.Join(inside, "control.json")
	if e := os.WriteFile(file, []byte("{}"), 0600); e != nil {
		t.Fatal(e)
	}
	if !within(file, f.root) || within(f.registrationPath, f.root) {
		t.Fatal("filesystem containment")
	}
	f.layer.Controls[0].TunableBy = []string{"ghost"}
	f.save(t)
	r, code := invoke(f, false, fixedSeams())
	if code != 2 || r.Code != "binding_mismatch" || *r.AuthenticatedLayers != 1 || r.SourceOrdinal == nil || *r.SourceOrdinal != 0 {
		t.Fatalf("ghost tuner: %d %+v", code, r)
	}
}

func TestLateFailureScrubsSelectedMetadata(t *testing.T) {
	f := toy(t)
	f.reg.Projects[0].ID = "private-project"
	f.reg.Projects[0].Layers[0].ID = "private-layer"
	f.layer.ID = "private-layer"
	f.reg.Controls[0].TunableBy = []string{"private-layer"}
	f.layer.Controls[0].TunableBy = []string{"private-layer"}
	f.reg.Keys[0].ID = "private-key"
	f.reg.Projects[0].Layers[0].KeyID = "private-key"
	f.save(t)
	raw, e := os.ReadFile(f.layerPath)
	if e != nil {
		t.Fatal(e)
	}
	var env envelope
	if json.Unmarshal(raw, &env) != nil {
		t.Fatal("envelope")
	}
	env.Signatures[0].KeyID = "private-hint"
	raw, _ = json.Marshal(env)
	if e = os.WriteFile(f.layerPath, raw, 0600); e != nil {
		t.Fatal(e)
	}
	calls := 0
	s := fixedSeams()
	s.read = func(path string, limit int) (snapshot, error) {
		snap, e := readSnapshot(path, limit)
		calls++
		if calls == 3 && e == nil {
			snap.data = append([]byte{}, snap.data...)
			snap.data[0] = '!'
		}
		return snap, e
	}
	var out bytes.Buffer
	code := run([]string{"config", "explain", "--registration", f.registrationPath, "--project", f.root}, &out, s)
	if code != 2 {
		t.Fatal(code)
	}
	var r report
	if json.Unmarshal(out.Bytes(), &r) != nil || r.Code != "source_changed" || *r.ExpectedLayers != 1 || *r.LoadedLayers != 1 || *r.AuthenticatedLayers != 1 || r.SourceOrdinal == nil || *r.SourceOrdinal != 0 {
		t.Fatalf("late receipt: %s", out.String())
	}
	for _, secret := range []string{"private-project", "private-layer", "private-key", "private-hint", f.layerPath, f.registrationPath, f.reg.Projects[0].Layers[0].PayloadSHA256} {
		if strings.Contains(out.String(), secret) {
			t.Fatalf("leaked selected metadata: %q", secret)
		}
	}
}

func TestMandatoryConstraintAndAgeBoundary(t *testing.T) {
	f := toy(t)
	root := &f.reg.Controls[0]
	root.Class = "mandatory"
	root.MinSamplePPM = 1000000
	root.MaxSamplePPM = 1000000
	root.DefaultSamplePPM = 1000000
	root.OnFault = "hold"
	root.TunableBy = []string{}
	f.layer.Preferences = []preference{}
	f.save(t)
	r, code := invoke(f, false, fixedSeams())
	if code != 1 || r.Code != "policy_conflict" {
		t.Fatalf("mandatory weakening: %d %+v", code, r)
	}
	f = toy(t)
	f.reg.Projects[0].Layers[0].MaxAgeSeconds = 3600
	b, _ := json.Marshal(f.reg)
	if e := os.WriteFile(f.registrationPath, b, 0600); e != nil {
		t.Fatal(e)
	}
	r, code = invoke(f, false, fixedSeams())
	if code != 2 || r.Code != "clock_invalid" || *r.LoadedLayers != 1 {
		t.Fatalf("max age exact boundary: %d %+v", code, r)
	}
}

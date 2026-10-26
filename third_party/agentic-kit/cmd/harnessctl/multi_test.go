package main

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func signedLayer(t *testing.T, private ed25519.PrivateKey, path string, l layer) string {
	t.Helper()
	payload, e := json.Marshal(l)
	if e != nil {
		t.Fatal(e)
	}
	h := sha256.Sum256(payload)
	pae := []byte("DSSEv1 " + strconv.Itoa(len(payloadType)) + " " + payloadType + " " + strconv.Itoa(len(payload)) + " ")
	pae = append(pae, payload...)
	sig := ed25519.Sign(private, pae)
	env := envelope{PayloadType: payloadType, Payload: base64.StdEncoding.EncodeToString(payload), Signatures: []signature{{Sig: base64.StdEncoding.EncodeToString(sig)}}}
	b, e := json.Marshal(env)
	if e != nil || os.WriteFile(path, b, 0600) != nil {
		t.Fatal("write signed layer")
	}
	return hex.EncodeToString(h[:])
}
func TestGlobalCatalogWithProjectLocalTuners(t *testing.T) {
	f := toy(t)
	base := filepath.Dir(f.root)
	other := filepath.Join(base, "other")
	if e := os.Mkdir(other, 0700); e != nil {
		t.Fatal(e)
	}
	f.reg.Controls[0].TunableBy = []string{"project", "personal"}
	orgID := "org"
	f.reg.Projects[0].OrganizationID = &orgID
	f.reg.Projects[0].TeamIDs = []string{"team"}
	f.reg.Projects[0].PrincipalID = "person"
	shared := []struct{ id, kind, target string }{{"install", "installation", "host"}, {"org", "organization", "org"}, {"team", "team", "team"}, {"personal", "personal", "person"}}
	refs := make([]layerRef, 0, 5)
	for i, s := range shared {
		path := filepath.Join(base, s.id+".json")
		l := f.layer
		l.ID = s.id
		l.Kind = s.kind
		l.TargetID = s.target
		l.Controls = []constraint{f.layer.Controls[0]}
		l.Controls[0].TunableBy = []string{"project", "personal"}
		l.Preferences = []preference{}
		hash := signedLayer(t, f.private, path, l)
		refs = append(refs, layerRef{ID: s.id, Kind: s.kind, TargetID: s.target, Path: path, KeyID: "issuer", Revision: 1, PayloadSHA256: hash, MaxAgeSeconds: 7200, Priority: i, ControlIDs: []string{"review"}})
	}
	projectRef := func(id, path string, priority int, tuner string, prefs []preference) layerRef {
		if prefs == nil {
			prefs = []preference{}
		}
		l := f.layer
		l.ID = "project"
		l.Kind = "project"
		l.TargetID = id
		l.Controls = []constraint{f.layer.Controls[0]}
		l.Controls[0].TunableBy = []string{tuner}
		l.Preferences = prefs
		hash := signedLayer(t, f.private, path, l)
		return layerRef{ID: "project", Kind: "project", TargetID: id, Path: path, KeyID: "issuer", Revision: 1, PayloadSHA256: hash, MaxAgeSeconds: 7200, Priority: priority, ControlIDs: []string{"review"}}
	}
	a := f.reg.Projects[0]
	a.ID = "a"
	a.Layers = append(append([]layerRef{}, refs...), projectRef("a", filepath.Join(base, "a.json"), 4, "project", []preference{{ControlID: "review", SamplePPM: 600000}}))
	b := a
	b.ID = "b"
	b.Root = other
	b.Layers = append(append([]layerRef{}, refs...), projectRef("b", filepath.Join(base, "b.json"), 4, "personal", nil))
	f.reg.Projects = []project{a, b}
	regBytes, _ := json.Marshal(f.reg)
	if e := os.WriteFile(f.registrationPath, regBytes, 0600); e != nil {
		t.Fatal(e)
	}
	ra, code := check(f.registrationPath, a.Root, true, fixedSeams())
	if code != 0 || ra.EffectiveControls[0].SamplePPM != 600000 || len(ra.EffectiveControls[0].TunableBy) != 1 || ra.EffectiveControls[0].TunableBy[0] != "project" {
		t.Fatalf("a: %d %+v", code, ra)
	}
	rb, code := check(f.registrationPath, b.Root, true, fixedSeams())
	if code != 0 || rb.EffectiveControls[0].SamplePPM != 500000 || len(rb.EffectiveControls[0].TunableBy) != 1 || rb.EffectiveControls[0].TunableBy[0] != "personal" {
		t.Fatalf("b: %d %+v", code, rb)
	}
	// A preference from the shared personal source must hold A, even though
	// B delegates to personal and A's project source has higher priority.
	l := f.layer
	l.ID = "personal"
	l.Kind = "personal"
	l.TargetID = "person"
	l.Controls = []constraint{f.layer.Controls[0]}
	l.Controls[0].TunableBy = []string{"project", "personal"}
	l.Preferences = []preference{{ControlID: "review", SamplePPM: 700000}}
	hash := signedLayer(t, f.private, refs[3].Path, l)
	for i := range f.reg.Projects {
		f.reg.Projects[i].Layers[3].PayloadSHA256 = hash
	}
	regBytes, _ = json.Marshal(f.reg)
	if e := os.WriteFile(f.registrationPath, regBytes, 0600); e != nil {
		t.Fatal(e)
	}
	if _, code = check(f.registrationPath, a.Root, false, fixedSeams()); code != 1 {
		t.Fatalf("hidden losing preference: %d", code)
	}
	if rb, code = check(f.registrationPath, b.Root, true, fixedSeams()); code != 0 || rb.EffectiveControls[0].SamplePPM != 700000 {
		t.Fatalf("b delegated: %d %+v", code, rb)
	}
}

func TestSignedInProjectLayerPlacement(t *testing.T) {
	f := toy(t)
	dir := filepath.Join(f.root, ".agentic")
	if e := os.Mkdir(dir, 0700); e != nil {
		t.Fatal(e)
	}
	f.layerPath = filepath.Join(dir, "layer.json")
	f.reg.Projects[0].Layers[0].Path = f.layerPath
	f.save(t)
	if r, code := invoke(f, true, fixedSeams()); code != 0 || r.Status != "authenticated" {
		t.Fatalf("signed in-project layer: %d %+v", code, r)
	}
	regBytes, _ := os.ReadFile(f.registrationPath)
	candidateReg := filepath.Join(dir, "registration.json")
	if e := os.WriteFile(candidateReg, regBytes, 0600); e != nil {
		t.Fatal(e)
	}
	if r, code := check(candidateReg, f.root, false, fixedSeams()); code != 2 || r.Code != "registration_invalid" {
		t.Fatalf("candidate-owned registration: %d %+v", code, r)
	}
	if e := os.WriteFile(f.layerPath, []byte("{}"), 0600); e != nil {
		t.Fatal(e)
	}
	if r, code := invoke(f, false, fixedSeams()); code != 2 || r.Code != "envelope_invalid" {
		t.Fatalf("tamper: %d %+v", code, r)
	}
	f.save(t)
	f.reg.Projects[0].Layers[0].PayloadSHA256 = strings.Repeat("0", 64)
	regBytes, _ = json.Marshal(f.reg)
	if e := os.WriteFile(f.registrationPath, regBytes, 0600); e != nil {
		t.Fatal(e)
	}
	if r, code := invoke(f, false, fixedSeams()); code != 2 || r.Code != "binding_mismatch" {
		t.Fatalf("pin mismatch: %d %+v", code, r)
	}
	f.save(t)
	f.reg.Keys[0].PublicKey = base64.StdEncoding.EncodeToString(make([]byte, 32))
	regBytes, _ = json.Marshal(f.reg)
	if e := os.WriteFile(f.registrationPath, regBytes, 0600); e != nil {
		t.Fatal(e)
	}
	if r, code := invoke(f, false, fixedSeams()); code != 2 || r.Code != "signature_invalid" {
		t.Fatalf("wrong key: %d %+v", code, r)
	}
	f.reg.Keys[0].PublicKey = base64.StdEncoding.EncodeToString(f.private.Public().(ed25519.PublicKey))
	f.save(t)
	real := f.layerPath + ".real"
	if e := os.Rename(f.layerPath, real); e != nil {
		t.Fatal(e)
	}
	if e := os.Symlink(real, f.layerPath); e != nil {
		t.Fatal(e)
	}
	if r, code := invoke(f, false, fixedSeams()); code != 2 || r.Code != "source_unavailable" {
		t.Fatalf("symlink: %d %+v", code, r)
	}
}

func TestSignedInProjectOrganizationLayer(t *testing.T) {
	f := toy(t)
	dir := filepath.Join(f.root, ".agentic")
	if e := os.Mkdir(dir, 0700); e != nil {
		t.Fatal(e)
	}
	orgID := "org"
	f.reg.Projects[0].OrganizationID = &orgID
	l := f.layer
	l.ID = "org"
	l.Kind = "organization"
	l.TargetID = "org"
	l.Preferences = []preference{}
	path := filepath.Join(dir, "org.json")
	hash := signedLayer(t, f.private, path, l)
	ref := layerRef{ID: "org", Kind: "organization", TargetID: "org", Path: path, KeyID: "issuer", Revision: 1, PayloadSHA256: hash, MaxAgeSeconds: 7200, Priority: 1, ControlIDs: []string{"review"}}
	f.reg.Projects[0].Layers = append(f.reg.Projects[0].Layers, ref)
	b, _ := json.Marshal(f.reg)
	if e := os.WriteFile(f.registrationPath, b, 0600); e != nil {
		t.Fatal(e)
	}
	if r, code := invoke(f, true, fixedSeams()); code != 0 || r.LayerCount != 2 || r.EffectiveControls[0].SamplePPM != 700000 {
		t.Fatalf("signed in-project org layer: %d %+v", code, r)
	}
}

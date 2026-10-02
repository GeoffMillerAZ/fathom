package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

// Flux's own API types, strict: each document decodes into its kind's type
// or the file fails (an unknown field, an unknown kind of a Flux group).
func TestTypedObjects(t *testing.T) {
	src := []byte(`apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: repo
  namespace: flux-system
spec:
  interval: 1m
  url: https://example.com/repo.git
---
apiVersion: helm.toolkit.fluxcd.io/v2
kind: HelmRelease
metadata:
  name: app
spec:
  interval: 1m
  chartRef:
    kind: OCIRepository
    name: app
`)
	got, err := typedObjects(src)
	if err != nil {
		t.Fatal(err)
	}
	want := []fobj{{"GitRepository", "flux-system", "repo"}, {"HelmRelease", "", "app"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("typedObjects = %+v, want %+v", got, want)
	}
	if _, err := typedObjects([]byte("apiVersion: kustomize.toolkit.fluxcd.io/v1\nkind: Kustomization\nmetadata:\n  name: k\nspec:\n  interval: 1m\n  prune: true\n  bogus: 1\n")); err == nil {
		t.Fatal("an unknown field must fail the strict decode")
	}
	if _, err := typedObjects([]byte("apiVersion: image.toolkit.fluxcd.io/v1\nkind: ImagePolicy\nmetadata:\n  name: p\n")); err == nil {
		t.Fatal("a kind the oracle carries no type for must fail, never pass")
	}
}

// flux build kustomization (offline, --dry-run, an empty scratch path) loads
// a Kustomization by name and namespace from its file: the CLI's own reading
// of the object.
func TestFluxBuildFinds(t *testing.T) {
	bin, err := exec.LookPath("flux")
	if err != nil {
		t.Skip("flux not on PATH")
	}
	dir := t.TempDir()
	f := filepath.Join(dir, "ks.yaml")
	if err := os.WriteFile(f, []byte("apiVersion: kustomize.toolkit.fluxcd.io/v1\nkind: Kustomization\nmetadata:\n  name: apps\n  namespace: flux-system\nspec:\n  interval: 1m\n  prune: true\n  path: ./apps\n  sourceRef:\n    kind: GitRepository\n    name: repo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	empty, err := emptyKustomizeDir(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := fluxBuild(bin, f, "apps", "flux-system", empty); err != nil {
		t.Fatalf("flux build of a present Kustomization: %v", err)
	}
	if err := fluxBuild(bin, f, "absent", "flux-system", empty); err == nil {
		t.Fatal("flux build of an absent Kustomization must fail")
	}
}

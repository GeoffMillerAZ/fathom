package fusion_test

// epoch_test.go — regression for dogfood O9: extractor-semantics changes must
// invalidate stored fingerprints, or hash-gated files keep observations
// produced by the OLD extractor forever (the H1 callgraph fixes were
// invisible on the fleet store until forced). The fingerprint is therefore
// salted with fusion.ExtractorEpoch.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/store"
)

func TestIngest_ExtractorEpochInvalidatesOldFingerprints(t *testing.T) {
	conn := connector.NewFauxConnector()
	st := store.NewMemoryStore()
	p := fusion.New(conn, st, defaultCfg())
	ctx := context.Background()

	rep1, err := p.Ingest(ctx, paymentSvc)
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if rep1.Skipped != 0 {
		t.Fatalf("first ingest skipped %d files, want 0", rep1.Skipped)
	}

	refs, err := conn.Discover(ctx, paymentSvc)
	if err != nil || len(refs) == 0 {
		t.Fatalf("Discover: %v (%d refs)", err, len(refs))
	}
	ref := refs[0]
	content, err := conn.Fetch(ctx, ref)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	plain := sha256.Sum256(content)
	plainHex := hex.EncodeToString(plain[:])

	// The stored fingerprint must NOT be the plain content hash — the epoch
	// salt is load-bearing.
	stored, found, err := st.Fingerprint(ctx, paymentSvc.ID, ref.Path)
	if err != nil || !found {
		t.Fatalf("Fingerprint after ingest: found=%v err=%v", found, err)
	}
	if stored == plainHex {
		t.Fatal("stored fingerprint equals the unsalted content hash — ExtractorEpoch is not in the fingerprint")
	}

	// Salted fingerprints are stable: a re-ingest skips everything.
	rep2, err := p.Ingest(ctx, paymentSvc)
	if err != nil {
		t.Fatalf("re-Ingest: %v", err)
	}
	if rep2.Skipped == 0 {
		t.Fatal("re-ingest skipped nothing — salted fingerprint is not stable")
	}

	// Simulate a store written before the epoch salt existed (or under an
	// older epoch): the plain content hash. The next ingest must RE-EXTRACT
	// that file rather than skip it.
	if err := st.SetFingerprint(ctx, paymentSvc.ID, ref.Path, plainHex); err != nil {
		t.Fatalf("SetFingerprint: %v", err)
	}
	rep3, err := p.Ingest(ctx, paymentSvc)
	if err != nil {
		t.Fatalf("post-downgrade Ingest: %v", err)
	}
	if rep3.Skipped != rep2.Skipped-1 {
		t.Errorf("pre-epoch fingerprint was not invalidated: skipped %d, want %d", rep3.Skipped, rep2.Skipped-1)
	}
	after, found, err := st.Fingerprint(ctx, paymentSvc.ID, ref.Path)
	if err != nil || !found {
		t.Fatalf("Fingerprint after re-extract: found=%v err=%v", found, err)
	}
	if after != stored {
		t.Errorf("re-extracted fingerprint %q, want the salted value %q (deterministic)", after, stored)
	}
}

// TestIngest_ExtractorEpoch_CUEDepthChangeForcesReExtraction is LT-5: the CUE
// extractor v2 (nested-field recursion, docs/plans/lang-truth.md) changed
// extraction semantics for every already-indexed .cue file, so
// ExtractorEpoch was bumped "2"→"3". The seeded fingerprint below is salted
// with epoch "2" — the TRUE prior epoch — deliberately: it pins the bump
// itself, so a merge-conflict revert of ExtractorEpoch back to "2" makes the
// seeded fingerprint match again, the file is skipped, and this test fails
// loudly. (Seeding an older epoch like "1" would mismatch under both "2" and
// "3" and let a revert slip through green.) The re-extraction must also
// surface a nested dotted field, proving the epoch bump and the CUE depth
// fix are wired together end to end.
func TestIngest_ExtractorEpoch_CUEDepthChangeForcesReExtraction(t *testing.T) {
	dir := t.TempDir()
	cueSrc := []byte("package svc\n\n#Layer: {\n\tbudgets: {\n\t\tmax_usd_per_run: number\n\t}\n}\n")
	if err := os.WriteFile(filepath.Join(dir, "schema.cue"), cueSrc, 0o644); err != nil {
		t.Fatalf("write schema.cue: %v", err)
	}

	conn := connector.NewLocalConnector()
	st := store.NewMemoryStore()
	svc := core.ServiceRef{ID: "cue-epoch-svc", Path: dir}
	ctx := context.Background()

	oldSalted := sha256.Sum256(append(append([]byte{}, cueSrc...), []byte("\x00fathom-extractor-epoch:2")...))
	if err := st.SetFingerprint(ctx, svc.ID, "schema.cue", hex.EncodeToString(oldSalted[:])); err != nil {
		t.Fatalf("SetFingerprint: %v", err)
	}

	p := fusion.New(conn, st, defaultCfg())
	rep, err := p.Ingest(ctx, svc)
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if rep.Skipped != 0 {
		t.Fatal("schema.cue was skipped (a pre-epoch-3 fingerprint was honored) — ExtractorEpoch was not bumped for the CUE depth change")
	}

	obs, err := st.QueryObservations(ctx, svc.ID, "")
	if err != nil {
		t.Fatalf("QueryObservations: %v", err)
	}
	var sawNested bool
	for _, o := range obs {
		if o.Symbol == "#Layer.budgets.max_usd_per_run" {
			sawNested = true
		}
	}
	if !sawNested {
		t.Error("nested CUE field #Layer.budgets.max_usd_per_run not observed after forced re-extraction")
	}
}

// TestIngest_ExtractorEpoch_ToolIdentityForcesReExtraction is TI-8: the
// ADR-021 §2 tool-identity classifier generation changed yaml-family
// extraction semantics, so ExtractorEpoch was bumped "3"→"4". The seeded
// fingerprint below is salted with epoch "3" — the TRUE prior epoch —
// deliberately (the Wave-1 lesson, docs/plans/lang-truth.md): the test must
// fail if the bump reverts, which a seed of "2" or "1" would not catch
// (either would already mismatch and re-extract regardless of whether "4"
// ever existed).
//
// kustomization.yaml is the sharpest proof available: pre-classifier, it was
// dispatched to the generic k8s extractor, which only emits a workload node
// when BOTH kind and metadata.name are present — kustomization.yaml has no
// metadata.name, so it produced exactly zero symbols (silently). Forced
// re-extraction under the new KustomizeExtractor must surface real resource
// and image symbols from the very same bytes.
func TestIngest_ExtractorEpoch_ToolIdentityForcesReExtraction(t *testing.T) {
	dir := t.TempDir()
	kustomizeSrc := []byte("apiVersion: kustomize.config.k8s.io/v1beta1\n" +
		"kind: Kustomization\n" +
		"resources:\n  - deployment.yaml\n  - service.yaml\n" +
		"images:\n  - name: myapp\n    newTag: \"1.2.3\"\n")
	if err := os.WriteFile(filepath.Join(dir, "kustomization.yaml"), kustomizeSrc, 0o644); err != nil {
		t.Fatalf("write kustomization.yaml: %v", err)
	}

	conn := connector.NewLocalConnector()
	st := store.NewMemoryStore()
	svc := core.ServiceRef{ID: "kustomize-epoch-svc", Path: dir}
	ctx := context.Background()

	oldSalted := sha256.Sum256(append(append([]byte{}, kustomizeSrc...), []byte("\x00fathom-extractor-epoch:3")...))
	if err := st.SetFingerprint(ctx, svc.ID, "kustomization.yaml", hex.EncodeToString(oldSalted[:])); err != nil {
		t.Fatalf("SetFingerprint: %v", err)
	}

	p := fusion.New(conn, st, defaultCfg())
	rep, err := p.Ingest(ctx, svc)
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if rep.Skipped != 0 {
		t.Fatal("kustomization.yaml was skipped (a pre-epoch-4 fingerprint was honored) — ExtractorEpoch was not bumped for the tool-identity change")
	}

	obs, err := st.QueryObservations(ctx, svc.ID, "")
	if err != nil {
		t.Fatalf("QueryObservations: %v", err)
	}
	var sawResource, sawImage bool
	for _, o := range obs {
		if o.SourceKind == core.KindKustomizeResource && o.Symbol == "deployment.yaml" {
			sawResource = true
		}
		if o.SourceKind == core.KindKustomizeImage && o.Symbol == "myapp" {
			sawImage = true
		}
	}
	if !sawResource || !sawImage {
		t.Errorf("kustomization.yaml re-extraction did not surface resource/image symbols (resource=%v image=%v) — tool-identity classifier not wired", sawResource, sawImage)
	}
}

// TestIngest_ExtractorEpoch_TSEngineForcesReExtraction is XT-5: Python,
// JavaScript/TypeScript/JSX/TSX, and Ruby now extract via the wazero
// tree-sitter engine (internal/extractor.NewTSExtractor, ADR-021 §4)
// instead of the CGO tree-sitter bindings (ADR-013), so ExtractorEpoch was
// bumped "4"→"5". The seeded fingerprint below is salted with epoch "4" —
// the TRUE prior epoch (the Wave-1/2 lesson, docs/plans/lang-truth.md): the
// test must fail if the bump reverts, which seeding "3" or earlier would not
// catch (either would already mismatch and re-extract regardless of whether
// "5" ever existed).
func TestIngest_ExtractorEpoch_TSEngineForcesReExtraction(t *testing.T) {
	dir := t.TempDir()
	pySrc := []byte("def greet(name):\n    return name\n")
	if err := os.WriteFile(filepath.Join(dir, "greet.py"), pySrc, 0o644); err != nil {
		t.Fatalf("write greet.py: %v", err)
	}

	conn := connector.NewLocalConnector()
	st := store.NewMemoryStore()
	svc := core.ServiceRef{ID: "tsengine-epoch-svc", Path: dir}
	ctx := context.Background()

	oldSalted := sha256.Sum256(append(append([]byte{}, pySrc...), []byte("\x00fathom-extractor-epoch:4")...))
	if err := st.SetFingerprint(ctx, svc.ID, "greet.py", hex.EncodeToString(oldSalted[:])); err != nil {
		t.Fatalf("SetFingerprint: %v", err)
	}

	p := fusion.New(conn, st, defaultCfg())
	rep, err := p.Ingest(ctx, svc)
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if rep.Skipped != 0 {
		t.Fatal("greet.py was skipped (a pre-epoch-5 fingerprint was honored) — ExtractorEpoch was not bumped for the tree-sitter engine migration")
	}

	obs, err := st.QueryObservations(ctx, svc.ID, "")
	if err != nil {
		t.Fatalf("QueryObservations: %v", err)
	}
	var sawFunc bool
	for _, o := range obs {
		if o.SourceKind == core.KindPyFunc && o.Symbol == "greet" {
			sawFunc = true
		}
	}
	if !sawFunc {
		t.Error("greet.py re-extraction did not surface the greet() function — tsengine extractor not wired")
	}
}

// TestIngest_ExtractorEpoch_GitHooksAndCloudFormationForceReExtraction pins
// epoch 39 (LB-Q-git-hooks-config and LB-Q-cloudformation): a lefthook config
// was fingerprinted as unclassified YAML and a CloudFormation template as
// generic YAML before the two doc-engine dialects were dispatched, so the bump
// "38"→"39" must re-read both. The seeded fingerprints are salted with "38",
// the TRUE prior epoch, so the test fails if the bump reverts (the XT-5 rule
// above); every dialect test still passes on a fresh store without the bump.
func TestIngest_ExtractorEpoch_GitHooksAndCloudFormationForceReExtraction(t *testing.T) {
	dir := t.TempDir()
	files := map[string][]byte{
		"lefthook.yml": []byte("pre-commit:\n  commands:\n    lint:\n      run: go vet ./...\n"),
		"template.yaml": []byte("AWSTemplateFormatVersion: \"2010-09-09\"\n" +
			"Resources:\n  Queue:\n    Type: AWS::SQS::Queue\n"),
	}
	conn := connector.NewLocalConnector()
	st := store.NewMemoryStore()
	svc := core.ServiceRef{ID: "epoch39-svc", Path: dir}
	ctx := context.Background()
	for name, src := range files {
		if err := os.WriteFile(filepath.Join(dir, name), src, 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		oldSalted := sha256.Sum256(append(append([]byte{}, src...), []byte("\x00fathom-extractor-epoch:38")...))
		if err := st.SetFingerprint(ctx, svc.ID, name, hex.EncodeToString(oldSalted[:])); err != nil {
			t.Fatalf("SetFingerprint %s: %v", name, err)
		}
	}

	p := fusion.New(conn, st, defaultCfg())
	t.Cleanup(p.Close)
	rep, err := p.Ingest(ctx, svc)
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if rep.Skipped != 0 {
		t.Fatalf("%d file(s) skipped (a pre-epoch-39 fingerprint was honored) — ExtractorEpoch was not bumped for the git-hooks-config and cloudformation dispatch", rep.Skipped)
	}

	obs, err := st.QueryObservations(ctx, svc.ID, "")
	if err != nil {
		t.Fatalf("QueryObservations: %v", err)
	}
	var sawHook, sawResource bool
	for _, o := range obs {
		if o.SourceRef == "lefthook.yml" && o.SourceKind == core.KindLefthookHook && o.Symbol == "pre-commit" {
			sawHook = true
		}
		if o.SourceRef == "template.yaml" && o.SourceKind == "CFNResource" && o.Symbol == "Queue" {
			sawResource = true
		}
	}
	if !sawHook || !sawResource {
		t.Errorf("re-extraction did not surface the dialects' symbols (LefthookHook pre-commit=%v, CFNResource Queue=%v)", sawHook, sawResource)
	}
}

// TestIngest_ExtractorEpoch_GoUnexportedConstVarForceReExtraction pins epoch 40
// (LB-F60): the Go extractor emitted a package-level const or var only when its
// name was exported, so a .go file fingerprinted before the change skipped its
// unexported ones, and the bump "39"→"40" must re-read it. The seeded
// fingerprint is salted with "39", the TRUE prior epoch, so the test fails if
// the bump reverts (the XT-5 rule above); the extractor test passes on a fresh
// store without the bump.
func TestIngest_ExtractorEpoch_GoUnexportedConstVarForceReExtraction(t *testing.T) {
	dir := t.TempDir()
	const name = "limits.go"
	src := []byte("package limits\n\nconst maxRetries = 3\n\nvar defaultName = \"x\"\n")
	conn := connector.NewLocalConnector()
	st := store.NewMemoryStore()
	svc := core.ServiceRef{ID: "epoch40-svc", Path: dir}
	ctx := context.Background()
	if err := os.WriteFile(filepath.Join(dir, name), src, 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	oldSalted := sha256.Sum256(append(append([]byte{}, src...), []byte("\x00fathom-extractor-epoch:39")...))
	if err := st.SetFingerprint(ctx, svc.ID, name, hex.EncodeToString(oldSalted[:])); err != nil {
		t.Fatalf("SetFingerprint %s: %v", name, err)
	}

	p := fusion.New(conn, st, defaultCfg())
	t.Cleanup(p.Close)
	rep, err := p.Ingest(ctx, svc)
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if rep.Skipped != 0 {
		t.Fatalf("%d file(s) skipped (a pre-epoch-40 fingerprint was honored) — ExtractorEpoch was not bumped for the unexported Go const and var", rep.Skipped)
	}

	obs, err := st.QueryObservations(ctx, svc.ID, "")
	if err != nil {
		t.Fatalf("QueryObservations: %v", err)
	}
	var sawConst, sawVar bool
	for _, o := range obs {
		if o.SourceRef == name && o.SourceKind == core.KindGoConst && o.Symbol == "maxRetries" {
			sawConst = true
		}
		if o.SourceRef == name && o.SourceKind == core.KindGoVar && o.Symbol == "defaultName" {
			sawVar = true
		}
	}
	if !sawConst || !sawVar {
		t.Errorf("re-extraction did not surface the unexported symbols (GoConst maxRetries=%v, GoVar defaultName=%v)", sawConst, sawVar)
	}
}

// TestIngest_ExtractorEpoch_PackageJSONDependencyReferencesForceReExtraction
// pins the second item under epoch 40 (LB-Q-package-json, wave la): the
// package-json dialect gained a references block, so the manifest's package
// entity now calls each dependency name it writes. A package.json fingerprinted
// at epoch 39 carried no such call, and the same bump "39"→"40" must re-read it.
// The seeded fingerprint is salted with "39", the TRUE prior epoch (the XT-5
// rule above).
func TestIngest_ExtractorEpoch_PackageJSONDependencyReferencesForceReExtraction(t *testing.T) {
	dir := t.TempDir()
	const name = "package.json"
	src := []byte("{\n  \"name\": \"acme\",\n  \"dependencies\": { \"left-pad\": \"^1.3.0\" }\n}\n")
	conn := connector.NewLocalConnector()
	st := store.NewMemoryStore()
	svc := core.ServiceRef{ID: "epoch40-pkgjson-svc", Path: dir}
	ctx := context.Background()
	if err := os.WriteFile(filepath.Join(dir, name), src, 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	oldSalted := sha256.Sum256(append(append([]byte{}, src...), []byte("\x00fathom-extractor-epoch:39")...))
	if err := st.SetFingerprint(ctx, svc.ID, name, hex.EncodeToString(oldSalted[:])); err != nil {
		t.Fatalf("SetFingerprint %s: %v", name, err)
	}

	p := fusion.New(conn, st, defaultCfg())
	t.Cleanup(p.Close)
	rep, err := p.Ingest(ctx, svc)
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if rep.Skipped != 0 {
		t.Fatalf("%d file(s) skipped (a pre-epoch-40 fingerprint was honored) — ExtractorEpoch was not bumped for the package.json dependency references", rep.Skipped)
	}

	obs, err := st.QueryObservations(ctx, svc.ID, "")
	if err != nil {
		t.Fatalf("QueryObservations: %v", err)
	}
	var sawDep bool
	for _, o := range obs {
		if o.SourceRef != name || o.SourceKind != core.KindPackageJSONPackage || o.Symbol != "acme" {
			continue
		}
		for _, c := range o.Calls {
			if c == "left-pad" {
				sawDep = true
			}
		}
	}
	if !sawDep {
		t.Error("re-extraction did not surface the package's dependency reference (PackageJSONPackage acme -> left-pad)")
	}
}

// TestIngest_ExtractorEpoch_CallQualificationForcesReExtraction pins epoch 41
// (LB-F65, LB-F66, wave lb): a Python call is stored qualified and a
// decorator's call belongs to the definition it decorates, and the Go
// extractor records a method called on a selector chain or a call's result
// and no type, operand or field name as a call. A .py or .go file
// fingerprinted at epoch 40 carried the old call text, so the bump "40"→"41"
// must re-read it. The seeded fingerprints are salted with "40", the TRUE
// prior epoch (the XT-5 rule above).
func TestIngest_ExtractorEpoch_CallQualificationForcesReExtraction(t *testing.T) {
	dir := t.TempDir()
	files := map[string][]byte{
		"routes.py": []byte("@app.get(\"/items\")\ndef read_items():\n    return []\n"),
		"use.go":    []byte("package use\n\nfunc Use(r *Resp) {\n\tr.Header.Get(\"a\")\n}\n"),
	}
	conn := connector.NewLocalConnector()
	st := store.NewMemoryStore()
	svc := core.ServiceRef{ID: "epoch41-svc", Path: dir}
	ctx := context.Background()
	for name, src := range files {
		if err := os.WriteFile(filepath.Join(dir, name), src, 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		oldSalted := sha256.Sum256(append(append([]byte{}, src...), []byte("\x00fathom-extractor-epoch:40")...))
		if err := st.SetFingerprint(ctx, svc.ID, name, hex.EncodeToString(oldSalted[:])); err != nil {
			t.Fatalf("SetFingerprint %s: %v", name, err)
		}
	}

	p := fusion.New(conn, st, defaultCfg())
	t.Cleanup(p.Close)
	rep, err := p.Ingest(ctx, svc)
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if rep.Skipped != 0 {
		t.Fatalf("%d file(s) skipped (a pre-epoch-41 fingerprint was honored) — ExtractorEpoch was not bumped for call qualification", rep.Skipped)
	}

	obs, err := st.QueryObservations(ctx, svc.ID, "")
	if err != nil {
		t.Fatalf("QueryObservations: %v", err)
	}
	var sawDecorator, sawChain bool
	for _, o := range obs {
		for _, c := range o.Calls {
			if o.SourceRef == "routes.py" && o.Symbol == "read_items" && c == "app.get" {
				sawDecorator = true
			}
			if o.SourceRef == "use.go" && o.Symbol == "Use" && c == "r.Header.Get" {
				sawChain = true
			}
		}
	}
	if !sawDecorator || !sawChain {
		t.Errorf("re-extraction did not surface the new call text (read_items -> app.get %v, Use -> r.Header.Get %v)", sawDecorator, sawChain)
	}
}

// TestIngest_ExtractorEpoch_WaveTLFormatsForceReExtraction pins epoch 42
// (wave tl, one bump for the wave): Compose files extract through the
// docker-compose dialect and gain a ComposeFile root entity (LB-F89), OpenAPI
// and Swagger documents through the openapi dialect (LB-Q-openapi), `.tf`
// files drop loop variables and provider meta-arguments from Calls and join
// the doc-xref pass (LB-F84, LB-F86), and `*.gradle` files extract through the
// gradle dialect (LB-F94). A file fingerprinted at epoch 41 carried the old
// reading, so the bump "41"→"42" must re-read it. The seeded fingerprints are
// salted with "41", the TRUE prior epoch (the XT-5 rule above).
func TestIngest_ExtractorEpoch_WaveTLFormatsForceReExtraction(t *testing.T) {
	dir := t.TempDir()
	files := map[string][]byte{
		"compose.yaml": []byte("services:\n  web:\n    image: nginx\n"),
		"openapi.yaml": []byte("openapi: 3.0.3\ninfo:\n  title: Pets\n  version: \"1\"\npaths:\n  /pets:\n    get:\n      operationId: listPets\n      responses:\n        \"200\":\n          description: ok\n"),
		"main.tf":      []byte("variable \"names\" {}\n\nresource \"null_resource\" \"each\" {\n  triggers = { for n in var.names : n => n }\n}\n"),
		"build.gradle": []byte("plugins {\n  id 'java'\n}\n\ntask hello {\n  doLast { println 'hi' }\n}\n"),
	}
	conn := connector.NewLocalConnector()
	st := store.NewMemoryStore()
	svc := core.ServiceRef{ID: "epoch42-svc", Path: dir}
	ctx := context.Background()
	for name, src := range files {
		if err := os.WriteFile(filepath.Join(dir, name), src, 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		oldSalted := sha256.Sum256(append(append([]byte{}, src...), []byte("\x00fathom-extractor-epoch:41")...))
		if err := st.SetFingerprint(ctx, svc.ID, name, hex.EncodeToString(oldSalted[:])); err != nil {
			t.Fatalf("SetFingerprint %s: %v", name, err)
		}
	}

	p := fusion.New(conn, st, defaultCfg())
	t.Cleanup(p.Close)
	rep, err := p.Ingest(ctx, svc)
	if err != nil {
		t.Fatalf("Ingest: %v", err)
	}
	if rep.Skipped != 0 {
		t.Fatalf("%d file(s) skipped (a pre-epoch-42 fingerprint was honored) — ExtractorEpoch was not bumped for the wave tl formats", rep.Skipped)
	}

	obs, err := st.QueryObservations(ctx, svc.ID, "")
	if err != nil {
		t.Fatalf("QueryObservations: %v", err)
	}
	var sawComposeFile, sawOpenAPIDoc bool
	for _, o := range obs {
		if o.SourceRef == "compose.yaml" && o.SourceKind == core.SourceKind("ComposeFile") {
			sawComposeFile = true
		}
		if o.SourceRef == "openapi.yaml" && o.SourceKind == core.SourceKind("OpenAPIDocument") {
			sawOpenAPIDoc = true
		}
	}
	if !sawComposeFile {
		t.Error("re-extraction did not surface the docker-compose dialect's ComposeFile root entity for compose.yaml")
	}
	if !sawOpenAPIDoc {
		t.Error("re-extraction did not surface the openapi dialect's OpenAPIDocument entity for openapi.yaml")
	}
}

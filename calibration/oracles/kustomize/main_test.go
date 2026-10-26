package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	kyaml "sigs.k8s.io/kustomize/kyaml/yaml"
	"strings"
	"testing"
	"time"
)

func TestDecodeTypedPositionsAndMismatch(t *testing.T) {
	src := []byte("# lead\nresources:\n  - a.yaml\n  - b.yaml\npatches:\n  - path: p.yaml\nimages:\n  - name: web\n    newTag: v1\n")
	k, err := decode("x/kustomization.yaml", src)
	if err != nil {
		t.Fatal(err)
	}
	var got []entry
	for _, e := range k.entries {
		got = append(got, e)
	}
	want := []entry{{Field: "resources", Index: 0, Value: "a.yaml", Line: 3}, {Field: "resources", Index: 1, Value: "b.yaml", Line: 4}, {Field: "patches", Index: 0, Value: "p.yaml", Line: 6}, {Field: "images", Index: 0, Value: "web", Line: 8}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
	if k.line != 2 {
		t.Fatalf("root line %d", k.line)
	}
	if _, err := decode("x/kustomization.yaml", []byte("resources:\n  - a.yaml\n  - 7\n")); err == nil {
		t.Fatal("typed/node mismatch accepted")
	}
}

func TestRecognizedNamesAndAmbiguity(t *testing.T) {
	root := t.TempDir()
	for _, n := range []string{"Kustomization", "kustomization.yml"} {
		if err := os.WriteFile(filepath.Join(root, n), []byte("resources: [a.yaml]\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := findKustomizations(root); err == nil || !strings.Contains(err.Error(), "multiple") {
		t.Fatalf("want ambiguity, got %v", err)
	}
	if err := os.Remove(filepath.Join(root, "kustomization.yml")); err != nil {
		t.Fatal(err)
	}
	got, err := findKustomizations(root)
	if err != nil || !reflect.DeepEqual(got, []string{"Kustomization"}) {
		t.Fatalf("%v %v", got, err)
	}
}

func TestCaps(t *testing.T) {
	for _, tc := range []struct {
		n, max int
		ok     bool
	}{{512, 512, true}, {513, 512, false}, {8192, 8192, true}, {8193, 8192, false}, {64, 64, true}, {65, 64, false}} {
		if err := withinCap(tc.n, tc.max); (err == nil) != tc.ok {
			t.Fatalf("%+v: %v", tc, err)
		}
	}
}

func TestScratchOverlap(t *testing.T) {
	root := t.TempDir()
	if err := guardScratch(root, filepath.Join(root, "cache")); err == nil {
		t.Fatal("nested scratch accepted")
	}
	if err := guardScratch(root, t.TempDir()); err != nil {
		t.Fatal(err)
	}
}

func TestRemoteEntries(t *testing.T) {
	for v, want := range map[string]bool{"https://example.com/a": true, "github.com/org/repo": true, "../base": false, "./pod.yaml": false} {
		if got := remoteRE.MatchString(v); got != want {
			t.Errorf("%q: %v", v, got)
		}
	}
}

func TestBoundedOutput(t *testing.T) {
	b := &boundedBuffer{limit: 4}
	if n, err := b.Write([]byte("abcd")); err != nil || n != 4 {
		t.Fatalf("at cap: %d %v", n, err)
	}
	if _, err := b.Write([]byte("x")); err == nil {
		t.Fatal("cap+1 accepted")
	}
}

func TestRealComponentInParent(t *testing.T) {
	bin := os.Getenv("KUSTOMIZE_BIN")
	if bin == "" {
		t.Skip("set KUSTOMIZE_BIN for native integration")
	}
	if _, err := os.Stat(bin); err != nil {
		t.Fatalf("explicit KUSTOMIZE_BIN unavailable: %v", err)
	}
	work := t.TempDir()
	for p, s := range map[string]string{
		"parent/kustomization.yaml":    "resources:\n  - pod.yaml\ncomponents:\n  - ../component\n",
		"parent/pod.yaml":              "apiVersion: v1\nkind: Pod\nmetadata:\n  name: test\nspec:\n  containers:\n    - name: web\n      image: old:v1\n",
		"component/kustomization.yaml": "apiVersion: kustomize.config.k8s.io/v1alpha1\nkind: Component\nimages:\n  - name: old\n    newTag: v2\n",
	} {
		abs := filepath.Join(work, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(abs), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(s), 0644); err != nil {
			t.Fatal(err)
		}
	}
	r := runner{binary: bin, timeout: 30 * time.Second, env: nativeEnv(work, bin)}
	original, err := os.ReadFile(filepath.Join(work, "component/kustomization.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	k, err := decode("component/kustomization.yaml", original)
	if err != nil {
		t.Fatal(err)
	}
	if len(k.entries) != 1 {
		t.Fatalf("entries=%d", len(k.entries))
	}
	baseline, err := build(context.Background(), r, work, "parent/kustomization.yaml")
	if err != nil {
		t.Fatal(err)
	}
	variant, err := variantBuild(context.Background(), r, work, "parent/kustomization.yaml", "component/kustomization.yaml", original, k.entries[0])
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(baseline, variant) {
		t.Fatal("component content had no effect in actual parent")
	}
	restored, err := os.ReadFile(filepath.Join(work, "component/kustomization.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(original, restored) {
		t.Fatal("component was not restored")
	}
}

func TestNativeRejectionClassification(t *testing.T) {
	if err := exec.Command("/bin/sh", "-c", "exit 1").Run(); !nativeRejection(err) {
		t.Fatalf("exit 1 should be semantic rejection: %v", err)
	}
	for _, err := range []error{exec.Command("/bin/sh", "-c", "exit 2").Run(), exec.Command("/bin/sh", "-c", "kill -9 $$").Run(), errors.New("write failed"), context.DeadlineExceeded, errOutputCap} {
		if nativeRejection(err) {
			t.Fatalf("infrastructure failure accepted: %v", err)
		}
	}
}

func TestSourceAndBuildBudgets(t *testing.T) {
	for _, tc := range []struct {
		n, max int
		good   bool
	}{{maxSourceFiles, maxSourceFiles, true}, {maxSourceFiles + 1, maxSourceFiles, false}, {maxSourceBytes, maxSourceBytes, true}, {maxSourceBytes + 1, maxSourceBytes, false}, {maxNativeCalls, maxNativeCalls, true}, {maxNativeCalls + 1, maxNativeCalls, false}} {
		if (withinCap(tc.n, tc.max) == nil) != tc.good {
			t.Fatalf("%+v", tc)
		}
	}
}

func TestUniqueOwnedScratchPreservesPreexisting(t *testing.T) {
	root := t.TempDir()
	scratch := t.TempDir()
	old := filepath.Join(scratch, "subject")
	if err := os.Mkdir(old, 0700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(old, "sentinel")
	if err := os.WriteFile(marker, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	owned, err := ownedScratch(root, scratch)
	if err != nil {
		t.Fatal(err)
	}
	if owned == old {
		t.Fatal("reused preexisting directory")
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatal("deleted preexisting content")
	}
}

func TestBinarySHA256(t *testing.T) {
	p := filepath.Join(t.TempDir(), "kustomize")
	if err := os.WriteFile(p, []byte("x"), 0700); err != nil {
		t.Fatal(err)
	}
	h, err := fileSHA256(p)
	if err != nil {
		t.Fatal(err)
	}
	if h != "2d711642b726b04401627ca9fbac32f5c8530fb1903cc4db02258717921a4881" {
		t.Fatalf("hash %s", h)
	}
}

func TestTypedInventoryIncludesNonLists(t *testing.T) {
	k, err := decode("kustomization.yaml", []byte("namePrefix: p-\nnamespace: ns\ncommonLabels:\n  team: demo\nresources:\n  - pod.yaml\n"))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"unsupported:namePrefix", "unsupported:namespace", "unsupported:commonLabels", "graded:resources"} {
		if k.inventory[name] != 1 {
			t.Errorf("%s=%d", name, k.inventory[name])
		}
	}
}

func TestNestedRemoteClosureAndNativePaths(t *testing.T) {
	root, err := decode("parent/kustomization.yaml", []byte("resources:\n  - ../base\n"))
	if err != nil {
		t.Fatal(err)
	}
	base, err := decode("base/kustomization.yaml", []byte("components:\n  - ../component\n"))
	if err != nil {
		t.Fatal(err)
	}
	comp, err := decode("component/kustomization.yaml", []byte("apiVersion: kustomize.config.k8s.io/v1alpha1\nkind: Component\nconfigurations:\n  - https://example.com/config.yaml\n"))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]kfile{root.rel: root, base.rel: base, comp.rel: comp}
	graph := map[string][]string{root.rel: {base.rel}, base.rel: {comp.rel}}
	if got := closureUnsafeReason(root.rel, graph, files); got != "remote" {
		t.Fatalf("nested ungraded path: %q", got)
	}
	comp.typed.Configurations = []string{"/outside/config.yaml"}
	files[comp.rel] = comp
	if got := closureUnsafeReason(root.rel, graph, files); got != "absolute" {
		t.Fatalf("absolute path: %q", got)
	}
}

func TestNativeEnvBlocksGitNetwork(t *testing.T) {
	env := strings.Join(nativeEnv(t.TempDir(), "/bin/false"), "\n")
	for _, want := range []string{"GIT_ALLOW_PROTOCOL=file", "GIT_SSH_COMMAND=/usr/bin/false", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null"} {
		if !strings.Contains(env, want) {
			t.Errorf("missing %s", want)
		}
	}
}

func TestActualAdmissionCaps(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"a", "b", "c"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("x"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := preflightSourceWithLimits(root, 3, 3); err != nil {
		t.Fatal(err)
	}
	if _, _, err := preflightSourceWithLimits(root, 2, 3); err == nil {
		t.Fatal("file cap+1 admitted")
	}
	if _, _, err := preflightSourceWithLimits(root, 3, 2); err == nil {
		t.Fatal("byte cap+1 admitted")
	}
	files := map[string]kfile{"root": {entries: []entry{{}, {}}}}
	if _, err := plannedNativeCallsWithLimit([]string{"root"}, files, nil, 4); err != nil {
		t.Fatal(err)
	}
	if _, err := plannedNativeCallsWithLimit([]string{"root"}, files, nil, 3); err == nil {
		t.Fatal("native call cap+1 admitted")
	}
}

// TestOracleCLIHelper enters the same flag parsing and output path as the CLI.
// The parent test passes only local fixture paths through this private env value.
func TestNativeInvalidDecodeVsOracleMismatch(t *testing.T) {
	_, err := decode("invalid/kustomization.yaml", []byte("patches:\n  - legacy.yaml\n"))
	if err == nil || !isNativeConfigError(err) {
		t.Fatalf("native-invalid patch was not classified: %v", err)
	}
	node, err := kyaml.Parse("resources:\n  - a.yaml\n")
	if err != nil {
		t.Fatal(err)
	}
	_, err = reconcileField(node, fields[0], 2)
	if err == nil || isNativeConfigError(err) {
		t.Fatalf("typed/source mismatch must refuse the oracle: %v", err)
	}
	files := map[string]kfile{"parent/kustomization.yaml": {rel: "parent/kustomization.yaml"}}
	graph := map[string][]string{"parent/kustomization.yaml": {"invalid/kustomization.yaml"}}
	if reason := closureUnsafeReason("parent/kustomization.yaml", graph, files); reason != "native-decode-closure" {
		t.Fatalf("parent of invalid file: %q", reason)
	}
}

func TestOracleCLIHelper(t *testing.T) {
	raw := os.Getenv("KUSTOMIZE_ORACLE_TEST_ARGS")
	if raw == "" {
		return
	}
	var args []string
	if err := json.Unmarshal([]byte(raw), &args); err != nil {
		t.Fatal(err)
	}
	flag.CommandLine = flag.NewFlagSet("kustomize-oracle", flag.ExitOnError)
	os.Args = append([]string{"kustomize-oracle"}, args...)
	main()
}

func TestOracleCLIPopulatesComponent(t *testing.T) {
	bin := os.Getenv("KUSTOMIZE_BIN")
	if bin == "" {
		t.Skip("set KUSTOMIZE_BIN for native CLI integration")
	}
	if _, err := os.Stat(bin); err != nil {
		t.Fatalf("explicit KUSTOMIZE_BIN unavailable: %v", err)
	}
	digest, err := fileSHA256(bin)
	if err != nil {
		t.Fatal(err)
	}
	subject := t.TempDir()
	scratch := t.TempDir()
	fixture := map[string]string{
		"parent/kustomization.yaml":        "resources:\n  - pod.yaml\ncomponents:\n  - ../component\n",
		"parent/pod.yaml":                  "apiVersion: v1\nkind: Pod\nmetadata:\n  name: test\nspec:\n  containers:\n    - name: web\n      image: old:v1\n",
		"component/kustomization.yaml":     "apiVersion: kustomize.config.k8s.io/v1alpha1\nkind: Component\nimages:\n  - name: old\n    newTag: v2\n",
		"invalid/kustomization.yaml":       "patches:\n  - legacy.yaml\n",
		"broken-parent/kustomization.yaml": "resources:\n  - ../invalid\n",
	}
	for name, body := range fixture {
		p := filepath.Join(subject, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	filesOut := filepath.Join(scratch, "files.txt")
	receiptOut := filepath.Join(scratch, "receipt.json")
	args := []string{"-subject", subject, "-kustomize", bin, "-kustomize-sha256", digest, "-scratch", scratch, "-files-out", filesOut, "-receipt-out", receiptOut}
	argJSON, err := json.Marshal(args)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestOracleCLIHelper$")
	cmd.Env = append(os.Environ(), "KUSTOMIZE_ORACLE_TEST_ARGS="+string(argJSON))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.Output()
	if err != nil {
		t.Fatalf("oracle CLI: %v; stderr=%s", err, stderr.String())
	}
	var truth out
	if err := json.NewDecoder(bytes.NewReader(stdout)).Decode(&truth); err != nil {
		t.Fatalf("decode oracle stdout: %v; stdout=%s", err, stdout)
	}
	component := "component/kustomization.yaml"
	parent := "parent/kustomization.yaml"
	hasPackage, hasImage, hasParentEdge := false, false, false
	for _, d := range truth.Definitions {
		if d.Path == component && d.Kind == "package" {
			hasPackage = true
		}
		if d.Path == component && d.Kind == "data_source" && d.Name == "old" {
			hasImage = true
		}
	}
	for _, x := range truth.CrossReferences {
		if x.Path == parent && x.ToPath == component {
			hasParentEdge = true
		}
	}
	if !hasPackage || !hasImage || !hasParentEdge {
		t.Fatalf("component truth absent: package=%v image=%v parent edge=%v; truth=%+v", hasPackage, hasImage, hasParentEdge, truth)
	}
	fileList, err := os.ReadFile(filesOut)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains("\n"+string(fileList), "\n"+component+"\n") {
		t.Fatalf("component missing from files-out: %q", fileList)
	}
	receiptBytes, err := os.ReadFile(receiptOut)
	if err != nil {
		t.Fatal(err)
	}
	var rec receipt
	if err := json.Unmarshal(receiptBytes, &rec); err != nil {
		t.Fatal(err)
	}
	if rec.Files != 4 || rec.Excluded["native-decode-rejected"] != 1 || rec.Excluded["native-decode-closure"] != 1 {
		t.Fatalf("native rejection accounting: files=%d excluded=%v", rec.Files, rec.Excluded)
	}
	cc := rec.Contexts[component]
	if cc.Valid != 1 || cc.Effective != 1 {
		t.Fatalf("component context=%+v", cc)
	}

	// Change only the input fact. The same real parent still builds, but the
	// component image selector no longer matches its Pod.
	changed := strings.Replace(fixture[component], "name: old", "name: absent", 1)
	if err := os.WriteFile(filepath.Join(subject, filepath.FromSlash(component)), []byte(changed), 0644); err != nil {
		t.Fatal(err)
	}
	cmd = exec.Command(os.Args[0], "-test.run=^TestOracleCLIHelper$")
	cmd.Env = append(os.Environ(), "KUSTOMIZE_ORACLE_TEST_ARGS="+string(argJSON))
	stderr.Reset()
	cmd.Stderr = &stderr
	changedStdout, err := cmd.Output()
	if err != nil {
		t.Fatalf("oracle CLI after source change: %v; stderr=%s", err, stderr.String())
	}
	if bytes.Equal(stdout, changedStdout) {
		t.Fatal("raw oracle truth did not respond to changed source fact")
	}
	var changedTruth out
	if err := json.NewDecoder(bytes.NewReader(changedStdout)).Decode(&changedTruth); err != nil {
		t.Fatalf("decode changed oracle stdout: %v", err)
	}
	packageKept := false
	for _, d := range changedTruth.Definitions {
		if d.Path == component && d.Kind == "package" {
			packageKept = true
		}
		if d.Path == component && d.Kind == "data_source" {
			t.Fatalf("native-noop component image remained effective: %+v", d)
		}
	}
	if !packageKept {
		t.Fatal("component package disappeared after image selector became a no-op")
	}
	changedReceiptBytes, err := os.ReadFile(receiptOut)
	if err != nil {
		t.Fatal(err)
	}
	var changedRec receipt
	if err := json.Unmarshal(changedReceiptBytes, &changedRec); err != nil {
		t.Fatal(err)
	}
	changedContext := changedRec.Contexts[component]
	if changedContext.Valid != 1 || changedContext.Noop != 1 || changedContext.Effective != 0 {
		t.Fatalf("changed component context=%+v", changedContext)
	}
}

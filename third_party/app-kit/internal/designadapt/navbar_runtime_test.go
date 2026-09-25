package designadapt

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/app-kit/internal/designassets"
	"github.com/geoffmilleraz/app-kit/internal/designconvert"
)

func TestNavbarRuntimePatchIsExactAndExplicit(t *testing.T) {
	input := "prefix;" + navbarFocusBefore + ";suffix"
	got, err := patchNavbarMenuFocus(input)
	if err != nil || got != "prefix;"+navbarFocusAfter+";suffix" || strings.Replace(got, navbarFocusAfter, navbarFocusBefore, 1) != input {
		t.Fatal("exact reversible focus-site replacement failed")
	}
	for _, bad := range []string{"", navbarFocusAfter, navbarFocusBefore + navbarFocusBefore} {
		if _, err := patchNavbarMenuFocus(bad); err == nil {
			t.Fatal("missing, already patched or ambiguous source accepted")
		}
	}
	for _, body := range [][]byte{nil, []byte(input), bytes.Repeat([]byte("x"), (256<<10)+1)} {
		if _, err := AdaptNavbarRuntime(NavbarRepairArtifact{}, nil, body); err == nil {
			t.Fatal("unrecognized runtime accepted")
		}
	}
}

// This exercises the actual replacement expression with a controlled timer, not
// a reimplementation of its predicate. Browser proof is a separate opt-in run.
func TestNavbarRuntimeFocusGuardTransitions(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node required for isolated runtime-expression proof")
	}
	script := `const assert=require('node:assert/strict');
const invoke=new Function('setTimeout', ` + "`" + `return function(){` + navbarFocusAfter + `}` + "`" + `);
let cases=0;
for(const selected of [false,true])for(const connected of [false,true])for(const open of [false,true])for(const active of [null,{}]){
 let timer,focuses=0;const menu={isConnected:connected,matches:s=>{assert.equal(s,':popover-open');return open},getActiveItem:()=>active,closest:s=>{assert.equal(s,'[data-appkit-navbar-repair]');return selected?{}:null},focus:opts=>{assert.deepEqual(opts,{preventScroll:true});focuses++}};
 invoke(fn=>{assert.equal(timer,undefined);timer=fn}).call(menu);
 menu.closest=()=>{throw Error('selection must be captured before callback')};timer();
 assert.equal(focuses,Number(!selected||(connected&&open&&active===null)));cases++;
}
for(const change of ['select','close','detach']){
 let timer,focuses=0,active=null,open=true;const menu={isConnected:true,matches:()=>open,getActiveItem:()=>active,closest:()=>({}),focus:()=>focuses++};
 invoke(fn=>timer=fn).call(menu);
 if(change==='select')active={};if(change==='close')open=false;if(change==='detach')menu.isConnected=false;
 timer();assert.equal(focuses,0);cases++;
}
assert.equal(cases,19);`
	if out, err := exec.Command(node, "-e", script).CombinedOutput(); err != nil {
		t.Fatalf("focus guard transitions failed: %v\n%s", err, out)
	}
}

func TestNavbarRuntimeLive(t *testing.T) {
	kit := os.Getenv("APPKIT_PROBE_CONVERSION_KIT_ROOT")
	if kit == "" {
		t.Skip("explicit private checkout required")
	}
	if !filepath.IsAbs(kit) {
		t.Fatal("absolute private checkout required")
	}
	original, err := designassets.ReadFile(filepath.Join(kit, "data/design-assets"), NavbarArchive, NavbarMember)
	if err != nil {
		t.Fatal("verified original unavailable")
	}
	source := []byte(original.Content)
	repair, err := AdaptNavbarRepair(designconvert.Source{ArchiveID: original.ArchiveID, Path: original.Path, SHA256: original.SHA256, Size: len(source)}, source)
	if err != nil {
		t.Fatal("paired repair unavailable")
	}
	runtimePath := filepath.Join(kit, "data/design-assets/reference-assets/elements-1.0.22-pilot/elements.js")
	runtime, err := os.ReadFile(runtimePath)
	if err != nil {
		t.Fatal("pinned runtime unavailable")
	}
	t.Cleanup(func() {
		body, err := os.ReadFile(runtimePath)
		if err != nil || !bytes.Equal(body, runtime) {
			t.Error("original runtime changed")
		}
	})
	a, err := AdaptNavbarRuntime(repair, source, runtime)
	if err != nil || VerifyNavbarRuntimeReplay(a, repair, source, runtime) != nil {
		t.Fatal("runtime replay failed")
	}
	if a.RepairSHA256 != "77a5a32f909eeb1403be5eff43525d0f581d983d070e612ec6678b9b9532f581" || strings.Replace(a.Runtime, navbarFocusAfter, navbarFocusBefore, 1) != string(runtime) {
		t.Fatal("original companion or runtime drifted")
	}
	for _, field := range []string{"Adapter", "RepairSHA256", "OriginalRuntimeSHA256", "RuntimeSHA256", "RuleSHA256", "Runtime", "WrapperAttribute", "Verification"} {
		t.Run(field, func(t *testing.T) {
			changed := a
			value := reflect.ValueOf(&changed).Elem().FieldByName(field)
			value.SetString(value.String() + "x")
			if VerifyNavbarRuntimeReplay(changed, repair, source, runtime) == nil {
				t.Fatal("changed runtime envelope accepted")
			}
		})
	}
	changed := a
	changed.SchemaVersion++
	if VerifyNavbarRuntimeReplay(changed, repair, source, runtime) == nil {
		t.Fatal("changed schema accepted")
	}
	changed = a
	changed.Runtime += "\n/* unreviewed */"
	changed.RuntimeSHA256 = navbarRuntimeDigest([]byte(changed.Runtime))
	if VerifyNavbarRuntimeReplay(changed, repair, source, runtime) == nil {
		t.Fatal("self-rehashed runtime accepted")
	}
	changedRepair := repair
	changedRepair.CSS += " "
	if _, err := AdaptNavbarRuntime(changedRepair, source, runtime); err == nil {
		t.Fatal("changed paired repair accepted")
	}
	if t.Failed() {
		return
	}
	out := t.TempDir()
	if requested := os.Getenv("APPKIT_PROBE_NAVBAR_RUNTIME_OUTPUT"); requested != "" {
		out = heroPrivateOutput(t, requested)
	}
	encoded, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	encoded = append(encoded, '\n')
	receipt := map[string]any{"kind": "private-p09-navbar-runtime-replay", "artifact_sha256": navbarRuntimeDigest(encoded), "runtime_sha256": a.RuntimeSHA256, "repair_sha256": a.RepairSHA256, "original_runtime_sha256": a.OriginalRuntimeSHA256, "verification": "not-verified", "browser": "not-tested"}
	body, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string][]byte{"runtime-artifact.json": encoded, "elements-navbar.js": []byte(a.Runtime), "runtime-receipt.json": append(body, '\n')} {
		if err := os.WriteFile(filepath.Join(out, name), body, 0600); err != nil {
			t.Fatal("private runtime output failed")
		}
	}
	t.Logf("private runtime replay retained at %s", out)
}

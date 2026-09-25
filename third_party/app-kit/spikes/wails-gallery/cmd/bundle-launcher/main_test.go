package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/app-kit/spikes/wails-gallery/internal/macbundle"
)

func fixturePlan() macbundle.LaunchPlan {
	return macbundle.LaunchPlan{
		Executable: "/private/relocated/Gallery.app/Contents/Resources/gallery-server",
		Args:       []string{"-native-host", "/private/relocated/Gallery.app/Contents/Resources/gallery-native", "-native-host-sha256", strings.Repeat("a", 64)},
		Manifest: macbundle.Manifest{Files: [3]macbundle.FileRecord{
			{}, {}, {Path: "Contents/Resources/gallery-native", SHA256: strings.Repeat("a", 64)},
		}},
	}
}

func TestLauncherVerifiesThenExecsOnlyThePairedParent(t *testing.T) {
	var steps []string
	var out, diagnostics bytes.Buffer
	selected := fixturePlan()
	code := run(nil, &out, &diagnostics, func() (string, error) {
		steps = append(steps, "executable")
		return "/private/relocated/Gallery.app/Contents/MacOS/gallery-launcher", nil
	}, func(path string) (macbundle.LaunchPlan, error) {
		steps = append(steps, "verify")
		if !strings.HasSuffix(path, "/Contents/MacOS/gallery-launcher") {
			t.Fatalf("did not use own executable: %q", path)
		}
		return selected, nil
	}, func(path string, argv, env []string) error {
		steps = append(steps, "exec")
		if path != selected.Executable || !reflect.DeepEqual(argv, append([]string{path}, selected.Args...)) || !reflect.DeepEqual(env, []string{"LANG=C"}) {
			t.Fatalf("unexpected exec: %q %q %q", path, argv, env)
		}
		return nil
	}, []string{"LANG=C"})
	if code != 0 || out.Len() != 0 || diagnostics.Len() != 0 || !reflect.DeepEqual(steps, []string{"executable", "verify", "exec"}) {
		t.Fatalf("code=%d steps=%q out=%q errors=%q", code, steps, out.String(), diagnostics.String())
	}
}

func TestLauncherCheckAndHelpCannotExecute(t *testing.T) {
	for _, args := range [][]string{{"-check"}, {"-h"}, {"--help"}} {
		var out, diagnostics bytes.Buffer
		calls := 0
		code := run(args, &out, &diagnostics, func() (string, error) {
			return "/private/relocated/Gallery.app/Contents/MacOS/gallery-launcher", nil
		}, func(string) (macbundle.LaunchPlan, error) {
			calls++
			return fixturePlan(), nil
		}, func(string, []string, []string) error {
			t.Fatal("read-only path executed a process")
			return nil
		}, nil)
		if code != 0 || out.Len() == 0 || diagnostics.Len() != 0 {
			t.Fatalf("%q: %d %q %q", args, code, out.String(), diagnostics.String())
		}
		if args[0] == "-check" {
			var body struct {
				CheckedOnly bool `json:"checked_only"`
			}
			if calls != 1 || json.Unmarshal(out.Bytes(), &body) != nil || !body.CheckedOnly {
				t.Fatalf("check did not report verified-only: %q", out.String())
			}
		} else if calls != 0 {
			t.Fatal("help touched the bundle")
		}
	}
}

func TestLauncherRejectsArgumentsAndValidationFailureWithoutFallback(t *testing.T) {
	for _, args := range [][]string{{"-native-host", "/other"}, {"-browser"}, {"-check", "extra"}, {"-psn_0_12345"}} {
		if code := run(args, io.Discard, io.Discard, func() (string, error) {
			t.Fatal("invalid args reached filesystem")
			return "", nil
		}, nil, nil, nil); code != 2 {
			t.Fatalf("%q: %d", args, code)
		}
	}
	for _, ownFailure := range []bool{false, true} {
		var out, diagnostics bytes.Buffer
		code := run(nil, &out, &diagnostics, func() (string, error) {
			if ownFailure {
				return "", errors.New("synthetic executable refusal")
			}
			return "/fixture/launcher", nil
		}, func(string) (macbundle.LaunchPlan, error) {
			return macbundle.LaunchPlan{}, errors.New("changed native identity\n\x1b[31m")
		}, func(string, []string, []string) error {
			t.Fatal("refusal reached exec")
			return nil
		}, nil)
		if code != 1 || out.Len() != 0 || diagnostics.Len() == 0 || strings.Contains(diagnostics.String(), "\x1b") {
			t.Fatalf("refusal: %d %q %q", code, out.String(), diagnostics.String())
		}
	}
}

func TestLauncherExecFailureIsNotBrowserSuccess(t *testing.T) {
	var out, diagnostics bytes.Buffer
	calls := 0
	code := run(nil, &out, &diagnostics, func() (string, error) { return "/private/relocated/Gallery.app/Contents/MacOS/gallery-launcher", nil }, func(string) (macbundle.LaunchPlan, error) { return fixturePlan(), nil }, func(string, []string, []string) error {
		calls++
		return errors.New("synthetic exec failure")
	}, nil)
	if code != 1 || calls != 1 || out.Len() != 0 || !strings.Contains(diagnostics.String(), "no fallback") {
		t.Fatalf("exec refusal: %d calls=%d %q %q", code, calls, out.String(), diagnostics.String())
	}
}

func TestLauncherRefusesMalformedPlanBeforeCheckOrExec(t *testing.T) {
	cases := map[string]func(*macbundle.LaunchPlan){
		"browser mode":     func(p *macbundle.LaunchPlan) { p.Args = []string{"-browser"} },
		"extra flags":      func(p *macbundle.LaunchPlan) { p.Args = append(p.Args, "-native-child") },
		"wrong server":     func(p *macbundle.LaunchPlan) { p.Executable = "/private/other-server" },
		"relative server":  func(p *macbundle.LaunchPlan) { p.Executable = "gallery-server" },
		"wrong native":     func(p *macbundle.LaunchPlan) { p.Args[1] = "/private/other-native" },
		"invalid digest":   func(p *macbundle.LaunchPlan) { p.Args[3] = strings.Repeat("Z", 64) },
		"different digest": func(p *macbundle.LaunchPlan) { p.Args[3] = strings.Repeat("b", 64) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			for _, args := range [][]string{nil, {"-check"}} {
				selected := fixturePlan()
				mutate(&selected)
				calls := 0
				var out, diagnostics bytes.Buffer
				code := run(args, &out, &diagnostics, func() (string, error) {
					return "/private/relocated/Gallery.app/Contents/MacOS/gallery-launcher", nil
				}, func(string) (macbundle.LaunchPlan, error) { return selected, nil }, func(string, []string, []string) error {
					calls++
					return nil
				}, nil)
				if code != 1 || calls != 0 || out.Len() != 0 || diagnostics.Len() == 0 {
					t.Fatalf("malformed plan %q accepted: code=%d calls=%d out=%q errors=%q", args, code, calls, out.String(), diagnostics.String())
				}
			}
		})
	}
}

func TestLauncherDependencyClosureIsNativeFree(t *testing.T) {
	cmd := exec.Command("go", "list", "-mod=readonly", "-deps", ".")
	cmd.Env = append(os.Environ(), "GOOS=darwin", "CGO_ENABLED=0", "GOWORK=off", "GOPROXY=off", "GOSUMDB=off")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("dependency inspection: %v\n%s", err, output)
	}
	for _, dependency := range strings.Fields(string(output)) {
		if dependency == "runtime/cgo" || strings.HasPrefix(dependency, "github.com/wailsapp/") {
			t.Fatalf("bundle launcher imports native dependency %s", dependency)
		}
	}
}

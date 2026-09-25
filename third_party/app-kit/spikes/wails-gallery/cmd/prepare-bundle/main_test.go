package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/app-kit/spikes/wails-gallery/internal/macbundle"
)

func validArgs() []string {
	return []string{"-out", "/private/stage/Gallery.app", "-launcher", "/private/input/launcher", "-launcher-sha256", strings.Repeat("a", 64), "-server", "/private/input/server", "-server-sha256", strings.Repeat("b", 64), "-native", "/private/input/native", "-native-sha256", strings.Repeat("c", 64), "-kit-revision", "fixture-source", "-min-macos", "12.0"}
}

func TestPreparationRequiresExplicitInputsBeforeAssembly(t *testing.T) {
	for _, args := range [][]string{nil, {"-unknown"}, {"-out"}, {"extra"}, append(validArgs(), "extra"), append(validArgs(), "-native-sha256=")} {
		var out, diagnostics bytes.Buffer
		calls := 0
		code := run(args, &out, &diagnostics, func(string, macbundle.Inputs) (macbundle.Manifest, error) {
			calls++
			return macbundle.Manifest{}, nil
		})
		if code != 2 || calls != 0 || out.Len() != 0 || diagnostics.Len() == 0 {
			t.Fatalf("args %q: code=%d calls=%d out=%q errors=%q", args, code, calls, out.String(), diagnostics.String())
		}
	}
	for pair := 0; pair < len(validArgs()); pair += 2 {
		args := validArgs()
		args = append(args[:pair], args[pair+2:]...)
		if code := run(args, io.Discard, io.Discard, func(string, macbundle.Inputs) (macbundle.Manifest, error) {
			t.Fatal("assembler called with missing flag")
			return macbundle.Manifest{}, nil
		}); code != 2 {
			t.Fatalf("missing pair %d: code=%d", pair, code)
		}
	}
}

func TestPreparationPassesExactSelectionAndWritesOnlyJSON(t *testing.T) {
	var out, diagnostics bytes.Buffer
	calls := 0
	code := run(validArgs(), &out, &diagnostics, func(destination string, got macbundle.Inputs) (macbundle.Manifest, error) {
		calls++
		want := macbundle.Inputs{
			Launcher:    macbundle.Executable{Path: "/private/input/launcher", SHA256: strings.Repeat("a", 64)},
			Server:      macbundle.Executable{Path: "/private/input/server", SHA256: strings.Repeat("b", 64)},
			Native:      macbundle.Executable{Path: "/private/input/native", SHA256: strings.Repeat("c", 64)},
			KitRevision: "fixture-source", MinMacOS: "12.0",
		}
		if destination != "/private/stage/Gallery.app" || !reflect.DeepEqual(got, want) {
			t.Fatalf("selection changed: %q %#v", destination, got)
		}
		return macbundle.Manifest{}, nil
	})
	var manifest macbundle.Manifest
	if code != 0 || calls != 1 || diagnostics.Len() != 0 || json.Unmarshal(out.Bytes(), &manifest) != nil {
		t.Fatalf("code=%d calls=%d out=%q errors=%q", code, calls, out.String(), diagnostics.String())
	}
}

func TestPreparationHelpAndFailuresNeverPretendToLaunch(t *testing.T) {
	var out, diagnostics bytes.Buffer
	if code := run([]string{"-h"}, &out, &diagnostics, func(string, macbundle.Inputs) (macbundle.Manifest, error) {
		t.Fatal("help must not assemble")
		return macbundle.Manifest{}, nil
	}); code != 0 || diagnostics.Len() != 0 || !strings.Contains(out.String(), "without launching") {
		t.Fatalf("help: %d %q %q", code, out.String(), diagnostics.String())
	}
	out.Reset()
	code := run(validArgs(), &out, &diagnostics, func(string, macbundle.Inputs) (macbundle.Manifest, error) {
		return macbundle.Manifest{}, errors.New("identity refused\n\x1b[31m")
	})
	if code != 1 || out.Len() != 0 || strings.ContainsAny(diagnostics.String(), "\x1b\r") || !json.Valid(bytes.TrimSpace(diagnostics.Bytes())) {
		t.Fatalf("failure: %d %q %q", code, out.String(), diagnostics.String())
	}
	code = run(validArgs(), failingWriter{}, io.Discard, func(string, macbundle.Inputs) (macbundle.Manifest, error) { return macbundle.Manifest{}, nil })
	if code != 1 {
		t.Fatalf("report write failure = %d", code)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("synthetic closed output") }

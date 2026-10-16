package main

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/cli/command"
	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/designadapt"
)

func TestAssetsAdaptHeroDispatch(t *testing.T) {
	if err := dataCmd(context.Background(), []string{"assets-adapt-hero", "-root", ""}); command.Code(err) != 2 {
		t.Fatal("hero subcommand not routed to strict invocation validation")
	}
}

func captureHeroCommand(t *testing.T, args ...string) ([]byte, error) {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	previous := os.Stdout
	os.Stdout = writer
	defer func() {
		os.Stdout = previous
		writer.Close()
	}()
	type result struct {
		body []byte
		err  error
	}
	done := make(chan result, 1)
	go func() {
		body, err := io.ReadAll(reader)
		done <- result{body, err}
	}()
	commandErr := dataAssetsAdaptHero(args)
	os.Stdout = previous
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	got := <-done
	if got.err != nil {
		t.Fatal(got.err)
	}
	return got.body, commandErr
}

func TestAssetsAdaptHeroRefusesBadFlagsAndUnavailableSourceWithoutOutput(t *testing.T) {
	root := t.TempDir()
	out := filepath.Join(root, "out")
	for _, args := range [][]string{
		{"-root", ""},
		{"-root", root, "extra"},
		{"-root", root, "-out="},
		{"-root", root, "-check="},
		{"-root", root, "-out", out, "-check", filepath.Join(root, "check")},
		{"-root", root, "-unknown"},
		{"-root", root},
		{"-root", root, "-out", out},
		{"-root", root, "-check", filepath.Join(root, "missing-delivery")},
	} {
		body, err := captureHeroCommand(t, args...)
		if command.Code(err) != 2 || len(body) != 0 {
			t.Fatalf("refusal lost exit/output boundary for %v", args)
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("invalid or unavailable source caused output")
	}
}

func TestAssetsAdaptHeroDefaultDoesNotCreateFilesystemState(t *testing.T) {
	parent := t.TempDir()
	missingRoot := filepath.Join(parent, "missing-assets")
	body, err := captureHeroCommand(t, "-root", missingRoot)
	if command.Code(err) != 2 || len(body) != 0 {
		t.Fatal("unavailable default JSON source lost exit/output boundary")
	}
	entries, readErr := os.ReadDir(parent)
	if readErr != nil || len(entries) != 0 {
		t.Fatal("read-only default created filesystem state")
	}
}

func TestHeroDeliveryExitClassification(t *testing.T) {
	if err := heroDeliveryExit(nil); err != nil {
		t.Fatal(err)
	}
	if code := command.Code(heroDeliveryExit(designadapt.ErrHeroDeliveryDrift)); code != 1 {
		t.Fatalf("drift code = %d", code)
	}
	wrapped := errors.Join(errors.New("context"), designadapt.ErrHeroDeliveryDrift)
	if code := command.Code(heroDeliveryExit(wrapped)); code != 1 {
		t.Fatalf("wrapped drift code = %d", code)
	}
	if code := command.Code(heroDeliveryExit(errors.New("could not check"))); code != 2 {
		t.Fatalf("operational error code = %d", code)
	}
}

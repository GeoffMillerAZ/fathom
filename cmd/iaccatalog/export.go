package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// dirExporter reads exports already made: <dir>/<namespace>-<name>_<version>.json. It is how a build runs on a
// machine that cannot run the provider's platform, and how the tests run with no OpenTofu.
type dirExporter struct {
	dir  string
	tool string
}

func (d dirExporter) export(_ context.Context, line pinLine, rel pinRelease) (io.ReadCloser, string, error) {
	name := strings.ReplaceAll(line.Source, "/", "-") + "_" + rel.Version + ".json"
	f, err := os.Open(filepath.Join(d.dir, name))
	if err != nil {
		return nil, "", err
	}
	return f, d.tool, nil
}

// tofuExporter runs OpenTofu against a filesystem mirror holding the pinned archive, so nothing is fetched: the
// archive must already be on disk and must hash to its pin. This is the maintainer tool the product never runs
// (I56): the product binary links none of it.
type tofuExporter struct {
	tofu     string
	archives string
	work     string
}

func archivePlatform() string { return runtime.GOOS + "_" + runtime.GOARCH }

func (t tofuExporter) export(ctx context.Context, line pinLine, rel pinRelease) (io.ReadCloser, string, error) {
	if archivePlatform() != line.Platform {
		return nil, "", fmt.Errorf("the pins are for %s and this host is %s: build there, or pass exports made there with -exports", line.Platform, archivePlatform())
	}
	name := line.Source[strings.IndexByte(line.Source, '/')+1:]
	zipName := fmt.Sprintf("terraform-provider-%s_%s_%s.zip", name, rel.Version, line.Platform)
	src := filepath.Join(t.archives, zipName)
	data, err := os.ReadFile(src)
	if err != nil {
		return nil, "", fmt.Errorf("the pinned archive is missing: %w", err)
	}
	sum := sha256.Sum256(data)
	if got := hex.EncodeToString(sum[:]); got != rel.SHA256 {
		return nil, "", fmt.Errorf("%s hashes to %s, not its pin %s", zipName, got, rel.SHA256)
	}
	dir, err := os.MkdirTemp(t.work, "tofu-*")
	if err != nil {
		return nil, "", err
	}
	defer os.RemoveAll(dir)
	mirror := filepath.Join(dir, "mirror", line.Registry, line.Source)
	if err := os.MkdirAll(mirror, 0o755); err != nil {
		return nil, "", err
	}
	if err := os.WriteFile(filepath.Join(mirror, zipName), data, 0o644); err != nil {
		return nil, "", err
	}
	rc := filepath.Join(dir, "cli.tfrc")
	cfg := fmt.Sprintf("provider_installation {\n  filesystem_mirror {\n    path = %q\n  }\n}\n", filepath.Join(dir, "mirror"))
	if err := os.WriteFile(rc, []byte(cfg), 0o644); err != nil {
		return nil, "", err
	}
	work := filepath.Join(dir, "work")
	if err := os.MkdirAll(work, 0o755); err != nil {
		return nil, "", err
	}
	tf := fmt.Sprintf("terraform {\n  required_providers {\n    %s = {\n      source  = \"%s/%s\"\n      version = \"= %s\"\n    }\n  }\n}\n", name, line.Registry, line.Source, rel.Version)
	if err := os.WriteFile(filepath.Join(work, "main.tf"), []byte(tf), 0o644); err != nil {
		return nil, "", err
	}
	env := append(cleanEnv(), "TF_CLI_CONFIG_FILE="+rc, "TF_DATA_DIR="+filepath.Join(dir, "data"), "TF_IN_AUTOMATION=1", "CHECKPOINT_DISABLE=1")
	run := func(args ...string) ([]byte, error) {
		cmd := exec.CommandContext(ctx, t.tofu, args...)
		cmd.Dir, cmd.Env = work, env
		var out, errb bytes.Buffer
		cmd.Stdout, cmd.Stderr = &out, &errb
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("%s %s: %w: %s", t.tofu, strings.Join(args, " "), err, tail(errb.String()))
		}
		return out.Bytes(), nil
	}
	if _, err := run("init", "-input=false", "-no-color"); err != nil {
		return nil, "", err
	}
	schema, err := run("providers", "schema", "-json")
	if err != nil {
		return nil, "", err
	}
	ver, err := run("version", "-json")
	if err != nil {
		return nil, "", err
	}
	var v struct {
		Version string `json:"terraform_version"`
	}
	if err := json.Unmarshal(ver, &v); err != nil || v.Version == "" {
		return nil, "", fmt.Errorf("tofu version -json is unreadable")
	}
	return io.NopCloser(bytes.NewReader(schema)), "opentofu " + v.Version, nil
}

// cleanEnv drops variables that would let a provider reach outside the mirror or share state.
func cleanEnv() []string {
	var out []string
	for _, e := range os.Environ() {
		k, _, _ := strings.Cut(e, "=")
		if strings.HasPrefix(k, "TF_") || k == "CHECKPOINT_DISABLE" {
			continue
		}
		out = append(out, e)
	}
	return out
}

func tail(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 400 {
		return s[len(s)-400:]
	}
	return s
}

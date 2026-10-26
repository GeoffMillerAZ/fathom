package main

import (
	"encoding/json"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestMatrixUnionKeepsDisabledWorkflowsAndDistinctRequirements(t *testing.T) {
	dst := &compiled{jobs: map[string]bool{}, workflows: map[string][]cwjob{}}
	mergeCompiled(dst, &compiled{jobs: map[string]bool{"a": true}, workflows: map[string][]cwjob{"ordinary": {{name: "a", requires: []string{"z"}}}}})
	mergeCompiled(dst, &compiled{jobs: map[string]bool{"b": true}, workflows: map[string][]cwjob{"scheduled": {{name: "b"}}, "ordinary": {{name: "a", requires: []string{"y", "z"}}}}})
	want := map[string][]cwjob{"ordinary": {{name: "a", requires: []string{"y", "z"}}}, "scheduled": {{name: "b"}}}
	if !reflect.DeepEqual(dst.workflows, want) || len(dst.jobs) != 2 {
		t.Fatalf("union: %#v", dst)
	}
}

func TestParameterProtocolRejectsIdentityAndInvalidCases(t *testing.T) {
	for _, tc := range []struct{ body, name, sha, want string }{
		{`{"subjects":[{"name":"example/project","sha":"abc","configs":{".circleci/config.yml":[{}]}}]}`, "example/project", "def", "identity"},
		{`{"subjects":[{"name":"example/project","sha":"abc","configs":{".circleci/config.yml":[]}}]}`, "example/project", "abc", "cases"},
		{`{"subjects":[{"name":"example/project","sha":"abc","configs":{".circleci/config.yml":[{"x":{}}]}}]}`, "example/project", "abc", "scalar"},
		{`{"subjects":[]}`, "missing", "abc", "subject"},
	} {
		t.Run(tc.want, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "matrix.json")
			if err := os.WriteFile(p, []byte(tc.body), 0600); err != nil {
				t.Fatal(err)
			}
			_, err := readParameterProtocol(p, tc.name, tc.sha)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want %s", err, tc.want)
			}
		})
	}
}

// Exercise main's fail-closed path, including parse errors before native compilation.
func TestMatrixMainRejectsInvalidAndDirtyConfigs(t *testing.T) {
	for _, tc := range []struct{ name, body, dirty, want string }{
		{"malformed", "jobs: [\n", "", "configuration"},
		{"modified", "version: 2.1\njobs: {}\n", "version: 2.1\njobs: {changed: {}}\n", "dirty"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			runGit := func(args ...string) string {
				t.Helper()
				out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
				if err != nil {
					t.Fatalf("git: %v: %s", err, out)
				}
				return strings.TrimSpace(string(out))
			}
			runGit("init", "-q")
			if err := os.MkdirAll(filepath.Join(dir, ".circleci"), 0700); err != nil {
				t.Fatal(err)
			}
			config := filepath.Join(dir, ".circleci/config.yml")
			if err := os.WriteFile(config, []byte(tc.body), 0600); err != nil {
				t.Fatal(err)
			}
			runGit("add", ".circleci")
			runGit("-c", "user.name=Oracle test", "-c", "user.email=oracle@example.invalid", "-c", "commit.gpgsign=false", "commit", "-qm", "fixture")
			sha := runGit("rev-parse", "HEAD")
			if tc.dirty != "" {
				if err := os.WriteFile(config, []byte(tc.dirty), 0600); err != nil {
					t.Fatal(err)
				}
			}
			protocol := filepath.Join(t.TempDir(), "matrix.json")
			body, _ := json.Marshal(map[string]any{"subjects": []any{map[string]any{"name": "example/project", "sha": sha, "configs": map[string]any{".circleci/config.yml": []any{map[string]any{}}}}}})
			if err := os.WriteFile(protocol, body, 0600); err != nil {
				t.Fatal(err)
			}
			cli := filepath.Join(t.TempDir(), "circleci")
			if err := os.WriteFile(cli, []byte("#!/bin/sh\nif [ \"$1\" = version ]; then echo 1.7.0; else echo 'jobs: {}'; fi\n"), 0700); err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(os.Args[0], "-test.run=^TestMatrixMainHelper$", "--", "-subject", dir, "-subject-name", "example/project", "-matrix", protocol, "-scratch", t.TempDir(), "-cli", cli)
			cmd.Env = append(os.Environ(), "CIRCLECI_MATRIX_MAIN_HELPER=1")
			out, err := cmd.CombinedOutput()
			if err == nil || !strings.Contains(string(out), tc.want) {
				t.Fatalf("err=%v output=%s; want failure containing %s", err, out, tc.want)
			}
		})
	}
}

func TestMatrixMainHelper(t *testing.T) {
	if os.Getenv("CIRCLECI_MATRIX_MAIN_HELPER") != "1" {
		return
	}
	for i, a := range os.Args {
		if a == "--" {
			os.Args = append([]string{os.Args[0]}, os.Args[i+1:]...)
			break
		}
	}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	main()
	os.Exit(0)
}

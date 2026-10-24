package main

import (
	"bytes"
	"maps"
	"net/http"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestReadoutHookRtkStandsDown(t *testing.T) {
	for _, mode := range []string{"observe", "lean", "rich"} {
		t.Run(mode, func(t *testing.T) {
			readoutHookEnv(t, "[readouts]\nmode = \""+mode+"\"\n")
			cwd := t.TempDir()
			for _, command := range []string{"rtk ls -1", "env X=1 rtk ls -1", "sudo rtk ls -1", "FATHOM_READOUT=raw rtk ls -1", "ls -1"} {
				t.Run(command, func(t *testing.T) {
					freshHookLog(t)
					code, out, errOut, steps := runReadoutHook(t, []string{"--event", "post-tool"}, readoutPostToolEnvelope(t, cwd, command, readoutListing("")))
					if code != 0 || out != "" || errOut != "" {
						t.Fatalf("exit %d, stdout %q, stderr %q; want silent success", code, out, errOut)
					}
					rows := readoutRows(t)
					if len(rows) != 1 {
						t.Fatalf("rows = %+v, want one readout row", rows)
					}
					row := rows[0]
					if command == "ls -1" {
						if row.Meta["rule"] != "ls-head@1" || row.Meta["raw"] != "false" || !slices.Contains(steps, "render") {
							t.Fatalf("unprefixed readout lost: row %+v, steps %v", row, steps)
						}
						return
					}
					if row.Kind != "raw" || row.Meta["reason"] != "rtk" || row.Meta["raw"] != "true" || row.Meta["emitted"] != "false" || row.Meta["rule"] != "" {
						t.Errorf("RTK skip row = %+v", row)
					}
					if row.Meta["bytes_in"] != row.Meta["bytes_out"] || row.Meta["lines_in"] != row.Meta["lines_out"] {
						t.Errorf("RTK skip must preserve output sizes: %+v", row)
					}
					if !slices.Contains(steps, "test-run") {
						t.Errorf("RTK run skipped test-run tier: %v", steps)
					}
					for _, step := range []string{"rules", "render", "joins", "rich", "policy"} {
						if slices.Contains(steps, step) {
							t.Errorf("RTK run reached %s: %v", step, steps)
						}
					}
				})
			}
		})
	}
}

func TestReadoutHookRtkSuppressesRichAdvice(t *testing.T) {
	repo := t.TempDir()
	readoutGitRepo(t, repo, map[string]string{
		"go.mod":            "module example.com/sample\n\ngo 1.22\n",
		".fathom/guard.cue": readoutGuardFixture,
	})
	for _, command := range []string{"rtk go test ./...", "go test ./..."} {
		t.Run(command, func(t *testing.T) {
			readoutHookEnv(t, "[readouts]\nmode = \"rich\"\n")
			code, out, errOut, _ := runReadoutHook(t, []string{"--event", "post-tool"}, readoutPostToolEnvelope(t, repo, command, "ok\texample.com/sample\t(cached)\n"))
			if code != 0 || errOut != "" {
				t.Fatalf("exit %d, stderr %q", code, errOut)
			}
			rows := readoutRows(t)
			if len(rows) != 1 {
				t.Fatalf("rows = %+v, want one readout row", rows)
			}
			if strings.HasPrefix(command, "rtk ") {
				if out != "" || rows[0].Meta["reason"] != "rtk" || rows[0].Meta["emitted"] != "false" {
					t.Fatalf("RTK output was enriched: stdout %q, row %+v", out, rows[0])
				}
			} else if !strings.Contains(out, "this command lacks -race") || rows[0].Meta["emitted"] != "true" {
				t.Fatalf("unprefixed rich advice lost: stdout %q, row %+v", out, rows[0])
			}
		})
	}
}

func TestReadoutHookRtkRecordsTestRuns(t *testing.T) {
	for _, mode := range []string{"observe", "lean", "rich"} {
		for _, outsideRoots := range []bool{false, true} {
			for _, command := range []string{"rtk go test ./x", "rtk test go test ./x", "rtk git status && go test ./..."} {
				for _, failed := range []bool{false, true} {
					t.Run(mode+"/outside="+strconv.FormatBool(outsideRoots)+"/"+command+"/failed="+strconv.FormatBool(failed), func(t *testing.T) {
						readoutHookEnv(t, "")
						t.Setenv("FATHOM_HTTP", "http://example.invalid")
						serveHookRequests(t, http.HandlerFunc(http.NotFound))
						repo := t.TempDir()
						gitInitTempRepo(t, repo)
						writeRepoFile(t, repo, "Taskfile.yml", "version: '3'\ntasks:\n  test:\n    cmds: [\"go test ./...\"]\n")
						writeRepoFile(t, repo, ".fathom/config.toml", "[hooks]\ntest_run = true\n")
						payload := testRunPostPayload(repo, command, "ok\texample.com/sample\t(cached)\n")
						event, standaloneEvent := "post-tool", "test-run"
						if failed {
							payload = testRunFailurePayload(repo, command, foldTailFail)
							event, standaloneEvent = "tool-failure", "failed-test-run"
						}
						var standalone, stderr bytes.Buffer
						runWithStdin([]string{"session-hook", "--event", standaloneEvent}, strings.NewReader(payload), &standalone, &stderr)
						want := foldRows(t)
						if len(want) != 1 || stderr.Len() != 0 {
							t.Fatalf("standalone rows %v, stderr %q; want one test run", want, stderr.String())
						}
						if mode == "rich" && failed {
							// Rich mode also records its delta status; a fresh log
							// has no prior run to compare against.
							want[0]["delta"] = "not-compared"
						}
						cfg := "[readouts]\nmode = " + strconv.Quote(mode) + "\n"
						if outsideRoots {
							cfg += "roots = [" + strconv.Quote(filepath.Join(repo, "elsewhere")) + "]\n"
						}
						setUserConfig(t, cfg)
						freshHookLog(t)
						code, out, errOut, steps := runReadoutHook(t, []string{"--event", event}, payload)
						if code != 0 || errOut != "" || out != standalone.String() {
							t.Errorf("folded exit %d, stderr %q, output %q; want output %q", code, errOut, out, standalone.String())
						}
						if got := foldRows(t); len(got) != 1 || !maps.Equal(got[0], want[0]) {
							t.Errorf("folded rows %v, want %v", got, want)
						}
						if !slices.Contains(steps, "test-run") {
							t.Errorf("test-run tier skipped: %v", steps)
						}
						rows := readoutRows(t)
						if outsideRoots {
							if len(rows) != 0 {
								t.Errorf("outside readout roots: rows %+v", rows)
							}
						} else if len(rows) != 1 || rows[0].Meta["reason"] != "rtk" || rows[0].Meta["emitted"] != "false" {
							t.Errorf("RTK readout stand-down lost: %+v", rows)
						}
					})
				}
			}
		}
	}
}

func TestMatchDeclaredTestCommandRtk(t *testing.T) {
	repo := t.TempDir()
	writeRepoFile(t, repo, "Taskfile.yml", "version: '3'\ntasks:\n  test:\n    cmds: [\"go test ./...\"]\n")
	for _, command := range []string{"rtk go test ./...", "rtk test go test ./..."} {
		t.Run(command, func(t *testing.T) {
			m, ok := matchDeclaredTestCommand(repo, command, nil)
			if !ok || m.Invocation != "go test" || m.Source == "" || m.Piped {
				t.Fatalf("matchDeclaredTestCommand(%q) = %+v, %v; want declared go test", command, m, ok)
			}
		})
	}
}

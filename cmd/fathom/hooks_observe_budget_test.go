package main

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/harness"
)

func TestTestsToRunObserveLogsOutputCap(t *testing.T) {
	for _, mode := range []string{"true", `"observe"`} {
		for _, fits := range []bool{true, false} {
			t.Run(mode+"/fits="+boolStr(fits), func(t *testing.T) {
				repo := testRunRepo(t, false)
				t.Setenv("FATHOM_HOME", t.TempDir())
				writeRepoFile(t, repo, ".fathom/config.toml", "[hooks]\ntests_to_run = "+mode+"\n")
				ident := handleIdentity(func(*http.Request) (map[string]svcInfo, error) {
					return map[string]svcInfo{"svc": {symbols: 1, langs: "go", path: repo}}, nil
				})
				serveHookRequests(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/identity" {
						ident.ServeHTTP(w, r)
					} else {
						joinRPCText(w, map[string]any{"total_symbols": 1, "tests": []string{"svc::money/money.TestRound"}})
					}
				}))
				text := "existing context"
				if !fits {
					text = strings.Repeat("context ", specLineTokenBudget)
				}
				got, ok := withTestsToRun("http://localhost", filepath.Join(repo, "money/money.go"), "budget-session", text, true)
				emitted := mode == "true" && fits
				if !ok || (emitted && !strings.Contains(got, "tests to run: TestRound")) || (!emitted && got != text) {
					t.Fatalf("context=%q ok=%v", got, ok)
				}
				beats, _ := readBeatLog(beatLogPath())
				for _, b := range beats {
					if b.Channel != testsToRunChannel {
						continue
					}
					dropped := ""
					if !fits {
						dropped = "budget"
					}
					if b.Kind != "line" || b.Meta["named"] != "1" || b.Meta["emitted"] != boolStr(emitted) || b.Meta["dropped"] != dropped {
						t.Fatalf("budget row=%+v, want emitted=%v dropped=%q", b, emitted, dropped)
					}
					return
				}
				t.Fatal("missing tests-to-run row")
			})
		}
	}
}

func TestPromptDefinitionsObserveLogsOutputCap(t *testing.T) {
	for _, mode := range []string{"true", `"observe"`} {
		for _, fits := range []bool{true, false} {
			t.Run(mode+"/fits="+boolStr(fits), func(t *testing.T) {
				repo := testRunRepo(t, false)
				t.Setenv("FATHOM_HOME", t.TempDir())
				writeRepoFile(t, repo, ".fathom/config.toml", "[hooks]\nprompt_definitions = "+mode+"\n")
				text := "existing context"
				if !fits {
					text = strings.Repeat("x", harness.CodexAdditionalContextCap)
				}
				defs := []promptDefinition{{Name: "Round", Kind: "func", File: "money/money.go", Line: 3, Count: 1}}
				got, ok := withPromptDefinitions(repo, "budget-session", text, true, defs, false)
				emitted := mode == "true" && fits
				if !ok || (emitted && !strings.Contains(got, "defined: Round")) || (!emitted && got != text) {
					t.Fatalf("context=%q ok=%v", got, ok)
				}
				beats, _ := readBeatLog(beatLogPath())
				for _, b := range beats {
					if b.Channel != "prompt-definitions" {
						continue
					}
					dropped := ""
					if !fits {
						dropped = "budget"
					}
					if b.Kind != "line" || b.Meta["definitions"] != "1" || b.Meta["emitted"] != boolStr(emitted) || b.Meta["dropped"] != dropped {
						t.Fatalf("budget row=%+v, want emitted=%v dropped=%q", b, emitted, dropped)
					}
					return
				}
				t.Fatal("missing prompt-definitions row")
			})
		}
	}
}

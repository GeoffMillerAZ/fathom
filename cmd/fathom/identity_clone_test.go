package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestIdentityClone(t *testing.T) {
	repo := t.TempDir()
	base := t.TempDir()
	clone := filepath.Join(base, "clone")
	gitT(t, repo, "init", "-q")
	if err := os.WriteFile(filepath.Join(repo, "a.go"), []byte("package a\n"), 0644); err != nil {
		t.Fatal(err)
	}
	gitT(t, repo, "add", "a.go")
	gitT(t, repo, "commit", "-qm", "base")
	gitT(t, repo, "remote", "add", "origin", "https://forge.example/team/project")
	gitT(t, base, "clone", "-q", repo, clone)
	headBytes, err := exec.Command("git", "-C", repo, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	head := strings.TrimSpace(string(headBytes))
	h := handleIdentity(func(*http.Request) (map[string]svcInfo, error) {
		return map[string]svcInfo{"svc": {path: repo, indexedCommit: head}}, nil
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/identity?path="+clone, nil))
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	note, _ := out["freshness_note"].(string)
	if rec.Code != 200 || out["service"] != "svc" || note != "clone at "+head[:7]+", index at "+head[:7] {
		t.Fatalf("identity: %d %s", rec.Code, rec.Body.String())
	}
	const baseURL = "http://127.0.0.1:1"
	old := inProcessReg.Load()
	inProcessReg.Store(&inProcessRegistry{self: baseURL, services: func(context.Context) (map[string]svcInfo, error) {
		return map[string]svcInfo{"svc": {path: repo, indexedCommit: head}}, nil
	}})
	t.Cleanup(func() { inProcessReg.Store(old) })
	cand, found, e := resolveIndexedCandidateErr(&http.Client{}, baseURL, clone)
	if e != nil || !found || cand.name != "svc" {
		t.Fatalf("edit-hook clone identity: %+v %v %v", cand, found, e)
	}

}

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// LB-F324: route-oracle ingests a subject into a scratch store, reads its
// routes back as BuildFromStore composes them, and scores them against the
// oracle: route pairs on (method, normalized path), bound triples with the
// handler, per subject and pooled, misses capped.
func TestCmdRouteOracle(t *testing.T) {
	repo := t.TempDir()
	write := func(rel, s string) {
		t.Helper()
		p := filepath.Join(repo, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(s), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("src/main/java/p/OwnerController.java", `package p;
@Controller
@RequestMapping("/owners")
class OwnerController {
	@GetMapping("/{ownerId}")
	String show() { return ""; }
	@PostMapping("/new")
	String create() { return ""; }
	@GetMapping("/find")
	String find() { return ""; }
}`)
	write("src/test/java/p/TestController.java", `package p;
@RestController
class TestController {
	@GetMapping("/test-only")
	String t() { return ""; }
}`)
	oracle := routeOracleFile{Tool: "spring-boot-actuator", ToolVersion: "test", Subject: "fixture", SHA: "0000000", Licence: "Apache-2.0", Routes: []routeOracleRoute{
		{Method: "GET", Path: "/owners/{ownerId}", Handler: "OwnerController.show"},
		{Method: "POST", Path: "/owners/new", Handler: "OwnerController.create"},
		{Method: "GET", Path: "/owners/find", Handler: "OwnerController.lookup"}, // handler disagrees: route hit, bound miss
		{Method: "GET", Path: "/oups", Handler: "CrashController.triggerException"},
	}}
	ob, _ := json.Marshal(oracle)
	dir := t.TempDir()
	op := filepath.Join(dir, "oracle.json")
	if err := os.WriteFile(op, ob, 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "reading.json")
	var stdout, stderr bytes.Buffer
	code := mainRun([]string{"route-oracle", "-oracle", op, "-repo", repo, "-only", "src/main/**", "-out", out, "-commit", "abc", "-date", "2026-10-09"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	var r routeReading
	if err := json.Unmarshal(b, &r); err != nil {
		t.Fatal(err)
	}
	if len(r.Subjects) != 1 {
		t.Fatalf("subjects = %+v", r.Subjects)
	}
	s := r.Subjects[0]
	// fathom: 3 routes (the test controller is outside -only); oracle 4.
	if s.OracleRoutes != 4 || s.FathomRoutes != 3 || s.RouteHits != 3 || s.BoundHits != 2 {
		t.Fatalf("counts = %+v", s)
	}
	if s.RouteRecall != 0.75 || s.RoutePrecision != 1 || s.BoundRecall != 0.5 || s.BoundPrecision != 0.6667 {
		t.Fatalf("ratios = route %.4f/%.4f bound %.4f/%.4f", s.RouteRecall, s.RoutePrecision, s.BoundRecall, s.BoundPrecision)
	}
	if s.Handlers != 3 || s.HandlersIndexed != 3 {
		t.Fatalf("handlers indexed %d of %d, want 3 of 3 (each binds to an indexed JavaMethod)", s.HandlersIndexed, s.Handlers)
	}
	if s.Subject != "fixture" || s.SHA != "0000000" || r.Pooled.HandlersIndexed != 3 {
		t.Fatalf("subject %q sha %q pooled handlers %d", s.Subject, s.SHA, r.Pooled.HandlersIndexed)
	}
	if s.Verdict != "FAIL" || r.Verdict != "FAIL" || r.Pooled.BoundHits != 2 {
		t.Fatalf("verdicts = %s/%s pooled %+v", s.Verdict, r.Verdict, r.Pooled)
	}
	if strings.Join(s.Missed, ";") != "GET /oups CrashController.triggerException;GET /owners/find OwnerController.lookup" {
		t.Fatalf("missed = %v", s.Missed)
	}
	if strings.Join(s.Extra, ";") != "GET /owners/find OwnerController.find" {
		t.Fatalf("extra = %v", s.Extra)
	}
	if !strings.Contains(stdout.String(), "verdict FAIL") {
		t.Fatalf("stdout = %s", stdout.String())
	}
}

func TestScoreRoutes_PassAndCap(t *testing.T) {
	var truth []routeOracleRoute
	for i := 0; i < 40; i++ {
		truth = append(truth, routeOracleRoute{Method: "GET", Path: "/r/" + string(rune('a'+i%26)) + string(rune('a'+i/26)), Handler: "H.h"})
	}
	s := scoreRoutes(truth, truth[:37])
	if s.Verdict != "PASS" || s.BoundRecall != 0.925 || s.BoundPrecision != 1 {
		t.Fatalf("score = %+v", s)
	}
	if s = scoreRoutes(truth, nil); len(s.Missed) != routeOracleMissCap || s.MissedMore != 40-routeOracleMissCap || s.Verdict != "FAIL" {
		t.Fatalf("capped = %d more %d", len(s.Missed), s.MissedMore)
	}
	// A path parameter's spelling never splits a pair: {id} and {ownerId}
	// normalize alike.
	s = scoreRoutes([]routeOracleRoute{{Method: "GET", Path: "/o/{ownerId}", Handler: "O.s"}}, []routeOracleRoute{{Method: "GET", Path: "/o/{id}/", Handler: "O.s"}})
	if s.BoundHits != 1 {
		t.Fatalf("normalized = %+v", s)
	}
}

func TestCmdRouteOracle_Usage(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := mainRun([]string{"route-oracle", "-oracle", "a.json"}, &stdout, &stderr); code != 2 {
		t.Fatalf("exit %d, want 2 (usage)", code)
	}
}

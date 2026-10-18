package main

// verify_indexstale_test.go — O1160: a strict shortfall whose every
// unproven claimed row is a gone: row held back only by the index-at-HEAD
// precondition (a dirty tree, or a commit after the ingest) is index-stale:
// exit 2, the documented operational code. The rows stay Unknown (I44).

import (
	"encoding/json"
	"strings"
	"testing"
)

const indexStaleGonePlan = "- [x] driftpkg is imported nowhere {id: GN-1, gone: import gonefixture/driftpkg}\n" +
	"- [x] no module requires the retired one {id: GN-2, gone: module example.com/retired}\n"

func indexStaleFixture(t *testing.T, plan string) (*selfSkipGitRepo, string, string) {
	t.Helper()
	repo := newGoneFixtureRepo(t)
	pf := repo.writePlan("docs/plans/stale.md", plan)
	repo.commitAll("initial")
	return repo, pf, ingestGoneFixture(t, repo)
}

func verifyStrict(t *testing.T, repo *selfSkipGitRepo, pf, store string, extra ...string) (int, string, string) {
	t.Helper()
	args := append([]string{"verify", "--service", "gonefixture", "--plan", pf, "--store", store, "--strict"}, extra...)
	return runInDir(t, repo.root, args)
}

func TestVerifyIndexStale_CleanTreeExits0(t *testing.T) {
	repo, pf, store := indexStaleFixture(t, indexStaleGonePlan)
	if code, so, se := verifyStrict(t, repo, pf, store); code != 0 {
		t.Fatalf("clean tree: want 0, got %d\n%s\n%s", code, so, se)
	}
}

func TestVerifyIndexStale_DirtyTreeExits2WithAggregate(t *testing.T) {
	repo, pf, store := indexStaleFixture(t, indexStaleGonePlan)
	repo.writePlan("stray.txt", "x\n")
	code, so, se := verifyStrict(t, repo, pf, store)
	if code != 2 {
		t.Fatalf("dirty tree, only gone rows unproven: want 2, got %d\n%s\n%s", code, so, se)
	}
	for _, want := range []string{"index-stale", "uncommitted changes", "stray.txt", "2 claimed gone row", "commit or stash, then re-ingest"} {
		if !strings.Contains(so, want) {
			t.Errorf("aggregate line missing %q:\n%s", want, so)
		}
	}
}

func TestVerifyIndexStale_DirtyTreeJSONAggregate(t *testing.T) {
	repo, pf, store := indexStaleFixture(t, indexStaleGonePlan)
	repo.writePlan("stray.txt", "x\n")
	code, so, _ := verifyStrict(t, repo, pf, store, "--json")
	if code != 2 {
		t.Fatalf("want 2, got %d\n%s", code, so)
	}
	var doc struct {
		IndexStale struct {
			Reason    string `json:"reason"`
			FirstPath string `json:"first_dirty_path"`
			Rows      int    `json:"rows"`
			Remedy    string `json:"remedy"`
		} `json:"index_stale"`
	}
	if err := json.Unmarshal([]byte(so), &doc); err != nil {
		t.Fatalf("json: %v\n%s", err, so)
	}
	if doc.IndexStale.Rows != 2 || doc.IndexStale.FirstPath != "stray.txt" || doc.IndexStale.Remedy == "" || doc.IndexStale.Reason == "" {
		t.Errorf("index_stale = %+v", doc.IndexStale)
	}
}

func TestVerifyIndexStale_OtherFailureKeepsExit1(t *testing.T) {
	repo, pf, store := indexStaleFixture(t, indexStaleGonePlan+
		"- [x] ghost {id: GN-3, symbol: GhostSymbol, skip_test: true, skip: \"fixture\"}\n")
	repo.writePlan("stray.txt", "x\n")
	code, so, se := verifyStrict(t, repo, pf, store)
	if code != 1 {
		t.Fatalf("a genuinely failing non-gone row: want 1, got %d\n%s\n%s", code, so, se)
	}
}

func TestVerifyIndexStale_HeadMovedExits2(t *testing.T) {
	repo, pf, store := indexStaleFixture(t, indexStaleGonePlan)
	repo.run("commit", "-q", "--allow-empty", "-m", "after the ingest")
	code, so, se := verifyStrict(t, repo, pf, store)
	if code != 2 {
		t.Fatalf("HEAD one commit past the ingest: want 2, got %d\n%s\n%s", code, so, se)
	}
	if !strings.Contains(so, "index-stale") || !strings.Contains(so, "HEAD is now") {
		t.Errorf("aggregate must name both commits:\n%s", so)
	}
}

// The daemon path reaches the same verdict: same aggregate, same exit 2.
func TestVerifyIndexStale_DaemonParity(t *testing.T) {
	repo, pf, store := indexStaleFixture(t, indexStaleGonePlan)
	repo.writePlan("stray.txt", "x\n")
	url := startParityDaemon(t, store)
	args := func(extra ...string) []string {
		return append([]string{"verify", "--service", "gonefixture", "--plan", pf, "--strict", "--json", "--ledger", t.TempDir() + "/l.jsonl"}, extra...)
	}
	sc, sOut, _ := runInDir(t, repo.root, args("--store", store))
	dc, dOut, dErr := runInDir(t, repo.root, args("--url", url))
	if sc != 2 || dc != 2 {
		t.Fatalf("exit: store %d, daemon %d, want 2 and 2\n%s\n%s", sc, dc, dOut, dErr)
	}
	for name, out := range map[string]string{"store": sOut, "daemon": dOut} {
		var doc struct {
			IndexStale struct {
				Rows      int    `json:"rows"`
				FirstPath string `json:"first_dirty_path"`
			} `json:"index_stale"`
		}
		if err := json.Unmarshal([]byte(out), &doc); err != nil {
			t.Fatalf("%s json: %v\n%s", name, err, out)
		}
		if doc.IndexStale.Rows != 2 || doc.IndexStale.FirstPath != "stray.txt" {
			t.Errorf("%s index_stale = %+v", name, doc.IndexStale)
		}
	}
}

package main

import (
	"bytes"
	"fmt"
	"sync"
	"testing"
)

// TestObserveIdsSurviveConcurrentMint pins IT-10: two (or more) concurrent
// `fathom observe` calls must never mint the same O<N> id. observe.go's own
// prior comment pre-approved the flock upgrade path once advisory ids
// proved insufficient — this test is the proof they do, under real
// concurrency (N goroutines racing the SAME id-scan-then-append window,
// after a seed entry already exists so every racer is scanning/appending,
// not merely racing first-file-creation like
// TestObserve_ConcurrentFirstCreationLosesNothing already covers).
func TestObserveIdsSurviveConcurrentMint(t *testing.T) {
	root := t.TempDir()

	// Seed one entry so every goroutine below races the SCAN+APPEND path
	// (id.go's advisory-id danger zone), not the O_CREATE|O_EXCL first-file
	// race a different test already pins.
	var seedOut, seedErr bytes.Buffer
	if code := cmdObserve([]string{"--root", root, "--symptom", "seed"}, &seedOut, &seedErr); code != 0 {
		t.Fatalf("seed observe exit = %d, stderr: %s", code, seedErr.String())
	}

	const n = 12
	var wg sync.WaitGroup
	codes := make([]int, n)
	stderrs := make([]bytes.Buffer, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			var stdout bytes.Buffer
			codes[i] = cmdObserve([]string{"--root", root, "--symptom", fmt.Sprintf("concurrent mint %d", i)}, &stdout, &stderrs[i])
		}(i)
	}
	wg.Wait()

	for i := 0; i < n; i++ {
		if codes[i] != 0 {
			t.Fatalf("observe %d exit = %d, stderr: %s", i, codes[i], stderrs[i].String())
		}
	}

	content, existed, err := readObservations(root)
	if err != nil || !existed {
		t.Fatalf("readObservations: existed=%v err=%v", existed, err)
	}

	matches := observationEntryHeader.FindAllStringSubmatch(content, -1)
	if len(matches) != n+1 { // +1 for the seed entry
		t.Fatalf("%d entry headers, want %d (no entry may be lost)", len(matches), n+1)
	}
	seen := map[string]int{}
	for _, m := range matches {
		seen[m[1]]++
	}
	for id, count := range seen {
		if count > 1 {
			t.Errorf("id O%s minted %d times — concurrent observes collided on the same id", id, count)
		}
	}
	if len(seen) != n+1 {
		t.Errorf("%d distinct ids, want %d — some concurrent mint collided", len(seen), n+1)
	}
}

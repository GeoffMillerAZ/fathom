package extractor

import (
	"strconv"
	"testing"
	"time"
)

// A map of n keys built by n sets was n*n/2 string comparisons: 150,000 keys took
// 15 s in a doc-engine YAML file inside the 2 MiB limit. A map from docIndexMin
// keys indexes them.
func TestDocNodeSetIsNotQuadratic(t *testing.T) {
	n := &docNode{kind: docMap}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 200_000; i++ {
			n.set("k"+strconv.Itoa(i), &docNode{kind: docScalar, scalar: strconv.Itoa(i)})
		}
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("200,000 sets did not finish in 5 s")
	}
	if len(n.keys) != 200_000 || n.get("k123456").scalar != "123456" || n.get("missing") != nil {
		t.Fatalf("keys %d, get %v", len(n.keys), n.get("k123456"))
	}
}

func TestDocNodeKeepsFirstSeenOrderAndReplaces(t *testing.T) {
	for _, size := range []int{3, docIndexMin - 1, docIndexMin, 3 * docIndexMin} {
		n := &docNode{kind: docMap}
		for i := 0; i < size; i++ {
			n.set("k"+strconv.Itoa(i), &docNode{kind: docScalar, scalar: "a"})
		}
		n.set("k1", &docNode{kind: docScalar, scalar: "b"})
		n.set("new", &docNode{kind: docScalar, scalar: "c"})
		if len(n.keys) != size+1 || n.keys[1] != "k1" || n.keys[size] != "new" || n.get("k1").scalar != "b" || n.get("new").scalar != "c" || n.get("k0").scalar != "a" {
			t.Fatalf("size %d: keys %d, order %v", size, len(n.keys), n.keys[:min(3, len(n.keys))])
		}
	}
}

// A copy of a node (an alias target) shares the index. A key set on the copy is
// not found on the original, and the original's own keys still are.
func TestDocNodeCopiesDoNotCorruptEachOther(t *testing.T) {
	n := &docNode{kind: docMap}
	for i := 0; i < 2*docIndexMin; i++ {
		n.set("k"+strconv.Itoa(i), &docNode{kind: docScalar, scalar: "v"})
	}
	n.keys = n.keys[:len(n.keys):len(n.keys)] // a copy that grows reallocates, as it does once a map is full
	cp := *n
	cp.set("only-in-copy", &docNode{kind: docScalar, scalar: "c"})
	if cp.get("only-in-copy") == nil || n.get("only-in-copy") != nil {
		t.Fatalf("copy has it %v, original has it %v", cp.get("only-in-copy") != nil, n.get("only-in-copy") != nil)
	}
	if cp.get("k5") == nil || n.get("k5") == nil || n.get("k63") == nil {
		t.Fatal("a shared key is lost")
	}
}

// An index is a hint, and a hint that no longer matches the keys is not trusted:
// a copy whose keys were rebuilt in another order, with the index it shared, finds
// a key at a position that holds another.
func TestDocNodeIgnoresAStaleIndex(t *testing.T) {
	n := &docNode{kind: docMap}
	for i := 0; i < 2*docIndexMin; i++ {
		n.set("k"+strconv.Itoa(i), &docNode{kind: docScalar, scalar: strconv.Itoa(i)})
	}
	n.get("k1") // builds the index
	cp := *n
	cp.keys = make([]string, len(n.keys))
	for i, k := range n.keys {
		cp.keys[len(cp.keys)-1-i] = k // same length, another order
	}
	for _, k := range cp.keys {
		if i := cp.indexOf(k); i >= 0 && cp.keys[i] != k {
			t.Fatalf("indexOf(%q) = %d, which holds %q", k, i, cp.keys[i])
		}
	}
}

// Of a key a mapping repeats, the first wins, with or without an index.
func TestDocNodeFirstOfARepeatedKeyWins(t *testing.T) {
	for _, size := range []int{4, 3 * docIndexMin} {
		n := &docNode{kind: docMap}
		for i := 0; i < size; i++ {
			n.keys = append(n.keys, "k"+strconv.Itoa(i))
			n.vals = append(n.vals, &docNode{kind: docScalar, scalar: "first"})
		}
		n.keys = append(n.keys, "k2")
		n.vals = append(n.vals, &docNode{kind: docScalar, scalar: "second"})
		if got := n.get("k2"); got == nil || got.scalar != "first" {
			t.Errorf("size %d: a repeated key reads %+v, want the first", size, got)
		}
	}
}

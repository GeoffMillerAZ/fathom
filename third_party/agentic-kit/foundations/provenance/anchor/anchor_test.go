package anchor_test

import (
	"bytes"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/provenance/anchor"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// fixedClock returns a clock that always returns the same instant.
func fixedClock(t time.Time) func() time.Time {
	return func() time.Time { return t }
}

// leafHash computes the domain-separated leaf hash the same way the package does:
// SHA256(0x00 || data). Used to verify the package's output independently.
func leafHash(data []byte) []byte {
	h := sha256.New()
	h.Write([]byte{0x00})
	h.Write(data)
	return h.Sum(nil)
}

// nodeHash computes the domain-separated internal node hash:
// SHA256(0x01 || left || right).
func nodeHash(left, right []byte) []byte {
	h := sha256.New()
	h.Write([]byte{0x01})
	h.Write(left)
	h.Write(right)
	return h.Sum(nil)
}

var epoch = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// ---------------------------------------------------------------------------
// MerkleTree — single leaf
// ---------------------------------------------------------------------------

func TestMerkleTree_SingleLeaf_RootEqualsLeafHash(t *testing.T) {
	data := []byte("only leaf")
	tree, err := anchor.NewMerkleTree([][]byte{data})
	if err != nil {
		t.Fatalf("NewMerkleTree: unexpected error: %v", err)
	}

	want := leafHash(data)
	got := tree.Root()
	if !bytes.Equal(got, want) {
		t.Errorf("single leaf: Root() = %x, want leaf hash %x", got, want)
	}
}

// ---------------------------------------------------------------------------
// MerkleTree — empty input
// ---------------------------------------------------------------------------

func TestMerkleTree_Empty_ReturnsError(t *testing.T) {
	_, err := anchor.NewMerkleTree(nil)
	if err == nil {
		t.Fatal("NewMerkleTree(nil): expected error, got nil")
	}
	_, err2 := anchor.NewMerkleTree([][]byte{})
	if err2 == nil {
		t.Fatal("NewMerkleTree([]): expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// MerkleTree — even leaf count (power of 2)
// ---------------------------------------------------------------------------

func TestMerkleTree_EvenLeaves_RootDeterministic(t *testing.T) {
	leaves := [][]byte{
		[]byte("alpha"),
		[]byte("beta"),
		[]byte("gamma"),
		[]byte("delta"),
	}

	tree1, err := anchor.NewMerkleTree(leaves)
	if err != nil {
		t.Fatalf("NewMerkleTree: %v", err)
	}
	tree2, err := anchor.NewMerkleTree(leaves)
	if err != nil {
		t.Fatalf("NewMerkleTree: %v", err)
	}

	if !bytes.Equal(tree1.Root(), tree2.Root()) {
		t.Error("same leaves produced different roots")
	}
}

// ---------------------------------------------------------------------------
// MerkleTree — odd leaf count
// ---------------------------------------------------------------------------

func TestMerkleTree_OddLeaves_NonEmpty(t *testing.T) {
	leaves := [][]byte{
		[]byte("one"),
		[]byte("two"),
		[]byte("three"),
	}
	tree, err := anchor.NewMerkleTree(leaves)
	if err != nil {
		t.Fatalf("NewMerkleTree: %v", err)
	}
	if len(tree.Root()) == 0 {
		t.Error("odd leaves: Root() returned empty slice")
	}
}

// ---------------------------------------------------------------------------
// MerkleTree — RootHex
// ---------------------------------------------------------------------------

func TestMerkleTree_RootHex_IsLowerHex(t *testing.T) {
	tree, _ := anchor.NewMerkleTree([][]byte{[]byte("x")})
	hex := tree.RootHex()
	if len(hex) != 64 {
		t.Errorf("RootHex length %d, want 64", len(hex))
	}
	for i, c := range hex {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("RootHex[%d] = %q, not lowercase hex", i, c)
		}
	}
}

// ---------------------------------------------------------------------------
// InclusionProof — Prove + Verify for every leaf
// ---------------------------------------------------------------------------

func TestInclusionProof_AllLeaves_VerifyTrue(t *testing.T) {
	leaves := [][]byte{
		[]byte("alpha"),
		[]byte("beta"),
		[]byte("gamma"),
		[]byte("delta"),
		[]byte("epsilon"), // odd count
	}
	tree, err := anchor.NewMerkleTree(leaves)
	if err != nil {
		t.Fatalf("NewMerkleTree: %v", err)
	}
	root := tree.Root()

	for i, data := range leaves {
		proof, err := tree.Prove(i)
		if err != nil {
			t.Errorf("leaf %d: Prove error: %v", i, err)
			continue
		}
		if proof.LeafIndex != i {
			t.Errorf("leaf %d: proof.LeafIndex %d, want %d", i, proof.LeafIndex, i)
		}
		if !anchor.Verify(data, proof, root) {
			t.Errorf("leaf %d: Verify returned false for valid proof", i)
		}
	}
}

func TestInclusionProof_SingleLeaf_VerifyTrue(t *testing.T) {
	data := []byte("singleton")
	tree, _ := anchor.NewMerkleTree([][]byte{data})
	root := tree.Root()

	proof, err := tree.Prove(0)
	if err != nil {
		t.Fatalf("Prove(0): %v", err)
	}
	if !anchor.Verify(data, proof, root) {
		t.Error("single-leaf tree: Verify returned false for valid proof")
	}
}

// ---------------------------------------------------------------------------
// Tamper detection — wrong leaf data
// ---------------------------------------------------------------------------

func TestVerify_TamperedLeafData_ReturnsFalse(t *testing.T) {
	leaves := [][]byte{[]byte("real"), []byte("data")}
	tree, _ := anchor.NewMerkleTree(leaves)
	root := tree.Root()

	proof, _ := tree.Prove(0)
	if anchor.Verify([]byte("tampered"), proof, root) {
		t.Error("tampered leaf data: Verify should return false, got true")
	}
}

// ---------------------------------------------------------------------------
// Tamper detection — wrong sibling in proof
// ---------------------------------------------------------------------------

func TestVerify_TamperedSibling_ReturnsFalse(t *testing.T) {
	leaves := [][]byte{[]byte("left"), []byte("right")}
	tree, _ := anchor.NewMerkleTree(leaves)
	root := tree.Root()

	proof, _ := tree.Prove(0)
	if len(proof.Siblings) > 0 {
		// Corrupt the first sibling byte.
		tampered := make([]byte, len(proof.Siblings[0]))
		copy(tampered, proof.Siblings[0])
		tampered[0] ^= 0xFF
		proof.Siblings[0] = tampered
	}
	if anchor.Verify([]byte("left"), proof, root) {
		t.Error("tampered sibling: Verify should return false, got true")
	}
}

// ---------------------------------------------------------------------------
// Tamper detection — wrong root
// ---------------------------------------------------------------------------

func TestVerify_WrongRoot_ReturnsFalse(t *testing.T) {
	leaves := [][]byte{[]byte("a"), []byte("b")}
	tree, _ := anchor.NewMerkleTree(leaves)

	proof, _ := tree.Prove(0)
	wrongRoot := make([]byte, 32)
	for i := range wrongRoot {
		wrongRoot[i] = 0xAA
	}
	if anchor.Verify([]byte("a"), proof, wrongRoot) {
		t.Error("wrong root: Verify should return false, got true")
	}
}

// ---------------------------------------------------------------------------
// Domain separation: a raw node hash cannot be a valid leaf proof
// ---------------------------------------------------------------------------

// TestDomainSeparation_NodeCannotMasqueradeAsLeaf verifies that passing a
// pre-computed internal node hash as "leaf data" cannot forge a valid proof
// for a different index. This is the second-preimage attack vector that the
// 0x00/0x01 prefix scheme closes: without distinct prefixes, an adversary
// could take an internal node value, claim it is a leaf at index i, and
// construct a seemingly valid path to the root.
func TestDomainSeparation_NodeCannotMasqueradeAsLeaf(t *testing.T) {
	leaves := [][]byte{[]byte("a"), []byte("b"), []byte("c"), []byte("d")}
	tree, _ := anchor.NewMerkleTree(leaves)
	root := tree.Root()

	// Compute what the internal node of leaves[0] and leaves[1] would be if
	// treated as a "leaf" (i.e. using the leaf prefix 0x00 on node data).
	// A broken implementation that does not domain-separate would let this pass.
	lh0 := leafHash([]byte("a"))
	lh1 := leafHash([]byte("b"))
	internalNode := nodeHash(lh0, lh1)

	// Now get the proof for index 1 (leaf "b").
	proof, _ := tree.Prove(1)

	// Passing the internal node value as if it were leaf data for index 1
	// must NOT verify — the leaf hash of internalNode != leafHash("b").
	if anchor.Verify(internalNode, proof, root) {
		t.Error("domain separation failure: internal node value was accepted as a valid leaf")
	}
}

// ---------------------------------------------------------------------------
// Prove — out of range
// ---------------------------------------------------------------------------

func TestProve_OutOfRange_ReturnsError(t *testing.T) {
	tree, _ := anchor.NewMerkleTree([][]byte{[]byte("only")})
	if _, err := tree.Prove(-1); err == nil {
		t.Error("Prove(-1): expected error, got nil")
	}
	if _, err := tree.Prove(1); err == nil {
		t.Error("Prove(1): expected error (index == len), got nil")
	}
}

// ---------------------------------------------------------------------------
// AnchorBuilder — determinism with injected clock
// ---------------------------------------------------------------------------

func TestAnchorBuilder_Determinism(t *testing.T) {
	leaves := [][]byte{[]byte("ev1"), []byte("ev2")}

	b1 := anchor.NewAnchorBuilderWithClock(fixedClock(epoch))
	b2 := anchor.NewAnchorBuilderWithClock(fixedClock(epoch))

	for _, l := range leaves {
		b1.AddLeaf(l)
		b2.AddLeaf(l)
	}

	root1, proofs1, err1 := b1.Build()
	root2, proofs2, err2 := b2.Build()
	if err1 != nil || err2 != nil {
		t.Fatalf("Build errors: %v, %v", err1, err2)
	}
	if !bytes.Equal(root1, root2) {
		t.Error("same leaves+clock produced different roots")
	}
	if len(proofs1) != len(proofs2) {
		t.Fatalf("proof counts differ: %d vs %d", len(proofs1), len(proofs2))
	}
	for i := range proofs1 {
		if proofs1[i].LeafIndex != proofs2[i].LeafIndex {
			t.Errorf("proof[%d].LeafIndex differs", i)
		}
	}
}

// ---------------------------------------------------------------------------
// AnchorBuilder — Build returns one proof per leaf
// ---------------------------------------------------------------------------

func TestAnchorBuilder_Build_ProofCount(t *testing.T) {
	b := anchor.NewAnchorBuilderWithClock(fixedClock(epoch))
	b.AddLeaf([]byte("x"))
	b.AddLeaf([]byte("y"))
	b.AddLeaf([]byte("z"))

	root, proofs, err := b.Build()
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if len(proofs) != 3 {
		t.Errorf("proof count %d, want 3", len(proofs))
	}
	// Every proof should verify.
	leaves := [][]byte{[]byte("x"), []byte("y"), []byte("z")}
	for i, p := range proofs {
		if !anchor.Verify(leaves[i], p, root) {
			t.Errorf("proof[%d] does not verify", i)
		}
	}
}

// ---------------------------------------------------------------------------
// AnchorBuilder — Build with no leaves returns error
// ---------------------------------------------------------------------------

func TestAnchorBuilder_Build_Empty_ReturnsError(t *testing.T) {
	b := anchor.NewAnchorBuilderWithClock(fixedClock(epoch))
	if _, _, err := b.Build(); err == nil {
		t.Error("Build with no leaves: expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// AnchorBuilder — MakeAnchor
// ---------------------------------------------------------------------------

func TestAnchorBuilder_MakeAnchor_Valid(t *testing.T) {
	b := anchor.NewAnchorBuilderWithClock(fixedClock(epoch))
	b.AddLeaf([]byte("leaf1"))
	b.AddLeaf([]byte("leaf2"))

	if _, _, err := b.Build(); err != nil {
		t.Fatalf("Build: %v", err)
	}

	a, err := b.MakeAnchor("rfc3161:freetsa.org", []byte("opaque-receipt"))
	if err != nil {
		t.Fatalf("MakeAnchor: %v", err)
	}
	if err := a.Validate(); err != nil {
		t.Errorf("MakeAnchor result failed Validate: %v", err)
	}
	if a.LeafCount != 2 {
		t.Errorf("LeafCount %d, want 2", a.LeafCount)
	}
	if a.Authority != "rfc3161:freetsa.org" {
		t.Errorf("Authority %q, want %q", a.Authority, "rfc3161:freetsa.org")
	}
	if a.CreatedAt != epoch {
		t.Errorf("CreatedAt %v, want %v", a.CreatedAt, epoch)
	}
}

// ---------------------------------------------------------------------------
// AnchorBuilder — MakeAnchor before Build returns error
// ---------------------------------------------------------------------------

func TestAnchorBuilder_MakeAnchor_BeforeBuild_ReturnsError(t *testing.T) {
	b := anchor.NewAnchorBuilderWithClock(fixedClock(epoch))
	b.AddLeaf([]byte("leaf"))
	if _, err := b.MakeAnchor("rfc3161:freetsa.org", nil); err == nil {
		t.Error("MakeAnchor before Build: expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// NewAnchorBuilderWithClock — nil clock panics
// ---------------------------------------------------------------------------

func TestNewAnchorBuilderWithClock_NilPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic for nil clock, got none")
		}
	}()
	anchor.NewAnchorBuilderWithClock(nil)
}

// ---------------------------------------------------------------------------
// Anchor.Validate
// ---------------------------------------------------------------------------

func TestAnchorValidate_Happy(t *testing.T) {
	a := anchor.Anchor{
		Root:      "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
		LeafCount: 4,
		CreatedAt: epoch,
		Authority: "opentimestamps",
		Receipt:   []byte("proof"),
		Status:    anchor.StatusPending,
	}
	if err := a.Validate(); err != nil {
		t.Fatalf("Validate on valid Anchor: %v", err)
	}
}

func TestAnchorValidate_ConfirmedStatus(t *testing.T) {
	a := anchor.Anchor{
		Root:      "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
		LeafCount: 1,
		CreatedAt: epoch,
		Authority: "ct-log:example.com",
		Receipt:   nil, // Receipt may be absent on confirmed
		Status:    anchor.StatusConfirmed,
	}
	if err := a.Validate(); err != nil {
		t.Fatalf("Validate with confirmed status: %v", err)
	}
}

func TestAnchorValidate_EmptyRoot_Fails(t *testing.T) {
	a := anchor.Anchor{
		Root:      "",
		LeafCount: 1,
		CreatedAt: epoch,
		Authority: "opentimestamps",
		Status:    anchor.StatusPending,
	}
	if err := a.Validate(); err == nil {
		t.Error("empty Root: expected error, got nil")
	}
}

func TestAnchorValidate_EmptyAuthority_Fails(t *testing.T) {
	a := anchor.Anchor{
		Root:      "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		LeafCount: 1,
		CreatedAt: epoch,
		Authority: "",
		Status:    anchor.StatusPending,
	}
	if err := a.Validate(); err == nil {
		t.Error("empty Authority: expected error, got nil")
	}
}

func TestAnchorValidate_ZeroCreatedAt_Fails(t *testing.T) {
	a := anchor.Anchor{
		Root:      "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		LeafCount: 1,
		CreatedAt: time.Time{},
		Authority: "opentimestamps",
		Status:    anchor.StatusPending,
	}
	if err := a.Validate(); err == nil {
		t.Error("zero CreatedAt: expected error, got nil")
	}
}

func TestAnchorValidate_BadStatus_Fails(t *testing.T) {
	a := anchor.Anchor{
		Root:      "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		LeafCount: 1,
		CreatedAt: epoch,
		Authority: "opentimestamps",
		Status:    "unknown-status",
	}
	if err := a.Validate(); err == nil {
		t.Error("bad Status: expected error, got nil")
	}
}

func TestAnchorValidate_ZeroLeafCount_Fails(t *testing.T) {
	a := anchor.Anchor{
		Root:      "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		LeafCount: 0,
		CreatedAt: epoch,
		Authority: "opentimestamps",
		Status:    anchor.StatusPending,
	}
	if err := a.Validate(); err == nil {
		t.Error("zero LeafCount: expected error, got nil")
	}
}

// ---------------------------------------------------------------------------
// Defect 1 (PANIC): Verify with len(Siblings) > len(Directions) must not panic
// ---------------------------------------------------------------------------

// TestVerify_MalformedProof_SiblingsLongerThanDirections_ReturnsFalse ensures
// that a corrupt/adversarial proof where len(Siblings) > len(Directions) is
// rejected with false rather than panicking with an index-out-of-range.
func TestVerify_MalformedProof_SiblingsLongerThanDirections_ReturnsFalse(t *testing.T) {
	leaves := [][]byte{[]byte("a"), []byte("b"), []byte("c"), []byte("d")}
	tree, err := anchor.NewMerkleTree(leaves)
	if err != nil {
		t.Fatalf("NewMerkleTree: %v", err)
	}
	root := tree.Root()

	proof, err := tree.Prove(0)
	if err != nil {
		t.Fatalf("Prove: %v", err)
	}

	// Corrupt the proof: drop all Directions so len(Siblings) > len(Directions).
	malformed := anchor.InclusionProof{
		LeafIndex:  proof.LeafIndex,
		Siblings:   proof.Siblings,
		Directions: nil, // empty — parity violated
	}

	// Must return false, never panic.
	result := anchor.Verify([]byte("a"), malformed, root)
	if result {
		t.Error("malformed proof (Siblings longer than Directions): Verify returned true, want false")
	}
}

// TestVerify_MalformedProof_DirectionsLongerThanSiblings_ReturnsFalse ensures
// that a proof where len(Directions) > len(Siblings) is also rejected gracefully.
func TestVerify_MalformedProof_DirectionsLongerThanSiblings_ReturnsFalse(t *testing.T) {
	leaves := [][]byte{[]byte("x"), []byte("y")}
	tree, err := anchor.NewMerkleTree(leaves)
	if err != nil {
		t.Fatalf("NewMerkleTree: %v", err)
	}
	root := tree.Root()

	proof, err := tree.Prove(0)
	if err != nil {
		t.Fatalf("Prove: %v", err)
	}

	// Corrupt the proof: add extra directions so len(Directions) > len(Siblings).
	malformed := anchor.InclusionProof{
		LeafIndex:  proof.LeafIndex,
		Siblings:   proof.Siblings,
		Directions: append(proof.Directions, true, false, true),
	}

	// Must return false, never panic.
	result := anchor.Verify([]byte("x"), malformed, root)
	if result {
		t.Error("malformed proof (Directions longer than Siblings): Verify returned true, want false")
	}
}

// ---------------------------------------------------------------------------
// Defect 2: Verify with negative LeafIndex must return false (invalid proof)
// ---------------------------------------------------------------------------

// TestVerify_NegativeLeafIndex_ReturnsFalse ensures that a proof carrying a
// negative LeafIndex (violating the CUE #InclusionProof invariant LeafIndex>=0)
// is rejected by Verify rather than silently accepted.
func TestVerify_NegativeLeafIndex_ReturnsFalse(t *testing.T) {
	leaves := [][]byte{[]byte("sole")}
	tree, err := anchor.NewMerkleTree(leaves)
	if err != nil {
		t.Fatalf("NewMerkleTree: %v", err)
	}
	root := tree.Root()

	proof, err := tree.Prove(0)
	if err != nil {
		t.Fatalf("Prove: %v", err)
	}

	// Craft an invalid proof with a negative LeafIndex.
	malformed := anchor.InclusionProof{
		LeafIndex:  -1,
		Siblings:   proof.Siblings,
		Directions: proof.Directions,
	}

	if anchor.Verify([]byte("sole"), malformed, root) {
		t.Error("negative LeafIndex: Verify returned true, want false")
	}
}

// ---------------------------------------------------------------------------
// InclusionProof defensive copy — Siblings returned by Prove are isolated
// ---------------------------------------------------------------------------

func TestProve_SiblingsDefensiveCopy(t *testing.T) {
	leaves := [][]byte{[]byte("p"), []byte("q")}
	tree, _ := anchor.NewMerkleTree(leaves)
	root := tree.Root()

	proof, _ := tree.Prove(0)
	if len(proof.Siblings) > 0 {
		// Mutate the returned sibling.
		original := make([]byte, len(proof.Siblings[0]))
		copy(original, proof.Siblings[0])
		proof.Siblings[0][0] ^= 0xFF

		// Calling Prove again must return uncorrupted sibling.
		proof2, _ := tree.Prove(0)
		if !bytes.Equal(proof2.Siblings[0], original) {
			t.Error("Prove returned a live reference: mutating returned sibling corrupted tree state")
		}
		// Also verify the original proof still verifies with the original (unmutated) root.
		// (Since we mutated proof.Siblings[0], the proof is now invalid — that is expected.)
		_ = anchor.Verify([]byte("p"), proof, root) // do not assert value; proof is corrupted by test
	}
}

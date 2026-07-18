package anchor

import (
	"fmt"
	"time"
)

// AnchorBuilder accumulates evidence leaf data, builds the Merkle tree, and
// produces the Anchor record. The clock is injected at construction time so
// that all tree operations and the resulting Anchor's CreatedAt are
// deterministic in tests — inline time.Now() is never called.
//
// Typical usage:
//
//	b := anchor.NewAnchorBuilderWithClock(clock)
//	for _, evidenceHash := range batch {
//	    b.AddLeaf(evidenceHash)
//	}
//	root, proofs, err := b.Build()
//	anchor, err := b.MakeAnchor("rfc3161:freetsa.org", receiptBytes)
type AnchorBuilder struct {
	leaves [][]byte
	tree   *MerkleTree
	proofs []InclusionProof
	built  bool
	now    func() time.Time
}

// NewAnchorBuilder returns an AnchorBuilder that uses time.Now().UTC() as its
// clock. Use NewAnchorBuilderWithClock in tests or when reproducible output is
// required.
func NewAnchorBuilder() *AnchorBuilder {
	return &AnchorBuilder{now: func() time.Time { return time.Now().UTC() }}
}

// NewAnchorBuilderWithClock returns an AnchorBuilder that uses the supplied
// clock function. Panics if clock is nil (following the eventstore convention
// of making misuse a programming error at construction time rather than a
// subtle bug at use time).
func NewAnchorBuilderWithClock(clock func() time.Time) *AnchorBuilder {
	if clock == nil {
		panic("anchor: NewAnchorBuilderWithClock: clock must not be nil")
	}
	return &AnchorBuilder{now: clock}
}

// AddLeaf appends a leaf to the builder. The data slice is defensively copied
// so the caller may reuse or mutate the underlying array after this call.
// AddLeaf panics if called after Build() — builders are single-use.
func (b *AnchorBuilder) AddLeaf(data []byte) {
	if b.built {
		panic("anchor: AnchorBuilder.AddLeaf: called after Build()")
	}
	cp := make([]byte, len(data))
	copy(cp, data)
	b.leaves = append(b.leaves, cp)
}

// Build constructs the Merkle tree from all accumulated leaves, computes one
// inclusion proof per leaf, and returns the root hash and proofs.
//
// Returns an error if no leaves have been added.
// Build may only be called once; a second call returns an error.
func (b *AnchorBuilder) Build() (root []byte, proofs []InclusionProof, err error) {
	if b.built {
		return nil, nil, fmt.Errorf("anchor: AnchorBuilder.Build: already built")
	}
	if len(b.leaves) == 0 {
		return nil, nil, fmt.Errorf("anchor: AnchorBuilder.Build: no leaves added")
	}

	tree, err := NewMerkleTree(b.leaves)
	if err != nil {
		return nil, nil, fmt.Errorf("anchor: AnchorBuilder.Build: %w", err)
	}

	ps := make([]InclusionProof, len(b.leaves))
	for i := range b.leaves {
		p, err := tree.Prove(i)
		if err != nil {
			return nil, nil, fmt.Errorf("anchor: AnchorBuilder.Build: proof for leaf %d: %w", i, err)
		}
		ps[i] = p
	}

	b.tree = tree
	b.proofs = ps
	b.built = true

	return tree.Root(), ps, nil
}

// MakeAnchor creates an Anchor record from the built tree.
//
// authority identifies the external timestamp authority. receipt is the opaque
// proof bytes returned by that authority (may be nil for a pending anchor that
// has been submitted but not yet confirmed — call with StatusPending, update to
// StatusConfirmed when the receipt arrives; that update is an adapter concern).
//
// Returns an error if Build() has not yet been called successfully.
func (b *AnchorBuilder) MakeAnchor(authority string, receipt []byte) (Anchor, error) {
	if !b.built {
		return Anchor{}, fmt.Errorf("anchor: AnchorBuilder.MakeAnchor: must call Build() first")
	}

	var receiptCopy []byte
	if len(receipt) > 0 {
		receiptCopy = make([]byte, len(receipt))
		copy(receiptCopy, receipt)
	}

	status := StatusPending
	if len(receiptCopy) > 0 {
		status = StatusConfirmed
	}

	a := Anchor{
		Root:      b.tree.RootHex(),
		LeafCount: len(b.leaves),
		CreatedAt: b.now(),
		Authority: authority,
		Receipt:   receiptCopy,
		Status:    status,
	}
	return a, nil
}

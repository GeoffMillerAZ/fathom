// Package anchor provides cryptographic proof-of-existence for batches of
// evidence content-hashes. It builds a Merkle tree from a set of leaf
// byte-slices, computes per-leaf inclusion proofs, and records the tree root
// plus an opaque authority receipt in an Anchor value that can be persisted to
// an append-only ledger.
//
// Privacy design: each leaf carries exactly one evidence item's hash. The
// inclusion proof for leaf i reveals only the Merkle path from that leaf to
// the root — the siblings along the path are hashes, not raw evidence data —
// so any single item can be proven to have existed at anchor time without
// revealing the content (or even the hashes) of the other items in the batch.
//
// Trust design: the Anchor does not trust our own ledger for proof-of-time.
// It relies on an external authority (RFC 3161 timestamp authority, OpenTimestamps,
// a CT log, etc.) to bind the Merkle root to a wall-clock time. The SUBMISSION
// to that authority is an adapter concern — this package is pure domain logic.
//
// CUE contracts live in schema.cue; Go types here must stay in sync with that
// schema. hash.go provides the canonical hashing helpers; builder.go provides
// AnchorBuilder; validate.go provides Validate() methods.
//
// Concurrency: MerkleTree values are safe for concurrent read access once constructed; AnchorBuilder is not safe for concurrent use.
package anchor

import "time"

// AnchorStatus represents the lifecycle state of an Anchor.
//
// CUE mirror: #AnchorStatus in schema.cue.
type AnchorStatus string

const (
	// StatusPending means the root has been submitted to the external authority
	// but the authority's receipt has not yet been confirmed or received.
	StatusPending AnchorStatus = "pending"

	// StatusConfirmed means the external authority has returned a receipt that
	// has been stored in the Receipt field.
	StatusConfirmed AnchorStatus = "confirmed"
)

// InclusionProof proves that a single leaf was included in the Merkle tree
// that produced a known root. The proof is a path from the leaf to the root:
// at each level, the sibling hash and direction let the verifier recompute
// the parent node until the root is reconstructed.
//
// Privacy: the Siblings slice contains only hashes, not raw leaf data, so
// presenting a proof for leaf i does not reveal the content of any other leaf.
//
// CUE mirror: #InclusionProof in schema.cue.
type InclusionProof struct {
	// LeafIndex is the 0-based position of the proven leaf in the original
	// leaf slice.
	LeafIndex int

	// Siblings contains the sibling hash at each level of the Merkle tree,
	// from the leaf level up to (but not including) the root. len(Siblings)
	// equals the height of the tree (ceil(log2(padded leaf count))), or 0 for
	// a single-leaf tree.
	//
	// Each element is a raw 32-byte SHA-256 digest.
	Siblings [][]byte

	// Directions[i] is true when the current node at level i is the RIGHT
	// child (i.e. the sibling is on the LEFT); false when the current node
	// is the LEFT child (sibling is on the RIGHT). This mirrors standard
	// RFC 9162 / Certificate Transparency proof direction encoding.
	Directions []bool
}

// Anchor is a durable proof-of-existence record binding a Merkle root to an
// external authority's timestamp at a specific wall-clock instant. It carries
// enough information to:
//   - Recompute the root from any inclusion proof + leaf data.
//   - Identify which external authority anchored the root and when.
//   - Store the authority's opaque receipt for later offline verification.
//
// CUE mirror: #Anchor in schema.cue.
type Anchor struct {
	// Root is the lowercase hex-encoded SHA-256 Merkle root of the batch.
	// Must be non-empty; 64 hex characters for a standard SHA-256 root.
	Root string

	// LeafCount is the number of evidence items in the batch (>= 1).
	LeafCount int

	// CreatedAt is the UTC wall-clock time at which this Anchor was created
	// (injected via AnchorBuilder's clock — never time.Now() inline).
	CreatedAt time.Time

	// Authority identifies the external timestamp authority that anchored the
	// root (e.g. "rfc3161:freetsa.org", "opentimestamps", "ct-log:oak2024h2").
	// Must be non-empty.
	Authority string

	// Receipt is the opaque proof returned by the authority (e.g. a DER-encoded
	// RFC 3161 TimeStampToken, an OpenTimestamps proof file). May be nil when
	// Status is pending and the receipt has not yet been received.
	Receipt []byte

	// Status is the lifecycle state of this Anchor record.
	Status AnchorStatus
}

// MerkleTree is an immutable, domain-separated Merkle tree built from a
// fixed slice of leaf byte-slices. Once constructed it is safe for concurrent
// read access (Prove, Root, RootHex).
//
// Leaf hashing: SHA256(0x00 || data).
// Internal node hashing: SHA256(0x01 || left || right).
//
// The 0x00/0x01 domain-separation prefixes prevent second-preimage attacks
// where an adversary passes an internal node value as if it were leaf data.
// Without distinct prefixes, a node hash and a leaf hash live in the same
// hash space; with them, a value produced by the leaf hash function cannot
// be confused with a value produced by the node hash function.
//
// Odd-node rule: when a level has an odd number of nodes, the rightmost node
// is promoted unchanged to the next level (it is NOT duplicated). Duplication
// would create two leaves with identical hashes and break proof uniqueness.
// "Promote" means the unpaired node is carried up as-is to pair at the next
// level, which is deterministic and avoids introducing a phantom leaf.
type MerkleTree struct {
	// leaves stores the domain-separated leaf hashes (SHA256(0x00 || data)),
	// one per input leaf, in input order.
	leaves [][]byte

	// levels stores all tree levels bottom-up: levels[0] = leaf hashes,
	// levels[len-1] = single root hash.
	levels [][][]byte
}

// Root returns the Merkle root as a raw 32-byte slice. The returned slice is
// a defensive copy — callers may freely modify it.
func (m *MerkleTree) Root() []byte {
	if len(m.levels) == 0 {
		return nil
	}
	top := m.levels[len(m.levels)-1]
	if len(top) == 0 {
		return nil
	}
	out := make([]byte, len(top[0]))
	copy(out, top[0])
	return out
}

// RootHex returns the lowercase hex-encoded Merkle root (64 characters for
// SHA-256).
func (m *MerkleTree) RootHex() string {
	return toHex(m.Root())
}

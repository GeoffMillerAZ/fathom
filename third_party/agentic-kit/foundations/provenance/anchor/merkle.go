package anchor

import "fmt"

// NewMerkleTree constructs an immutable MerkleTree from the given leaf
// byte-slices. Each element in leaves is hashed with the domain-separated
// leaf hash (SHA256(0x00 || data)) before tree construction begins.
//
// At least one leaf is required; an error is returned for an empty slice.
//
// The tree is built bottom-up: starting from the leaf hash level, pairs of
// adjacent nodes are combined with hashNode. When a level has an odd number
// of nodes the rightmost node is promoted unchanged to the next level (NOT
// duplicated — see MerkleTree documentation for rationale). This continues
// until a single root hash remains.
//
// The input leaves slice and each element's bytes are defensively copied so
// callers may freely modify their slices after this call.
func NewMerkleTree(leaves [][]byte) (*MerkleTree, error) {
	if len(leaves) == 0 {
		return nil, fmt.Errorf("anchor: NewMerkleTree: at least one leaf is required")
	}

	// Build the leaf hash level (level 0).
	leafLevel := make([][]byte, len(leaves))
	for i, data := range leaves {
		leafLevel[i] = hashLeaf(data)
	}

	// Build all levels up to the root.
	levels := buildLevels(leafLevel)

	return &MerkleTree{
		leaves: copyLevel(leafLevel),
		levels: levels,
	}, nil
}

// buildLevels constructs all tree levels from the leaf level to the root.
// levels[0] is the leaf level; levels[len-1] contains the single root hash.
func buildLevels(leafLevel [][]byte) [][][]byte {
	var levels [][][]byte
	current := copyLevel(leafLevel)
	levels = append(levels, current)

	for len(current) > 1 {
		next := buildNextLevel(current)
		levels = append(levels, next)
		current = next
	}
	return levels
}

// buildNextLevel reduces current to the next level up the tree.
// Pairs of nodes are combined with hashNode; if the count is odd, the last
// node is promoted as-is (not duplicated).
func buildNextLevel(current [][]byte) [][]byte {
	n := len(current)
	pairs := n / 2
	extra := n % 2 // 1 if odd, 0 if even

	next := make([][]byte, pairs+extra)
	for i := 0; i < pairs; i++ {
		next[i] = hashNode(current[2*i], current[2*i+1])
	}
	if extra == 1 {
		// Promote the unpaired rightmost node unchanged.
		next[pairs] = make([]byte, len(current[n-1]))
		copy(next[pairs], current[n-1])
	}
	return next
}

// Prove returns an InclusionProof for the leaf at the given 0-based index.
//
// The proof contains, for each level from the leaf up to (but not including)
// the root, the sibling hash and a direction bit indicating whether the
// current node is the left or right child.
//
// Returns an error if index is out of range.
func (m *MerkleTree) Prove(index int) (InclusionProof, error) {
	if index < 0 || index >= len(m.leaves) {
		return InclusionProof{}, fmt.Errorf("anchor: Prove: index %d out of range [0, %d)", index, len(m.leaves))
	}

	// Single-leaf tree: the root IS the leaf hash; no siblings needed.
	if len(m.leaves) == 1 {
		return InclusionProof{
			LeafIndex:  index,
			Siblings:   nil,
			Directions: nil,
		}, nil
	}

	var siblings [][]byte
	var directions []bool

	pos := index
	for level := 0; level < len(m.levels)-1; level++ {
		nodes := m.levels[level]

		if pos < len(nodes) {
			isRight := pos%2 == 1
			var siblingIdx int
			if isRight {
				siblingIdx = pos - 1
			} else {
				siblingIdx = pos + 1
			}

			if siblingIdx < len(nodes) {
				// Sibling exists at this level.
				sibling := make([]byte, len(nodes[siblingIdx]))
				copy(sibling, nodes[siblingIdx])
				siblings = append(siblings, sibling)
				// directions[i] is true when the current node is the RIGHT child.
				directions = append(directions, isRight)
			} else {
				// No sibling (promoted node): the node was promoted as-is and
				// had no partner at this level. We record a nil placeholder so
				// the verifier knows to pass the current hash straight through.
				siblings = append(siblings, nil)
				directions = append(directions, false) // promoted: treated as left
			}
		}

		pos = pos / 2
	}

	return InclusionProof{
		LeafIndex:  index,
		Siblings:   siblings,
		Directions: directions,
	}, nil
}

// Verify recomputes the Merkle root from leafData and proof, and returns true
// if the recomputed root matches the supplied root.
//
// The verification algorithm:
//  1. Validate structural invariants of the proof (LeafIndex >= 0,
//     len(Siblings) == len(Directions)). Return false on any violation —
//     never panic on adversarial or corrupt input.
//  2. Hash the leaf: current = hashLeaf(leafData).
//  3. For each step in the proof: if the sibling is nil (promoted node),
//     keep current unchanged; otherwise combine current and sibling using
//     hashNode, in the order dictated by the direction bit.
//  4. Return bytes.Equal(current, root).
//
// Returns false for any tampered input (wrong leaf data, wrong sibling, wrong
// root, wrong direction) or any structurally invalid proof.
func Verify(leafData []byte, proof InclusionProof, root []byte) bool {
	// Structural guard: enforce the CUE invariant LeafIndex >= 0.
	if proof.LeafIndex < 0 {
		return false
	}
	// Structural guard: Siblings and Directions must be the same length.
	// An adversary or deserialisation bug can produce a mismatched proof;
	// we must return false rather than panic on index out of range.
	if len(proof.Siblings) != len(proof.Directions) {
		return false
	}

	current := hashLeaf(leafData)

	for i, sibling := range proof.Siblings {
		if sibling == nil {
			// Promoted node: no sibling at this level; current passes through.
			continue
		}
		isRight := proof.Directions[i]
		if isRight {
			// Current node is the right child: parent = hash(sibling, current).
			current = hashNode(sibling, current)
		} else {
			// Current node is the left child: parent = hash(current, sibling).
			current = hashNode(current, sibling)
		}
	}

	return equalBytes(current, root)
}

// equalBytes returns true iff a and b have equal length and identical content.
// It avoids importing bytes just for this comparison.
func equalBytes(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// copyLevel returns a deep copy of a level (slice of hash slices).
func copyLevel(level [][]byte) [][]byte {
	out := make([][]byte, len(level))
	for i, h := range level {
		out[i] = make([]byte, len(h))
		copy(out[i], h)
	}
	return out
}

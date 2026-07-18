package anchor

import (
	"crypto/sha256"
	"encoding/hex"
)

// hashLeaf computes the domain-separated leaf hash:
//
//	SHA256(0x00 || data)
//
// The 0x00 prefix is the "leaf" domain tag. It ensures that a value produced
// by hashLeaf can never equal a value produced by hashNode, which uses the
// 0x01 prefix. Without this separation an adversary who knows an internal
// node value N = hashNode(L, R) could claim "N is a leaf" and potentially
// forge an inclusion proof for a fabricated leaf — a second-preimage attack
// enabled by operating in a single hash domain.
func hashLeaf(data []byte) []byte {
	h := sha256.New()
	h.Write([]byte{0x00})
	h.Write(data)
	return h.Sum(nil)
}

// hashNode computes the domain-separated internal node hash:
//
//	SHA256(0x01 || left || right)
//
// The 0x01 prefix is the "node" domain tag. It ensures the output cannot be
// confused with a leaf hash (see hashLeaf). Both left and right must be
// 32-byte SHA-256 digests.
func hashNode(left, right []byte) []byte {
	h := sha256.New()
	h.Write([]byte{0x01})
	h.Write(left)
	h.Write(right)
	return h.Sum(nil)
}

// toHex encodes b as a lowercase hexadecimal string.
func toHex(b []byte) string {
	return hex.EncodeToString(b)
}

package main

import "testing"

func TestTreeIdentityHasUnambiguousBinaryBoundaries(t *testing.T) {
	one := []snapshotFile{{path: "appkit/a", body: []byte("x\x00appkit/b\x00y")}}
	two := []snapshotFile{{path: "appkit/a", body: []byte("x")}, {path: "appkit/b", body: []byte("y")}}
	if treeHash(one) == treeHash(two) {
		t.Fatal("distinct file inventories share a tree identity")
	}
	if treeHash(one) != treeHash(one) {
		t.Fatal("tree identity is unstable")
	}
}

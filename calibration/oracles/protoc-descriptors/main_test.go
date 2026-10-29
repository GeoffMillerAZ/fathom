package main

import "testing"

func TestReferenceDestinationKeepsCompilerContexts(t *testing.T) {
	contexts := map[string]map[string][2]string{"one.proto": {".shared.Dup": {"a.proto", "Dup"}}, "two.proto": {".shared.Dup": {"b.proto", "Dup"}}}
	fallback := map[string][2]string{".shared.Dup": {"b.proto", "Dup"}}
	for _, tc := range []struct{ file, want string }{{"one.proto", "a.proto"}, {"two.proto", "b.proto"}} {
		got, ok := referenceDestination(tc.file, ".shared.Dup", contexts, fallback)
		if !ok || got != [2]string{tc.want, "Dup"} {
			t.Errorf("%s destination = %v, %v; want %s", tc.file, got, ok, tc.want)
		}
	}
	if _, ok := referenceDestination("unattested.proto", ".shared.Dup", contexts, fallback); ok {
		t.Error("native lookup used global fallback without compiler context")
	}
	got, ok := referenceDestination("image.proto", ".shared.Dup", nil, fallback)
	if !ok || got != [2]string{"b.proto", "Dup"} {
		t.Error("jointly compiled image fallback changed")
	}
}

func TestDescriptorFileUsesCompilerRootPrecedence(t *testing.T) {
	files := map[string]bool{"short/common.proto": true, "longer/root/common.proto": true}
	got, ok := descriptorFile("common.proto", []string{"short", "longer/root"}, files)
	if !ok || got != "short/common.proto" {
		t.Fatalf("descriptor source = %q, %v; want own compiler root first", got, ok)
	}
}

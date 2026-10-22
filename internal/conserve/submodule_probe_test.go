package conserve

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNewInputGeneratedProbeTreatsSubmoduleAsAbsent(t *testing.T) {
	r := newInputTestRepo(t)
	r.write("a.go", "package a\n")
	r.commitAll("initial")
	first := r.headSHA()
	r.run("update-index", "--add", "--cacheinfo", "160000,"+first+",nested")
	r.run("commit", "-qm", "add gitlink")
	base := r.headSHA()
	if err := os.Mkdir(filepath.Join(r.root, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	r.run("update-index", "--cacheinfo", "160000,"+base+",nested")
	r.run("commit", "-qm", "bump gitlink")
	in, err := NewInput(context.Background(), r.root, base, nil, "svc", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(in.ChangedFiles) != 1 || in.ChangedFiles[0] != "nested" {
		t.Fatalf("changed paths = %v", in.ChangedFiles)
	}
	if in.HeadGenerated == nil {
		t.Fatal("missing streaming probe")
	}
	if generated, ok, err := in.HeadGenerated("nested"); err != nil || ok || generated {
		t.Fatalf("gitlink probe = %v, %v, %v", generated, ok, err)
	}
	if b, ok, err := in.HeadFile("nested"); err != nil || ok || b != nil {
		t.Fatalf("gitlink read = %q, %v, %v", b, ok, err)
	}
}

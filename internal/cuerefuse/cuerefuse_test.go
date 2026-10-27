package cuerefuse_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Every non-test cue/load configuration in the module must carry the refusing
// registry (or go through fusion's cueTypedLoadConfig helper): a bare
// load.Config{} builds the default registry and names the public host.
func TestEveryCUELoadConfigSetsRefusingRegistry(t *testing.T) {
	root := filepath.Join("..", "..")
	found := 0
	err := filepath.WalkDir(filepath.Join(root, "internal"), func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return err
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		src := string(b)
		if strings.Contains(src, "load.Instances(") && !strings.Contains(src, "cuelang.org/go/cue/load") {
			return nil
		}
		for _, line := range strings.Split(src, "\n") {
			if strings.Contains(line, "load.Config{") {
				found++
				if !strings.Contains(line, "Registry:") && !strings.Contains(p, "cueregistry.go") {
					t.Errorf("%s: load.Config without Registry: %s", p, strings.TrimSpace(line))
				}
			}
			if strings.Contains(line, "load.Instances(") && strings.Contains(p, "typedcue.go") && !strings.Contains(line, "cueTypedLoadConfig(") {
				t.Errorf("%s: load.Instances bypasses cueTypedLoadConfig: %s", p, strings.TrimSpace(line))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if found == 0 {
		t.Fatal("no load.Config found: test is vacuous")
	}
}

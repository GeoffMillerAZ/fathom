package main

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed default-policy.json
var defaultPolicy []byte

//go:embed schema.cue
var policySchema []byte

//go:embed lefthook.yml
var lefthookFragment string

// scaffold writes a new, private directory and never changes repository hooks.
// os.Mkdir refuses an existing path, including an existing empty directory.
func scaffold(dir string) error {
	if dir == "" {
		return fmt.Errorf("scaffold directory is empty")
	}
	if err := os.Mkdir(dir, 0o700); err != nil {
		return fmt.Errorf("create scaffold directory: %w", err)
	}
	for _, asset := range []struct {
		name string
		data []byte
	}{
		{"policy.json", defaultPolicy},
		{"schema.cue", policySchema},
		{"lefthook.yml", []byte(lefthookFragment)},
	} {
		path := filepath.Join(dir, asset.name)
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			return fmt.Errorf("create %s: %w", asset.name, err)
		}
		if _, err := file.Write(asset.data); err != nil {
			file.Close()
			return fmt.Errorf("write %s: %w", asset.name, err)
		}
		if err := file.Close(); err != nil {
			return fmt.Errorf("close %s: %w", asset.name, err)
		}
	}
	return nil
}

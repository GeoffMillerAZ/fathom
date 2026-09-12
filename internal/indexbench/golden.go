package indexbench

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// golden.go is the one place that reads and writes this package's committed
// goldens, so "how do I regenerate this?" has exactly one answer:
//
//	go test ./internal/indexbench/... -update
//
// Every failure message in the package prints that command. A golden a
// reader cannot regenerate in one command is a golden somebody eventually
// edits by hand to make the build green (BP-21 will need exactly this when
// obligations ranking changes).
const RegenerateCommand = "go test ./internal/indexbench/... -update"

// Paths of the committed goldens, relative to this package directory.
const (
	fauxtownGoldenPath = "testdata/fauxtown.golden.json"
	replayGoldenPath   = "replay/golden.json"
)

// packageDir resolves this package's directory from the compiled-in source
// path, so goldens are found and rewritten no matter which directory the
// test binary runs in.
func packageDir() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		wd, _ := os.Getwd()
		return wd
	}
	return filepath.Dir(file)
}

// marshalGolden renders v the way every golden in this package is stored:
// two-space indent, no HTML escaping, one trailing newline. Byte-identical
// for byte-identical input.
func marshalGolden(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// readGolden decodes the golden at rel into v.
func readGolden(rel string, v any) error {
	data, err := os.ReadFile(filepath.Join(packageDir(), rel))
	if err != nil {
		return fmt.Errorf("indexbench: read golden %s: %w (regenerate: %s)", rel, err, RegenerateCommand)
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("indexbench: parse golden %s: %w", rel, err)
	}
	return nil
}

// writeGolden rewrites the golden at rel from v.
func writeGolden(rel string, v any) error {
	data, err := marshalGolden(v)
	if err != nil {
		return err
	}
	p := filepath.Join(packageDir(), rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}

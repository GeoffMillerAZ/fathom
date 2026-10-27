// Command preseedscan reads static module calls for calibration preparation.
// Its HCL parser supplies only module locations to OpenTofu's native oracle;
// it supplies no definitions, references, or truth to the grade.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
)

type moduleCall struct {
	Name    string `json:"name"`
	Source  string `json:"source"`
	Version string `json:"version,omitempty"`
}

func literalString(block *hclsyntax.Block, name string, required bool) (string, error) {
	attr, ok := block.Body.Attributes[name]
	if !ok {
		if required {
			return "", fmt.Errorf("module %q lacks %s", block.Labels[0], name)
		}
		return "", nil
	}
	value, diagnostics := attr.Expr.Value(nil)
	if diagnostics.HasErrors() || !value.IsKnown() || value.IsNull() || value.Type().FriendlyName() != "string" {
		return "", fmt.Errorf("module %q has nonliteral %s", block.Labels[0], name)
	}
	return value.AsString(), nil
}

func scanDir(dir string) ([]moduleCall, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	calls := []moduleCall{}
	seen := map[string]bool{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".tf") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		parsed, diagnostics := hclsyntax.ParseConfig(data, path, hcl.Pos{Line: 1, Column: 1})
		if diagnostics.HasErrors() {
			return nil, fmt.Errorf("parse %s: %s", path, diagnostics.Error())
		}
		body, ok := parsed.Body.(*hclsyntax.Body)
		if !ok {
			return nil, fmt.Errorf("parse %s: unexpected HCL body type", path)
		}
		for _, block := range body.Blocks {
			if block.Type != "module" {
				continue
			}
			if len(block.Labels) != 1 || seen[block.Labels[0]] {
				return nil, fmt.Errorf("duplicate or malformed module block in %s", path)
			}
			seen[block.Labels[0]] = true
			source, err := literalString(block, "source", true)
			if err != nil {
				return nil, err
			}
			version, err := literalString(block, "version", false)
			if err != nil {
				return nil, err
			}
			calls = append(calls, moduleCall{Name: block.Labels[0], Source: source, Version: version})
		}
	}
	sort.Slice(calls, func(i, j int) bool { return calls[i].Name < calls[j].Name })
	return calls, nil
}

func main() {
	dir := flag.String("dir", "", "one module directory")
	flag.Parse()
	if *dir == "" {
		fmt.Fprintln(os.Stderr, "usage: preseedscan -dir <module directory>")
		os.Exit(2)
	}
	calls, err := scanDir(*dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := json.NewEncoder(os.Stdout).Encode(calls); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

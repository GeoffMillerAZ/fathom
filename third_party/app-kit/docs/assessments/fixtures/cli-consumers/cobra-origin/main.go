// Command cobra-origin prepares an isolated, unpublished dependency experiment.
// It never edits the module cache, root module graph, or a consumer.
package main

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"go/format"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/mod/sumdb/dirhash"
)

//go:embed *.go.txt
var additions embed.FS

type moduleSpec struct{ Name, Version, Sum, Dir, Addition string }
type moduleReceipt struct {
	Name         string            `json:"name"`
	Version      string            `json:"version"`
	InputSum     string            `json:"inputSum"`
	CandidateSum string            `json:"candidateSum"`
	Files        map[string]string `json:"files"`
	Changed      []string          `json:"changed"`
}

func main() {
	cobraDir := flag.String("cobra", "", "exact cached Cobra v1.10.2 source")
	pflagDir := flag.String("pflag", "", "exact cached pflag v1.0.10 source")
	out := flag.String("out", "", "new absolute output directory outside the checkout")
	flag.Parse()
	if err := prepare(*cobraDir, *pflagDir, *out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func prepare(cobraDir, pflagDir, out string) error {
	if !filepath.IsAbs(out) || flag.NArg() != 0 {
		return fmt.Errorf("require absolute fresh --out and no positional arguments")
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(out))
	if err != nil {
		return err
	}
	out = filepath.Join(parent, filepath.Base(out))
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	cwd, err = filepath.EvalSymlinks(cwd)
	if err != nil {
		return err
	}
	if within(cwd, out) {
		return fmt.Errorf("output must be outside working checkout")
	}
	if _, err := os.Lstat(out); !os.IsNotExist(err) {
		return fmt.Errorf("output must not exist: %s", out)
	}
	specs := []moduleSpec{
		{"github.com/spf13/cobra", "v1.10.2", "h1:DMTTonx5m65Ic0GOoRY2c16WCbHxOOw6xxezuLaBpcU=", cobraDir, "cobra_invocation.go.txt"},
		{"github.com/spf13/pflag", "v1.0.10", "h1:4EBh2KAYBwaONj6b2Ye1GiHfwjqyROoF4RwYO+vPwFk=", pflagDir, "pflag_origin.go.txt"},
	}
	var receipts []moduleReceipt
	var trees []map[string][]byte
	for _, spec := range specs {
		source, err := filepath.EvalSymlinks(spec.Dir)
		if err != nil || spec.Dir == "" {
			return fmt.Errorf("invalid source %q", spec.Dir)
		}
		if within(source, out) || within(out, source) {
			return fmt.Errorf("source/output overlap")
		}
		tree, err := readTree(source)
		if err != nil {
			return err
		}
		sum, err := treeSum(spec, tree)
		if err != nil {
			return err
		}
		if sum != spec.Sum {
			return fmt.Errorf("%s source mismatch: got %s, want %s", spec.Name, sum, spec.Sum)
		}
		before := hashes(tree)
		if err := patch(spec, tree); err != nil {
			return err
		}
		after := hashes(tree)
		var changed []string
		for name, digest := range after {
			if before[name] != digest {
				changed = append(changed, name)
			}
		}
		sort.Strings(changed)
		candidate, err := treeSum(spec, tree)
		if err != nil {
			return err
		}
		receipts = append(receipts, moduleReceipt{spec.Name, spec.Version, sum, candidate, after, changed})
		trees = append(trees, tree)
	}
	// Only write after both complete source snapshots verify and every patch matches.
	if err := os.Mkdir(out, 0700); err != nil {
		return err
	}
	for i, tree := range trees {
		for name, data := range tree {
			path := filepath.Join(out, filepath.Base(specs[i].Name), filepath.FromSlash(name))
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				return err
			}
			if err := os.WriteFile(path, data, 0600); err != nil {
				return err
			}
		}
	}
	receipt, err := json.MarshalIndent(receipts, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(out, "receipt.json"), append(receipt, '\n'), 0600)
}

func within(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func readTree(root string) (map[string][]byte, error) {
	tree := map[string][]byte{}
	var total int64
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || info.Size() > 4<<20 || len(tree) >= 500 {
			return fmt.Errorf("unsafe or excessive module file: %s", path)
		}
		total += info.Size()
		if total > 20<<20 {
			return fmt.Errorf("module exceeds snapshot budget")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		tree[filepath.ToSlash(rel)] = data
		return nil
	})
	return tree, err
}

func treeSum(spec moduleSpec, tree map[string][]byte) (string, error) {
	prefix := spec.Name + "@" + spec.Version + "/"
	names := make([]string, 0, len(tree))
	for name := range tree {
		names = append(names, prefix+name)
	}
	return dirhash.Hash1(names, func(name string) (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(tree[strings.TrimPrefix(name, prefix)])), nil
	})
}

func hashes(tree map[string][]byte) map[string]string {
	result := map[string]string{}
	for name, data := range tree {
		result[name] = fmt.Sprintf("%x", sha256.Sum256(data))
	}
	return result
}

func patch(spec moduleSpec, tree map[string][]byte) error {
	replace := func(file, old, new string) error {
		if bytes.Count(tree[file], []byte(old)) != 1 {
			return fmt.Errorf("patch context mismatch: %s", file)
		}
		tree[file] = bytes.Replace(tree[file], []byte(old), []byte(new), 1)
		return nil
	}
	if strings.HasSuffix(spec.Name, "/cobra") {
		if err := replace("command.go", "type Command struct {", "type Command struct {\n\t// Experimental, inherited from the nearest explicit ancestor; default off.\n\tinvocationErrorOrigins *bool\n"); err != nil {
			return err
		}
		for file, count := range map[string]int{"args.go": 7} {
			lines := strings.Split(string(tree[file]), "\n")
			found := 0
			for i, line := range lines {
				if !strings.Contains(line, "return fmt.Errorf(") {
					continue
				}
				kind := "arguments"
				if found == 0 {
					kind = "command"
				}
				lines[i] = strings.Replace(line, "return fmt.Errorf(", "return invocationError(cmd, \""+kind+"\", fmt.Errorf(", 1) + ")"
				found++
			}
			if found != count {
				return fmt.Errorf("%s: expected %d constructors, found %d", file, count, found)
			}
			tree[file] = []byte(strings.Join(lines, "\n"))
		}
		// Keep command-free group helpers raw; apply policy at the command boundary.
		for _, call := range []string{
			"validateRequiredFlagGroups(groupStatus)",
			"validateOneRequiredFlagGroups(oneRequiredGroupStatus)",
			"validateExclusiveFlagGroups(mutuallyExclusiveGroupStatus)",
		} {
			old := "if err := " + call + "; err != nil {\n\t\treturn err"
			if err := replace("flag_groups.go", old, strings.TrimSuffix(old, "return err")+"return invocationError(c, \"flag-group\", err)"); err != nil {
				return err
			}
		}
		old := "return fmt.Errorf(`required flag(s) \"%s\" not set`, strings.Join(missingFlagNames, `\", \"`))"
		if err := replace("command.go", old, "return invocationError(c, \"required-flags\", "+strings.TrimPrefix(old, "return ")+")"); err != nil {
			return err
		}
		if err := replace("command.go", "return c.FlagErrorFunc()(c, err)", "result := c.FlagErrorFunc()(c, err)\n\t\tif result == err { return invocationFlagError(c, err) }\n\t\treturn result"); err != nil {
			return err
		}
		if err := replace("command.go", "if err := c.ParseFlags(flags); err != nil {\n\t\t\treturn nil, args, err", "if err := c.ParseFlags(flags); err != nil {\n\t\t\treturn nil, args, invocationFlagError(c, err)"); err != nil {
			return err
		}
	} else {
		if err := replace("errors.go", "cause error\n}", "cause error\n\tbuiltin bool\n}"); err != nil {
			return err
		}
		if err := replace("flag.go", "err := flag.Value.Set(value)", "parser := flag.Value\n\tbuiltin := builtinConversionValue(parser)\n\terr := parser.Set(value)"); err != nil {
			return err
		}
		if err := replace("flag.go", "cause: err,", "cause: err,\n\t\t\tbuiltin: builtin,"); err != nil {
			return err
		}
	}
	data, err := additions.ReadFile(spec.Addition)
	if err != nil {
		return err
	}
	name := strings.TrimSuffix(spec.Addition, ".txt")
	if _, ok := tree[name]; ok {
		return fmt.Errorf("addition already exists: %s", name)
	}
	tree[name] = data
	for file, data := range tree {
		if !strings.HasSuffix(file, ".go") {
			continue
		}
		// Format only touched files; unrelated upstream bytes stay exact.
		if file != name && file != "args.go" && file != "flag_groups.go" && file != "command.go" && file != "errors.go" && file != "flag.go" {
			continue
		}
		formatted, err := format.Source(data)
		if err != nil {
			return fmt.Errorf("%s: %w", file, err)
		}
		tree[file] = formatted
	}
	return nil
}

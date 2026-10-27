// Counts the measured extractor's ungraded pairs; this is not an oracle.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/fusion"
)

func count(root string, files []string) (int, error) {
	if len(files) > 4096 {
		return 0, errors.New("membership cap")
	}
	doc, err := extractor.NewDocDialectExtractor("json-schema")
	if err != nil {
		return 0, err
	}
	var docs []fusion.DocFile
	total := int64(0)
	seen := map[string]bool{}
	for _, name := range files {
		if name == "." || filepath.IsAbs(name) || filepath.Clean(name) != name || name == ".." || strings.HasPrefix(name, ".."+string(filepath.Separator)) || seen[name] {
			return 0, errors.New("invalid membership")
		}
		seen[name] = true
		path := filepath.Join(root, name)
		canonical, err := filepath.EvalSymlinks(path)
		if err != nil {
			return 0, err
		}
		if canonical != path {
			return 0, errors.New("symlink input")
		}
		info, err := os.Lstat(path)
		if err != nil {
			return 0, err
		}
		if !info.Mode().IsRegular() || info.Size() > 2*1024*1024 {
			return 0, errors.New("input type or cap")
		}
		total += info.Size()
		if total > 512*1024*1024 {
			return 0, errors.New("population cap")
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return 0, err
		}
		result, err := doc.ExtractDoc("subject", name, raw)
		if err != nil {
			return 0, err
		}
		docs = append(docs, fusion.DocFile{Path: name, Result: result})
	}
	pairs := map[[3]string]bool{}
	for _, x := range append(fusion.ResolveDocXrefs(docs), fusion.ResolveIdentityXrefs(docs)...) {
		pairs[[3]string{x.FromFile, x.From, x.ToFile}] = true
	}
	return len(pairs), nil
}

func main() {
	root := flag.String("subject", "", "pinned subject root")
	membership := flag.String("membership", "", "harness unmatched JSON with tenancy_files")
	flag.Parse()
	if *root == "" || *membership == "" {
		fmt.Fprintln(os.Stderr, "subject and membership required")
		os.Exit(2)
	}
	absolute, err := filepath.Abs(*root)
	if err != nil {
		panic(err)
	}
	absolute, err = filepath.EvalSymlinks(absolute)
	if err != nil {
		panic(err)
	}
	source, err := os.Open(*membership)
	if err != nil {
		panic(err)
	}
	raw, err := io.ReadAll(io.LimitReader(source, 64*1024*1024+1))
	source.Close()
	if err != nil || len(raw) > 64*1024*1024 {
		panic("membership read cap")
	}
	var input struct {
		Files []string `json:"tenancy_files"`
	}
	if err = json.Unmarshal(raw, &input); err != nil || input.Files == nil {
		panic("invalid membership JSON")
	}
	n, err := count(absolute, input.Files)
	if err != nil {
		panic(err)
	}
	hash := sha256.Sum256(raw)
	if err = json.NewEncoder(os.Stdout).Encode(map[string]any{"files": len(input.Files), "fathom_pairs": n, "membership_sha256": hex.EncodeToString(hash[:]), "graded": false, "unit": "source file, root entity, target file"}); err != nil {
		panic(err)
	}
}

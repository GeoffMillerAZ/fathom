// Snapshot the live dispatch/extraction of every existing YAML dialect fixture.
// This is regression equivalence, never an independent language-level oracle.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/fusion"
)

type row struct {
	Path    string `json:"path"`
	Source  string `json:"source_sha256"`
	Kind    string `json:"tool_kind"`
	Skipped string `json:"skipped,omitempty"`
	Error   string `json:"error,omitempty"`
	Nodes   int    `json:"nodes"`
	Digest  string `json:"nodes_sha256"`
}

func main() {
	root := flag.String("root", ".", "fixture repository")
	flag.Parse()
	var files []string
	err := filepath.WalkDir(filepath.Join(*root, "internal/extractor/dialects"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(*root, path)
		rel = filepath.ToSlash(rel)
		if strings.Contains(rel, "/azure-pipelines/") || !strings.Contains(rel, "/testdata/") {
			return nil
		}
		if ext := filepath.Ext(rel); ext == ".yml" || ext == ".yaml" {
			files = append(files, rel)
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
	sort.Strings(files)
	refs := make([]core.SourceRef, 0, len(files))
	for _, file := range files {
		refs = append(refs, core.SourceRef{Path: file})
	}
	pipe := fusion.New(nil, nil, fusion.Config{})
	defer pipe.Close()
	results := make([]row, 0, len(files))
	for _, file := range files {
		source, err := os.ReadFile(filepath.Join(*root, filepath.FromSlash(file)))
		if err != nil {
			panic(err)
		}
		extraction, err := pipe.ExtractForBench("dispatch-review", refs, file, source)
		var nodes []string
		for _, node := range extraction.Nodes {
			sort.Strings(node.Calls)
			data, err := json.Marshal(node)
			if err != nil {
				panic(err)
			}
			nodes = append(nodes, string(data))
		}
		sort.Strings(nodes)
		digest := sha256.Sum256([]byte(strings.Join(nodes, "\n")))
		sourceDigest := sha256.Sum256(source)
		r := row{Path: file, Source: hex.EncodeToString(sourceDigest[:]), Kind: string(extraction.ToolKind), Skipped: extraction.Skipped, Nodes: len(nodes), Digest: hex.EncodeToString(digest[:])}
		if err != nil {
			r.Error = err.Error()
		}
		results = append(results, r)
	}
	if err := json.NewEncoder(os.Stdout).Encode(results); err != nil {
		panic(err)
	}
}

package callgraph

import (
	"path"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// ctorScopeIndex is temporary Build state, never retained on Graph. Keys
// are directories and dot-boundary name suffixes, with every candidate
// retained so ambiguity within or across directory suffixes stays unresolved.
type ctorScopeIndex map[string]map[string][]string

func buildCtorScopeIndex(nodes map[string]core.SymbolNode) ctorScopeIndex {
	index := make(ctorScopeIndex)
	for key, n := range nodes {
		if n.Kind == core.KindGoField || n.FilePath == "" {
			continue
		}
		dir := path.Dir(n.FilePath)
		if dir == "." || dir == "/" {
			continue
		}
		for i := 0; i < len(n.Name)-1; i++ {
			if n.Name[i] != '.' {
				continue
			}
			if index[dir] == nil {
				index[dir] = make(map[string][]string)
			}
			suffix := n.Name[i:]
			index[dir][suffix] = append(index[dir][suffix], key)
		}
	}
	return index
}

func (index ctorScopeIndex) resolve(importPath, methodSuffix string) string {
	if importPath == "" || len(methodSuffix) < 2 {
		return ""
	}
	var match string
	for {
		for _, key := range index[importPath][methodSuffix] {
			if match != "" {
				return ""
			}
			match = key
		}
		i := strings.IndexByte(importPath, '/')
		if i < 0 {
			return match
		}
		importPath = importPath[i+1:]
	}
}

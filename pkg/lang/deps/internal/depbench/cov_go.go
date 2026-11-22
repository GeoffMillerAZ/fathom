package depbench

import (
	"path"
	"strings"

	"github.com/geoffmilleraz/fathom/pkg/lang/deps"
	"golang.org/x/mod/modfile"
)

// COVGoInventory uses tracked module roots and source directories, independently
// of product edges and native resolution. Nested modules shadow parent roots.
func COVGoInventory(tree map[string][]byte) map[string]bool {
	modules := map[string]string{}
	for file, src := range tree {
		if path.Base(file) != "go.mod" {
			continue
		}
		name := modfile.ModulePath(src)
		if name != "" {
			modules[path.Dir(file)] = name
		}
	}
	out := map[string]bool{}
	for _, name := range modules {
		out["go:"+name] = true
	}
	for file := range tree {
		if !strings.HasSuffix(file, ".go") {
			continue
		}
		dir := path.Dir(file)
		for root := dir; ; root = path.Dir(root) {
			if name, ok := modules[root]; ok {
				suffix := strings.TrimPrefix(strings.TrimPrefix(dir, root), "/")
				if root == "." {
					suffix = dir
				}
				if suffix != "." && suffix != "" {
					name += "/" + suffix
				}
				out["go:"+name] = true
				break
			}
			if root == "." {
				break
			}
		}
	}
	return out
}

// COVGoScore also rejects an external subpath under a tracked module. Go module
// roots establish locality independently of which packages the oracle could list.
func COVGoScore(o Observation, inventory map[string]bool) COVReading {
	out := COVScore(o, inventory)
	for _, file := range o.Files {
		if !file.Complete {
			continue
		}
		for _, e := range file.Edges {
			if e.Resolution != deps.ResolvedExternal || inventory[e.To] {
				continue
			}
			for local := range inventory {
				if strings.HasPrefix(e.To, local+"/") {
					out.ExternalAsLocal++
					break
				}
			}
		}
	}
	out.B1 = out.B1 && out.ExternalAsLocal == 0
	return out
}

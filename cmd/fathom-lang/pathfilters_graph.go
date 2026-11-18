package main

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"slices"

	"github.com/geoffmilleraz/fathom/pkg/depgraph"
	"github.com/geoffmilleraz/fathom/pkg/lang/deps"
)

type pathFilterGraph struct {
	units     []depgraph.Unit
	files     map[string]deps.FileDeps
	inventory []string
	closures  map[string]pathFilterClosure
}

type pathFilterClosure struct {
	paths, trusted []string
	unknowns       []pathFilterUnknown
}

func readPathFilterDependencies(ctx context.Context, view fs.FS, reader *deps.Reader, inventory []string) (*pathFilterGraph, error) {
	var inputs []deps.Input
	for _, name := range inventory {
		if validPathFilterPath(name) {
			inputs = append(inputs, deps.Input{Path: name})
		}
	}
	var files []deps.FileDeps
	if len(inputs) != 0 {
		read, err := reader.ReadFiles(ctx, view, inputs)
		if errors.Is(err, deps.ErrInput) {
			return nil, err
		}
		// A failed file has typed unknowns. A cancellation leaves missing
		// evidence for the unread suffix. Account for both within each closure.
		files = read
	}
	byPath := make(map[string]deps.FileDeps, len(files))
	var fileEdges []deps.FileDeps
	for _, file := range files {
		byPath[file.Path] = file
		if len(file.Edges) != 0 {
			edges := slices.DeleteFunc(slices.Clone(file.Edges), func(edge deps.Edge) bool {
				return edge.Resolution == deps.ResolvedDirectory
			})
			fileEdges = append(fileEdges, deps.FileDeps{Edges: edges})
		}
	}
	units, err := reversePathFilterEdges(fileEdges)
	if err != nil {
		return nil, err
	}
	return &pathFilterGraph{units, byPath, inventory, make(map[string]pathFilterClosure)}, nil
}

func reversePathFilterEdges(files []deps.FileDeps) ([]depgraph.Unit, error) {
	wire, err := json.Marshal(declaredPairs(files))
	if err != nil {
		return nil, err
	}
	var declarations []depgraph.EdgeDecl
	if err := json.Unmarshal(wire, &declarations); err != nil {
		return nil, err
	}

	// Reverse checkout file edges so Dependents walks toward dependencies.
	// External units have no checkout path. The dependency reader has
	// already resolved the file identities, so no unit resolver is needed.
	var units []depgraph.Unit
	indices := make(map[string]int)
	for _, declaration := range declarations {
		if declaration.From.Ecosystem != "repo" || declaration.To.Ecosystem != "repo" {
			continue
		}
		target := declaration.To.Name
		i, exists := indices[target]
		if !exists {
			i = len(units)
			indices[target] = i
			units = append(units, depgraph.Unit{Ecosystem: "repo", Name: target})
		}
		units[i].Deps = append(units[i].Deps, declaration.From.Name)
	}
	return units, nil
}

func (g *pathFilterGraph) closure(entry string) ([]string, []string, []pathFilterUnknown) {
	if entry == "" {
		return []string{}, nil, nil
	}
	if cached, ok := g.closures[entry]; ok {
		return cached.paths, cached.trusted, cached.unknowns
	}

	closure := pathFilterClosure{paths: append(depgraph.Dependents(g.units, []string{entry}), entry)}
	slices.Sort(closure.paths)
	for _, name := range closure.paths {
		file, exists := g.files[name]
		switch {
		case !pathFilterInventoryHas(g.inventory, name):
			closure.unknowns = append(closure.unknowns, pathFilterUnknown{Path: name, Code: "file-not-in-inventory"})
		case !exists || (!file.Complete && len(file.Unknowns) == 0):
			closure.unknowns = append(closure.unknowns, pathFilterUnknown{Path: name, Code: "missing-dependency-evidence"})
		}
		for _, unknown := range file.Unknowns {
			closure.unknowns = append(closure.unknowns, pathFilterUnknown{
				Path: name, Code: string(unknown.Code), Line: unknown.Line, Raw: unknown.Raw, DecidableBy: unknown.DecidableBy,
			})
		}
		for _, edge := range file.Edges {
			if edge.Resolution == deps.ResolvedDirectory {
				closure.unknowns = append(closure.unknowns, pathFilterUnknown{
					Path: name, Code: "directory-dependency", Target: edge.To, Line: edge.Line,
				})
			}
		}
		if file.Trusted {
			closure.trusted = append(closure.trusted, name)
		}
	}
	g.closures[entry] = closure
	return closure.paths, closure.trusted, closure.unknowns
}

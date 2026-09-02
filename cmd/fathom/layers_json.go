package main

// layers_json.go — `fathom layers list --json` (RP-17,
// docs/plans/reach-parity.md): the same catalog the table prints — schema
// data (EVAL) always, production truth (CLAIMS, LAST_COMPILE) only when
// --store/--service supplied it — as data, with a broken project pack
// carried as an entry with `load_error`, never dropped.

import (
	"encoding/json"
	"io"
	"sort"

	"github.com/geoffmilleraz/fathom/internal/semantic"
)

type layerJSON struct {
	Name          string   `json:"name"`
	Source        string   `json:"source"`
	Overrides     bool     `json:"overrides"`
	Kind          string   `json:"kind,omitempty"`
	Tier          string   `json:"tier,omitempty"`
	Stratum       string   `json:"stratum,omitempty"`
	Evaled        bool     `json:"evaled"`
	DriftTracked  bool     `json:"drift_tracked"`
	CorpusInclude []string `json:"corpus_include,omitempty"`
	LoadError     string   `json:"load_error,omitempty"`
	// Claims and LastCompile are present only when the store was read.
	Claims      *int   `json:"claims,omitempty"`
	LastCompile string `json:"last_compile,omitempty"`
}

type layersListJSON struct {
	Root   string      `json:"root"`
	Layers []layerJSON `json:"layers"`
	// Orphans are layers with served claims but no defining pack; only
	// computed when the store was read.
	Orphans []layerJSON `json:"orphans,omitempty"`
	// StoreRead says whether claims/last_compile carry store truth.
	StoreRead bool `json:"store_read"`
}

// renderLayersListJSON writes the envelope. claimCounts/lastCompile are
// nil when no store was read.
func renderLayersListJSON(w io.Writer, root string, infos []semantic.LayerInfo, claimCounts map[string]int, lastCompile map[string]string) error {
	out := layersListJSON{Root: root, Layers: []layerJSON{}, StoreRead: claimCounts != nil}
	known := map[string]bool{}
	for _, info := range infos {
		known[info.Name] = true
		stratum := info.Stratum
		if stratum == "" && info.Kind != "" {
			stratum = "concern"
		}
		l := layerJSON{
			Name: info.Name, Source: string(info.Source), Overrides: info.Overrides,
			Kind: info.Kind, Tier: info.Tier, Stratum: stratum, Evaled: info.Evaled,
			DriftTracked: info.Serving.DriftTracked, CorpusInclude: info.CorpusInclude,
			LoadError: info.LoadError,
		}
		if claimCounts != nil {
			n := claimCounts[info.Name]
			l.Claims = &n
			l.LastCompile = lastCompile[info.Name]
		}
		out.Layers = append(out.Layers, l)
	}
	sort.Slice(out.Layers, func(i, j int) bool { return out.Layers[i].Name < out.Layers[j].Name })
	if claimCounts != nil {
		var names []string
		for layer := range claimCounts {
			if !known[layer] {
				names = append(names, layer)
			}
		}
		sort.Strings(names)
		for _, name := range names {
			n := claimCounts[name]
			out.Orphans = append(out.Orphans, layerJSON{Name: name, Claims: &n, LastCompile: lastCompile[name]})
		}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

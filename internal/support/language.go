package support

// language.go — the language/tool registry adapter (SM-4, design §2). It
// reads internal/extractor/dialects/<id>/dialect.json manifests (what
// fathom has actually built, LB-F1) and docs/plans/language-breadth.queue.json
// (the target list for these two kinds, LB-F22) as plain JSON, decoding
// only the fields this package needs — never internal/extractor's own
// Dialect type, so importing `support` never pulls in that package's
// tree-sitter/wasm extraction engines.

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// queueEntry is the minimal decode of one docs/plans/language-breadth.queue.json
// row (LB-F22) — never the full worker-recipe shape (calibration_subjects,
// depends_on, notes, …), which belongs to the language-breadth plan alone.
type queueEntry struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Kind   string `json:"kind"` // "language" | "tooling"
	Wave   int    `json:"wave"`
	Engine string `json:"engine"`
}

// kind maps the queue file's own vocabulary ("language" | "tooling") to
// support.Kind ("language" | "tool").
func (q queueEntry) kind() Kind {
	if q.Kind == "tooling" {
		return KindTool
	}
	return KindLanguage
}

// dialectManifest is the minimal decode of one
// internal/extractor/dialects/<id>/dialect.json this package needs.
type dialectManifest struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Engine string `json:"engine"`
}

// languageCapabilities is the fixed capability-column order for a kind:
// L1-L4 for a language, L1-L3 for a tool format — L4 is n/a for a tool
// (design §2/§3).
func languageCapabilities(kind Kind) []string {
	if kind == KindTool {
		return []string{string(L1), string(L2), string(L3)}
	}
	return []string{string(L1), string(L2), string(L3), string(L4)}
}

// languageRegistry builds the language/tool registry (design §2's `support.
// Registry`, "what fathom loads") from the dialect manifests under
// dialectsDir and the language queue at queuePath: only an id with a landed
// manifest is registered here (a queue row with no manifest yet is a
// declared target, CheckClosure's job — SM-7 — not this adapter's).
func languageRegistry(dialectsDir, queuePath string) ([]RegistryEntry, error) {
	queueData, err := os.ReadFile(queuePath)
	if err != nil {
		return nil, fmt.Errorf("support: read %s: %w", queuePath, err)
	}
	var queue []queueEntry
	if err := json.Unmarshal(queueData, &queue); err != nil {
		return nil, fmt.Errorf("support: decode %s: %w", queuePath, err)
	}
	byID := make(map[string]queueEntry, len(queue))
	for _, q := range queue {
		byID[q.ID] = q
	}

	manifests, err := loadDialectManifests(dialectsDir)
	if err != nil {
		return nil, err
	}

	entries := make([]RegistryEntry, 0, len(manifests))
	for _, m := range manifests {
		kind := KindLanguage
		if m.Kind == "tool" || m.Kind == "tooling" {
			kind = KindTool
		}
		name := m.ID
		wave := 0
		if q, ok := byID[m.ID]; ok {
			kind = q.kind()
			name = q.Name
			wave = q.Wave
		}
		entries = append(entries, RegistryEntry{
			Kind:         kind,
			ID:           m.ID,
			Name:         name,
			Wave:         wave,
			Engine:       m.Engine,
			Capabilities: languageCapabilities(kind),
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Kind != entries[j].Kind {
			return entries[i].Kind < entries[j].Kind
		}
		return entries[i].ID < entries[j].ID
	})
	return entries, nil
}

// loadDialectManifests decodes every dialect.json directly under dir's
// immediate subdirectories, skipping any that has none yet.
func loadDialectManifests(dir string) ([]dialectManifest, error) {
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("support: read dialects dir %s: %w", dir, err)
	}
	manifests := make([]dialectManifest, 0, len(dirEntries))
	for _, de := range dirEntries {
		if !de.IsDir() {
			continue
		}
		path := filepath.Join(dir, de.Name(), "dialect.json")
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("support: read %s: %w", path, err)
		}
		var m dialectManifest
		if err := json.Unmarshal(data, &m); err != nil {
			return nil, fmt.Errorf("support: decode %s: %w", path, err)
		}
		manifests = append(manifests, m)
	}
	return manifests, nil
}

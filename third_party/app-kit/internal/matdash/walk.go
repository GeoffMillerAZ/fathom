// Package matdash walks the MatDash React template source tree and produces
// index records (normalize.Component + normalize.VariantRow) for every .tsx
// component file. It reuses the shared twplus/index and twplus/normalize
// packages so the MatDash corpus lands in the same SQLite schema as the
// TailwindPlus corpus, making it searchable via the same MCP tools.
package matdash

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/geoffmilleraz/app-kit/internal/twplus/normalize"
)

// Record is one indexed file: the component descriptor plus the single
// variant row ready for index.Builder.AddComponent.
type Record struct {
	Component normalize.Component
	Rows      []normalize.VariantRow
}

// skipSegments are path segments that, when present anywhere in the relative
// path under src, cause the file to be skipped.
var skipSegments = map[string]bool{
	"Code":    true,
	"Codes":   true,
	"context": true,
	"routes":  true,
	"api":     true,
	"utils":   true,
	"assets":  true,
	"css":     true,
	"types":   true,
}

// includeSections are the only top-level directories under src that we index.
var includeSections = map[string]bool{
	"components": true,
	"views":      true,
	"layouts":    true,
}

// Walk traverses srcRoot (the `src` directory of a MatDash package) and calls
// fn for each indexable .tsx file. It returns the total number of files walked
// and the number skipped.
func Walk(srcRoot string, fn func(Record) error) (walked, skipped int, err error) {
	err = filepath.WalkDir(srcRoot, func(path string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		if d.IsDir() {
			return nil
		}

		rel, relErr := filepath.Rel(srcRoot, path)
		if relErr != nil {
			return relErr
		}

		// Only .tsx files.
		if !strings.HasSuffix(rel, ".tsx") {
			skipped++
			return nil
		}

		// Split into slash-separated parts (works on all platforms since
		// filepath.Rel always uses the OS separator).
		parts := strings.Split(rel, string(filepath.Separator))

		// First segment must be an included section.
		if !includeSections[parts[0]] {
			skipped++
			return nil
		}

		// Skip if any segment matches a skip set.
		for _, seg := range parts {
			if skipSegments[seg] {
				skipped++
				return nil
			}
		}

		// Skip App.tsx, main.tsx, *.d.ts anywhere in the tree.
		base := filepath.Base(path)
		if base == "App.tsx" || base == "main.tsx" || strings.HasSuffix(base, ".d.ts") {
			skipped++
			return nil
		}

		rec, recErr := buildRecord(srcRoot, rel, parts)
		if recErr != nil {
			return fmt.Errorf("matdash: %s: %w", rel, recErr)
		}
		walked++
		return fn(rec)
	})
	return walked, skipped, err
}

// buildRecord constructs the normalize.Component and its single VariantRow
// from the relative path parts and the raw file contents.
// parts must be filepath.Separator-split segments of rel.
func buildRecord(srcRoot, rel string, parts []string) (Record, error) {
	// parts[0]        = section ("components" | "views" | "layouts")
	// parts[1..n-2]   = intermediate directories
	// parts[n-1]      = filename (e.g. "Button.tsx" or "index.tsx")

	section := parts[0]
	filename := parts[len(parts)-1]

	// Derive stem (strip .tsx).
	stem := strings.TrimSuffix(filename, ".tsx")

	// If stem == "index", use the parent directory name as the component name.
	var name string
	if stem == "index" && len(parts) >= 2 {
		name = parts[len(parts)-2]
	} else {
		name = stem
	}

	// Group = "/" -joined intermediate segments between section and filename.
	group := strings.Join(parts[1:len(parts)-1], "/")

	// Category mirrors Group (no deeper subdivision in MatDash).
	category := group

	// FullName is unique per file: section/group/name or section/name when
	// group is empty.
	var fullName string
	if group == "" {
		fullName = section + "/" + name
	} else {
		fullName = section + "/" + group + "/" + name
	}

	// Read file contents.
	absPath := filepath.Join(srcRoot, rel)
	codeBytes, err := os.ReadFile(absPath)
	if err != nil {
		return Record{}, err
	}
	code := string(codeBytes)

	// Content hash: lowercase hex sha256 of the raw bytes.
	sum := sha256.Sum256(codeBytes)
	contentHash := hex.EncodeToString(sum[:])

	// Extract Tailwind utility classes.
	classes := extractClasses(code)

	// EmbedText: join section + group + name + classes (non-empty parts).
	embedParts := []string{section}
	if group != "" {
		embedParts = append(embedParts, group)
	}
	embedParts = append(embedParts, name)
	if classes != "" {
		embedParts = append(embedParts, classes)
	}
	embedText := strings.Join(embedParts, " ")

	variant := normalize.Variant{
		FullName:         fullName,
		Name:             name,
		Code:             code,
		Language:         "tsx",
		Framework:        "react",
		TailwindVersion:  4,
		Mode:             "none",
		SupportsDarkMode: false,
	}

	comp := normalize.Component{
		Section:  section,
		Group:    group,
		Category: category,
		Name:     name,
		FullName: fullName,
		Variants: []normalize.Variant{variant},
	}

	row := normalize.VariantRow{
		Section:          section,
		Group:            group,
		Category:         category,
		Name:             name,
		FullName:         fullName,
		Framework:        "react",
		Language:         "tsx",
		TailwindVersion:  4,
		Mode:             "none",
		SupportsDarkMode: false,
		ContentHash:      contentHash,
		Code:             code,
		Classes:          classes,
		EmbedText:        embedText,
	}

	return Record{
		Component: comp,
		Rows:      []normalize.VariantRow{row},
	}, nil
}

// classAttr matches className="...", className={"..."}, class="..." in TSX.
// The four capture groups cover: double-quote, single-quote, backtick, and
// JSX {" / '} string literals.
var classAttr = regexp.MustCompile(
	`(?:className|class)\s*=\s*(?:"([^"]*)"|'([^']*)'|` + "`" + `([^` + "`" + `]*)` + "`" + `|{["']([^"'}]*)["']})`,
)

// extractClasses pulls Tailwind utility classes out of TSX code, ranks by
// frequency, dedupes, and caps at 120 tokens so embed_text stays compact.
func extractClasses(code string) string {
	const maxClasses = 120
	freq := map[string]int{}
	for _, m := range classAttr.FindAllStringSubmatch(code, -1) {
		// Any of the four capture groups may be populated.
		val := m[1] + m[2] + m[3] + m[4]
		for _, tok := range strings.Fields(val) {
			tok = strings.TrimSpace(tok)
			if tok == "" || strings.ContainsAny(tok, "{}$()") {
				continue
			}
			freq[tok]++
		}
	}
	if len(freq) == 0 {
		return ""
	}
	toks := make([]string, 0, len(freq))
	for t := range freq {
		toks = append(toks, t)
	}
	sort.Slice(toks, func(i, j int) bool {
		if freq[toks[i]] != freq[toks[j]] {
			return freq[toks[i]] > freq[toks[j]]
		}
		return toks[i] < toks[j]
	})
	if len(toks) > maxClasses {
		toks = toks[:maxClasses]
	}
	return strings.Join(toks, " ")
}

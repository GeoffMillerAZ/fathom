package extractor

import (
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/core"
)

// DockerfileExtractor implements Extractor for Dockerfile files. It uses a
// hand-written line scanner (no external dependency) that honours backslash
// line-continuations and skips # comments.
//
// Output:
//   - KindDockerStage — one per FROM directive.
//     Name = the AS alias when present, otherwise the <image>[:<tag>|@<digest>]
//     reference.
//     Signature = normalised FROM line, e.g. "FROM golang:1.26 AS build".
//     Line = the 1-based line number of the opening FROM line.
//     Calls = [base stage name] when the image references a previously-defined
//     stage (stage→base edge); empty otherwise.
//     Tier = core.TierPreScan (heuristic line parse, not a BuildKit AST).
//
// Extract never returns a non-nil error.
type DockerfileExtractor struct{}

// NewDockerfileExtractor returns a new DockerfileExtractor.
func NewDockerfileExtractor() *DockerfileExtractor { return &DockerfileExtractor{} }

// Extract scans src as a Dockerfile and returns one SymbolNode per build stage.
func (d *DockerfileExtractor) Extract(service, relPath string, src []byte) ([]core.SymbolNode, error) {
	pkg := pkgFromPath(relPath)

	// logicalLines holds each logical line (after joining continuations) with
	// its starting 1-based line number in the original file.
	type logicalLine struct {
		text    string
		lineNum int // 1-based line number of the first physical line
	}

	// Join backslash-continued lines into logical lines.
	var logicals []logicalLine
	raw := strings.Split(string(src), "\n")
	i := 0
	for i < len(raw) {
		startLine := i + 1 // 1-based
		var joined strings.Builder
		for i < len(raw) {
			line := raw[i]
			i++
			if strings.HasSuffix(line, "\\") {
				joined.WriteString(line[:len(line)-1])
				joined.WriteByte(' ')
			} else {
				joined.WriteString(line)
				break
			}
		}
		logicals = append(logicals, logicalLine{text: joined.String(), lineNum: startLine})
	}

	// stageNames tracks stage names defined by AS aliases, in order, so we
	// can detect forward references to earlier stages.
	stageNames := map[string]struct{}{}

	var nodes []core.SymbolNode

	for _, ll := range logicals {
		trimmed := strings.TrimSpace(ll.text)

		// Skip blank lines and comments.
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Case-insensitive FROM check.
		upper := strings.ToUpper(trimmed)
		if !strings.HasPrefix(upper, "FROM") {
			continue
		}
		// Must be "FROM " (keyword followed by space or end-of-line).
		rest := trimmed[4:]
		if rest != "" && rest[0] != ' ' && rest[0] != '\t' {
			continue
		}
		rest = strings.TrimSpace(rest)

		// Strip optional --platform=... flag.
		if strings.HasPrefix(strings.ToLower(rest), "--platform") {
			idx := strings.Index(rest, " ")
			if idx < 0 {
				continue // bare --platform with no image — malformed; skip
			}
			rest = strings.TrimSpace(rest[idx:])
		}

		// Split on whitespace to find image and optional AS <name>.
		fields := strings.Fields(rest)
		if len(fields) == 0 {
			continue
		}
		imageRef := fields[0]

		// Look for AS <alias>.
		alias := ""
		if len(fields) >= 3 && strings.EqualFold(fields[1], "AS") {
			alias = fields[2]
		}

		// Determine the name: alias if present, else imageRef.
		name := imageRef
		if alias != "" {
			name = alias
		}

		// Build normalised signature.
		var sigParts []string
		sigParts = append(sigParts, "FROM", imageRef)
		if alias != "" {
			sigParts = append(sigParts, "AS", alias)
		}
		sig := strings.Join(sigParts, " ")

		// Calls edge: if the imageRef matches a previously-defined stage name,
		// record it as a stage→base dependency.
		var calls []string
		if _, ok := stageNames[imageRef]; ok {
			calls = []string{imageRef}
		}

		// Register this stage name (alias takes precedence; imageRef is not a
		// referenceable stage name unless it has no alias).
		if alias != "" {
			stageNames[alias] = struct{}{}
		} else {
			stageNames[imageRef] = struct{}{}
		}

		nodes = append(nodes, core.SymbolNode{
			Service:   service,
			Package:   pkg,
			Name:      name,
			Kind:      core.KindDockerStage,
			FilePath:  relPath,
			Line:      ll.lineNum,
			Signature: sig,
			Calls:     calls,
			Tier:      core.TierPreScan,
		})
	}

	// Deterministic output: sort by Line then Name.
	sort.Slice(nodes, func(i, j int) bool {
		if nodes[i].Line != nodes[j].Line {
			return nodes[i].Line < nodes[j].Line
		}
		return nodes[i].Name < nodes[j].Name
	})

	return nodes, nil
}

// ensure DockerfileExtractor satisfies the Extractor interface at compile time.
var _ Extractor = (*DockerfileExtractor)(nil)

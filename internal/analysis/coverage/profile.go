package coverage

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// Mode is a Go cover profile's coverage mode — its header line's value
// (`mode: <mode>`): "set" (did this line run at all), "count" (how many
// times), or "atomic" (count, safe under -race).
type Mode string

// The three modes `go test -coverprofile` ever writes.
const (
	ModeSet    Mode = "set"
	ModeCount  Mode = "count"
	ModeAtomic Mode = "atomic"
)

// Block is one profiled statement run: one block line of a Go cover
// profile, covering source from (StartLine, StartCol) to (EndLine, EndCol)
// inclusive, spanning NumStmt statements, executed Count times.
type Block struct {
	// ImportPath is the block's file path exactly as the profile wrote
	// it — import-path-relative (the module path under test, prefixed onto
	// the package-relative file path), never repo-relative on its own.
	// JoinSymbols resolves it against the repo-relative FilePath of the
	// indexed symbols it joins with.
	ImportPath string
	StartLine  int
	StartCol   int
	EndLine    int
	EndCol     int
	NumStmt    int
	Count      int
}

// Profile is a fully parsed Go cover profile.
type Profile struct {
	Mode   Mode
	Blocks []Block
}

// blockLineRe matches one cover-profile block line:
//
//	file:startLine.startCol,endLine.endCol numStmt count
var blockLineRe = regexp.MustCompile(`^(.+):(\d+)\.(\d+),(\d+)\.(\d+) (\d+) (\d+)$`)

// ParseProfile parses the Go cover profile format `go test -coverprofile`
// writes: a `mode: set|count|atomic` header line, then one block line per
// profiled statement run. Returns an error on a missing/malformed mode
// line, or any subsequent line that does not match the block grammar — a
// silently-skipped malformed line would under-report coverage with no
// signal that it happened. Blank lines are tolerated (skipped) wherever
// they appear; the profile format itself never emits them, but a
// hand-edited or concatenated file might.
//
// CRITICAL fix (review_tool.go finding 3): every error names the line
// NUMBER and a byte LENGTH only, never the line's own content — this
// profile is read from an MCP-supplied path (review_diff's
// coverage_profiles), so an error string that echoed the offending line
// verbatim was a content leak through the MCP error channel when that
// path named the wrong (possibly sensitive) file.
func ParseProfile(r io.Reader) (Profile, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	var p Profile
	modeSeen := false
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if !modeSeen {
			const prefix = "mode:"
			if !strings.HasPrefix(line, prefix) {
				return Profile{}, fmt.Errorf("coverage: line %d: expected a %q header (got a %d-byte line that is not one)", lineNo, "mode: set|count|atomic", len(line))
			}
			mode := Mode(strings.TrimSpace(strings.TrimPrefix(line, prefix)))
			switch mode {
			case ModeSet, ModeCount, ModeAtomic:
				p.Mode = mode
			default:
				return Profile{}, fmt.Errorf("coverage: line %d: unknown mode (%d-byte value; want set, count, or atomic)", lineNo, len(mode))
			}
			modeSeen = true
			continue
		}
		m := blockLineRe.FindStringSubmatch(line)
		if m == nil {
			return Profile{}, fmt.Errorf("coverage: line %d: malformed block line (%d bytes)", lineNo, len(line))
		}
		startLine, _ := strconv.Atoi(m[2])
		startCol, _ := strconv.Atoi(m[3])
		endLine, _ := strconv.Atoi(m[4])
		endCol, _ := strconv.Atoi(m[5])
		numStmt, _ := strconv.Atoi(m[6])
		count, _ := strconv.Atoi(m[7])
		p.Blocks = append(p.Blocks, Block{
			ImportPath: m[1],
			StartLine:  startLine,
			StartCol:   startCol,
			EndLine:    endLine,
			EndCol:     endCol,
			NumStmt:    numStmt,
			Count:      count,
		})
	}
	if err := sc.Err(); err != nil {
		return Profile{}, fmt.Errorf("coverage: scan: %w", err)
	}
	if !modeSeen {
		return Profile{}, fmt.Errorf("coverage: empty profile: missing %q header", "mode: set|count|atomic")
	}
	return p, nil
}

package custodian

// orientation.go -- the project pack's authored orientation set
// (docs/design/fathom-dir.md section 7.6): entries of an anchor (a repository
// path, or path#Symbol) and why. `fathom config audit` verifies them offline.
// An anchor that no longer resolves is named, never dropped silently.

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"cuelang.org/go/cue"

	"github.com/geoffmilleraz/fathom/internal/projectdir"
)

// OrientationConcernID is the finding id the orientation check reports under.
const OrientationConcernID = "orientation_anchors_resolve"

// maxAnchorFileBytes bounds the file read to look for an anchor's symbol.
const maxAnchorFileBytes = 4 << 20

// OrientationFindings checks the loaded project's orientation set. It returns
// nil when the project sets no orientation, else exactly one finding: holds,
// or violated naming every anchor that does not resolve (or the reason the
// set did not load).
func OrientationFindings(root string, p projectdir.Project) []Finding {
	concern := Concern{
		ID:    OrientationConcernID,
		Want:  "every orientation anchor in the project pack resolves to a repository path, or to a symbol in that file",
		Why:   "a moved or renamed anchor sends every session to a place that no longer exists, and nothing else notices",
		Scope: ScopeProject,
	}
	packFile := p.File(projectdir.Pack)
	if err := packFile.Err(); err != nil {
		return []Finding{{Concern: concern, Status: StatusViolated, Detail: "the project pack does not load, so its orientation set cannot be checked: " + err.Error()}}
	}
	k, ok := p.Key("orientation")
	if !ok {
		return nil
	}
	if k.Err != nil {
		return []Finding{{Concern: concern, Status: StatusViolated, Detail: "the orientation set does not load: " + k.Err.Error()}}
	}
	it, err := k.Value.List()
	if err != nil {
		return []Finding{{Concern: concern, Status: StatusViolated, Detail: "the orientation set is not a list: " + err.Error()}}
	}
	var total int
	var dead []string
	for it.Next() {
		total++
		anchor, serr := it.Value().LookupPath(cue.ParsePath("anchor")).String()
		if serr != nil {
			dead = append(dead, fmt.Sprintf("entry %d: no anchor string", total))
			continue
		}
		if reason := anchorProblem(root, anchor); reason != "" {
			dead = append(dead, anchor+" ("+reason+")")
		}
	}
	if len(dead) == 0 {
		return []Finding{{Concern: concern, Status: StatusHolds, Detail: fmt.Sprintf("%d anchor(s) resolve", total)}}
	}
	return []Finding{{
		Concern:  concern,
		Status:   StatusViolated,
		Detail:   fmt.Sprintf("%d of %d anchor(s) do not resolve", len(dead), total),
		Evidence: dead,
	}}
}

// anchorProblem is why an anchor does not resolve, or "" when it does.
func anchorProblem(root, anchor string) string {
	rel, symbol, hasSym := strings.Cut(anchor, "#")
	rel = path.Clean(filepath.ToSlash(rel))
	if rel == "" || rel == "." || path.IsAbs(rel) || rel == ".." || strings.HasPrefix(rel, "../") {
		return "not a path inside the repository"
	}
	full := filepath.Join(root, filepath.FromSlash(rel))
	info, err := os.Stat(full)
	if err != nil {
		if os.IsNotExist(err) {
			return "path not found"
		}
		return "unreadable: " + err.Error()
	}
	if !hasSym {
		return ""
	}
	if symbol == "" || info.IsDir() {
		return "a symbol anchor names a file"
	}
	f, err := os.Open(full)
	if err != nil {
		return "unreadable: " + err.Error()
	}
	defer f.Close()
	buf := make([]byte, maxAnchorFileBytes)
	n, _ := f.Read(buf)
	re, err := regexp.Compile(`(^|[^A-Za-z0-9_])` + regexp.QuoteMeta(symbol) + `([^A-Za-z0-9_]|$)`)
	if err != nil || !re.Match(buf[:n]) {
		return "symbol not found in file"
	}
	return ""
}

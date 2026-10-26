package labs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/projectdir"
)

// ObservationLabLink is a pointer to a case, never a proof of its outcome.
// An entry without a Lab bullet has empty Lab and Case and its heading's Line.
// File is relative to the caller's project root; Line is one-based.
type ObservationLabLink struct {
	Observation, Lab, Case, File string
	Line                         int
}

var (
	labLinkPattern        = regexp.MustCompile(`^(lab-[a-z0-9]+(?:-[a-z0-9]+)*)/([A-Za-z0-9][A-Za-z0-9._-]*)$`)
	labObservationFile    = regexp.MustCompile(`^O[0-9]+\.md$`)
	labObservationHeading = regexp.MustCompile(`^### (O[0-9]+)\b`)
)

// ValidObservationLabLink reports whether text names one lab and one case.
// Shared by the file reader and observe's filing-time validation.
func ValidObservationLabLink(text string) bool { return labLinkPattern.MatchString(text) }

// ParseObservationLabLinks reads both observation formats. Missing files and
// absent bullets are unlinked, while read failures and malformed bullets are
// named. Results and diagnostics put the legacy log first, then per-entry files
// by numeric observation ID (filename breaks ties), then line.
// Markdown examples inside fenced blocks are not entry metadata.
func ParseObservationLabLinks(root string) ([]ObservationLabLink, error) {
	base := projectdir.Dir
	legacy := filepath.Join(base, string(projectdir.ObservationLog))
	files := []string{legacy}
	dir := filepath.Join(base, string(projectdir.Observations))
	entries, err := os.ReadDir(filepath.Join(root, dir))
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("%s: %w", filepath.ToSlash(dir), err)
	}
	for _, entry := range entries {
		if !entry.IsDir() && labObservationFile.MatchString(entry.Name()) {
			if _, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(entry.Name(), "O"), ".md")); err != nil {
				continue
			}
			files = append(files, filepath.Join(dir, entry.Name()))
		}
	}
	sort.Slice(files[1:], func(i, j int) bool {
		a, b := files[i+1], files[j+1]
		id := func(file string) int {
			n, _ := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(filepath.Base(file), "O"), ".md"))
			return n
		}
		if ai, bi := id(a), id(b); ai != bi {
			return ai < bi
		}
		return a < b
	})
	var out []ObservationLabLink
	var problems []error
	for _, file := range files {
		data, err := os.ReadFile(filepath.Join(root, file))
		if os.IsNotExist(err) && file == legacy {
			continue
		}
		if err != nil {
			problems = append(problems, fmt.Errorf("%s: %w", filepath.ToSlash(file), err))
			continue
		}
		rel := filepath.ToSlash(file)
		current := -1
		var fence byte
		var fenceLen int
		for i, line := range strings.Split(string(data), "\n") {
			trim := strings.TrimSpace(line)
			if len(trim) >= 3 && (trim[0] == '`' || trim[0] == '~') {
				n := 0
				for n < len(trim) && trim[n] == trim[0] {
					n++
				}
				if n >= 3 {
					if fence == 0 {
						fence = trim[0]
						fenceLen = n
						continue
					}
					if fence == trim[0] && n >= fenceLen && strings.TrimSpace(trim[n:]) == "" {
						fence = 0
						continue
					}
				}
			}
			if fence != 0 {
				continue
			}
			if m := labObservationHeading.FindStringSubmatch(line); m != nil {
				out = append(out, ObservationLabLink{Observation: m[1], File: rel, Line: i + 1})
				current = len(out) - 1
				continue
			}
			if !strings.HasPrefix(trim, "- **Lab:**") {
				continue
			}
			value, ok := strings.CutPrefix(trim, "- **Lab:** ")
			value = strings.TrimSpace(value)
			if !ok || !ValidObservationLabLink(value) || current < 0 {
				problems = append(problems, fmt.Errorf("%s:%d: malformed Lab link; want <lab>/<case> under an observation heading", rel, i+1))
				continue
			}
			if out[current].Lab != "" {
				problems = append(problems, fmt.Errorf("%s:%d: duplicate Lab link", rel, i+1))
				continue
			}
			parts := strings.SplitN(value, "/", 2)
			out[current].Lab = parts[0]
			out[current].Case = parts[1]
			out[current].Line = i + 1
		}
	}
	return out, errors.Join(problems...)
}

package intent

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// audit_testfuncs.go — IV-8 (O1136, I38): U4's authority for "does this named
// test exist" is the repository's own test files, not a daemon index that may
// lag HEAD. An index that does not know a name is a stale or partial answer;
// only a name no test file defines is a missing test.

// testFuncRe matches a Go function declaration at column 0 (a test, a
// benchmark, an example or a fuzz target, with or without a receiver).
var testFuncRe = regexp.MustCompile(`(?m)^func (?:\([^)]*\)\s*)?([A-Za-z_][A-Za-z0-9_]*)\(`)

// testTreeSkip are directories that are never the repository's own tests.
var testTreeSkip = map[string]bool{".git": true, "workspace": true, "node_modules": true, "ref": true, "tmp": true}

// testFuncsInTree returns the sorted, de-duplicated names of every function
// declared in a *_test.go file under root. ok is false when root is empty or
// unreadable, so a caller keeps the index verdict instead of inventing one.
func testFuncsInTree(root string) (names []string, ok bool) {
	if root == "" {
		return nil, false
	}
	if st, err := os.Stat(root); err != nil || !st.IsDir() {
		return nil, false
	}
	seen := map[string]bool{}
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // an unreadable subtree contributes nothing
		}
		if d.IsDir() {
			if path != root && testTreeSkip[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}
		body, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil
		}
		for _, m := range testFuncRe.FindAllSubmatch(body, -1) {
			seen[string(m[1])] = true
		}
		return nil
	})
	for n := range seen {
		names = append(names, n)
	}
	sort.Strings(names)
	return names, true
}

// namedTestInTree reports whether name (an exact function name, or a family
// "Stem*" of which any member counts) is declared in funcs (sorted).
func namedTestInTree(funcs []string, name string) bool {
	stem, family := strings.CutSuffix(name, "*")
	i := sort.SearchStrings(funcs, stem)
	if i >= len(funcs) {
		return false
	}
	if !family {
		return funcs[i] == stem
	}
	return strings.HasPrefix(funcs[i], stem)
}

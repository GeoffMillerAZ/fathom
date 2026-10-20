package readout

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/geoffmilleraz/fathom/internal/projectdir"
)

// RepoFile is a repository's .fathom/readouts.toml as the loader saw it
// (RO-33): where it is, the sha256 of its bytes, and whether the user lists
// that hash under `[readouts] trust`.
type RepoFile struct {
	Path    string
	SHA     string
	Trusted bool
}

// LoadRepoRules loads root's .fathom/readouts.toml into rs after the user
// scope (RO-33, docs/design/readouts.md section 11). See loadRepoRules.
func LoadRepoRules(rs *RuleSet, root string, trust []string) {
	loadRepoRules(rs, root, trust)
}

// loadRepoRules adds the repository's rules under every rule already in rs,
// so a user or built-in rule wins each conflict: a repository rule never
// takes a held id and ranks below every other rule at Render. A repository
// rule may extract records and add lines (extract and join stages) and
// nothing else: a rule with any other stage is refused and named, and the
// rest of the file still loads. Its rules run in observe until trust lists
// the file's sha256, so an edited file falls back to observe. An absent file
// is silent. Any failure is a Refusal, never an error.
func loadRepoRules(rs *RuleSet, root string, trust []string) {
	if rs == nil || root == "" {
		return
	}
	p := projectdir.Readouts.Path(root)
	data, err := os.ReadFile(p)
	if errors.Is(err, fs.ErrNotExist) {
		return
	}
	if err != nil {
		rs.Refused = append(rs.Refused, Refusal{Source: p, Reason: err.Error()})
		return
	}
	sum := sha256.Sum256(data)
	sha := hex.EncodeToString(sum[:])
	trusted := slices.Contains(trust, sha)
	rs.Repo = &RepoFile{Path: p, SHA: sha, Trusted: trusted}
	rules, err := parseRuleFile(data)
	if err != nil {
		rs.Refused = append(rs.Refused, Refusal{Source: p, Reason: err.Error()})
		return
	}
	held := map[string]bool{}
	for _, r := range rs.Rules {
		held[r.ID] = true
	}
	dir := os.DirFS(filepath.Dir(p))
	seen := map[string]bool{}
	var added []*Rule
	for i := range rules {
		r := &rules[i]
		r.Source = p
		refuse := func(reason string) {
			rs.Refused = append(rs.Refused, Refusal{Source: p, ID: r.ID, Reason: reason})
		}
		switch {
		case seen[r.ID]:
			refuse("duplicate id in the file")
			continue
		case held[r.ID]:
			refuse("the id is held by a user or built-in rule, which wins")
			continue
		}
		seen[r.ID] = true
		if k := firstNonAdditive(r.Stages); k != "" {
			refuse(fmt.Sprintf("a repository rule may only add: stage kind %q drops or replaces lines", k))
			continue
		}
		if err := prepare(r, dir); err != nil {
			refuse(err.Error())
			continue
		}
		r.untrusted = !trusted
		added = append(added, r)
	}
	rs.Rules = append(added, rs.Rules...)
}

// firstNonAdditive is the first stage kind that is not extract or join, the
// two that add; "" when every stage adds.
func firstNonAdditive(stages []Stage) string {
	for _, s := range stages {
		if s.Kind != StageExtract && s.Kind != StageJoin {
			return s.Kind
		}
	}
	return ""
}

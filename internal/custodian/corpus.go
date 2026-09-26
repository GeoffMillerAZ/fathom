package custodian

// corpus.go — CC-11 (docs/plans/custodian.md, vision 22 §7): the corpus
// registry. A check's `where:` names a corpus; the registry resolves the
// name to a sorted, repo-relative file set, and every kind that reads
// "a named set of files plus the repo" runs over whatever the name
// resolves to. New configuration types arrive as new resolvers behind
// this ONE engine — never a parallel system, never a per-tool custodian.
//
// A corpus also knows where commands live inside its files (commands.go):
// the markdown corpus cites `task X` in code spans, the YAML corpora run
// shell under their own keys. That is the only per-corpus knowledge the
// engine has; everything else is the kind's.
//
// Honesty floor (I38): a candidate that EXISTS but cannot be stat'd is a
// resolution error (unable-to-check upstream), never "absent".

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// The registered corpus names. CorpusAgentConfig (custodian.go) is the
// original one; these join it behind the same `where:` seam.
const (
	CorpusLefthook = "lefthook"
	CorpusTaskfile = "taskfile"
	CorpusCI       = "ci"
)

// corpus is one registered resolver.
type corpus struct {
	// about names the files, for refusal messages, empty-corpus details
	// and docs.
	about string
	// resolve returns the corpus files, repo-relative, sorted.
	resolve func(root string) ([]string, error)
}

// registry is the corpus seam. Adding a corpus is adding an entry here
// (plus, when its files carry commands, a case in commandSyntax) — and,
// per vision 22 §7, a pack of falsifiable concerns that catch a named
// failure: a corpus with no pack is a scanner with no reason to exist.
var registry = map[string]corpus{
	CorpusAgentConfig: {
		about:   "CLAUDE.md / AGENTS.md at any depth (git's authority on what belongs to the repo)",
		resolve: AgentConfigFiles,
	},
	CorpusLefthook: {
		about:   "the lefthook config lefthook loads: " + strings.Join(lefthookNames, ", ") + " (first present)",
		resolve: resolveLefthook,
	},
	CorpusTaskfile: {
		about:   "the root Taskfile (" + strings.Join(taskfileNames, ", ") + ", first present) plus the local Taskfiles it includes",
		resolve: resolveTaskfiles,
	},
	CorpusCI: {
		about:   "CI pipeline configs: .github/workflows/*.yml|yaml, " + strings.Join(ciFixedNames, ", "),
		resolve: resolveCI,
	},
}

// KnownCorpora lists the registered corpus names, sorted.
func KnownCorpora() []string {
	out := make([]string, 0, len(registry))
	for n := range registry {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// CorpusAbout describes a registered corpus's files ("" for an unknown
// name).
func CorpusAbout(name string) string { return registry[name].about }

// ResolveCorpus returns the files a registered corpus names for the repo
// at root, repo-relative and sorted. An unknown name is an error naming
// the registered ones (the loader refuses it first; this is the same
// honesty for a hand-built Check).
func ResolveCorpus(root, name string) ([]string, error) {
	c, ok := registry[name]
	if !ok {
		return nil, fmt.Errorf("unknown corpus %q (registered corpora: %s)", name, strings.Join(KnownCorpora(), ", "))
	}
	return c.resolve(root)
}

// corporaFor names the corpora a kind may read. nil means the kind reads
// no corpus at all (it names its own file, the daemon, or a log), so a
// `where:` on it would be a wish about behaviour that does not exist.
//
// pointers_resolve and stanza_tools_served stay markdown-only: a pointer
// is a markdown notion, and the stanza lives only in agent config.
func corporaFor(kind CheckKind) []string {
	switch kind {
	case KindForbiddenPattern, KindRequiredPattern, KindCommandDocumentedAndExists, KindCommandsResolve:
		return KnownCorpora()
	case KindPointersResolve, KindStanzaToolsServed:
		return []string{CorpusAgentConfig}
	}
	return nil
}

// lefthookNames are the lefthook config spellings this corpus resolves,
// in lefthook's own precedence. Lefthook loads ONE main config, so the
// corpus is the first present. The dot-prefixed and .config/ spellings
// and lefthook-local.yml are not resolved yet: doctor's lefthook_gates row
// renders this corpus (CC-13) and has always read exactly these two, so
// widening it changes that row for the repos that use them — a separate,
// named change, not a side effect of the seam.
var lefthookNames = []string{"lefthook.yml", "lefthook.yaml"}

func resolveLefthook(root string) ([]string, error) {
	for _, name := range lefthookNames {
		ok, err := present(filepath.Join(root, name))
		if err != nil {
			return nil, err
		}
		if ok {
			return []string{name}, nil
		}
	}
	return nil, nil
}

// taskfileNames are the root Taskfile spellings, in Task's own search
// order (the first present is the one `task` runs).
var taskfileNames = []string{
	"Taskfile.yml", "taskfile.yml", "Taskfile.yaml", "taskfile.yaml",
	"Taskfile.dist.yml", "taskfile.dist.yml", "Taskfile.dist.yaml", "taskfile.dist.yaml",
}

// rootTaskfile returns the Taskfile `task` would run at root ("" when
// there is none).
func rootTaskfile(root string) (string, error) {
	for _, name := range taskfileNames {
		ok, err := present(filepath.Join(root, name))
		if err != nil {
			return "", err
		}
		if ok {
			return name, nil
		}
	}
	return "", nil
}

// resolveTaskfiles is the root Taskfile plus every LOCAL Taskfile it
// includes (transitively): an included file's commands are the chain too.
// A remote include is not a file in this repo and is skipped; an include
// that names a missing local file is not a corpus member either — the
// commands_resolve check reports it as a broken link.
func resolveTaskfiles(root string) ([]string, error) {
	first, err := rootTaskfile(root)
	if err != nil || first == "" {
		return nil, err
	}
	g, err := loadTaskGraph(root, first)
	if err != nil {
		return nil, err
	}
	return g.files, nil
}

// ciFixedNames are the single-file CI configs this corpus resolves beside
// GitHub Actions' workflow directory. Tool names, not organizations
// (ADR-011): every one is a public CI product's documented default path.
var ciFixedNames = []string{
	".gitlab-ci.yml",
	".circleci/config.yml",
	"azure-pipelines.yml",
	"bitbucket-pipelines.yml",
	".buildkite/pipeline.yml",
}

func resolveCI(root string) ([]string, error) {
	var out []string
	wf := filepath.Join(root, ".github", "workflows")
	entries, err := os.ReadDir(wf)
	switch {
	case err == nil:
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			if ext := filepath.Ext(e.Name()); ext == ".yml" || ext == ".yaml" {
				out = append(out, ".github/workflows/"+e.Name())
			}
		}
	case errors.Is(err, fs.ErrNotExist):
	default:
		// The directory exists and cannot be listed: saying "no CI" here
		// would be the I38 failure — report it.
		return nil, fmt.Errorf(".github/workflows unreadable: %w", err)
	}
	for _, name := range ciFixedNames {
		ok, err := present(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out, nil
}

// present is a regular-file existence probe that keeps the I38 line: not
// there is (false, nil); there-but-unstattable is an error, never false.
func present(p string) (bool, error) {
	fi, err := os.Stat(p)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return fi.Mode().IsRegular(), nil
}

// corpusNoun is how a detail names a corpus's files, before countNoun's
// "(s)": "agent-config file", "lefthook file", "Taskfile", "CI config".
func corpusNoun(where string) string {
	switch where {
	case CorpusTaskfile:
		return "Taskfile"
	case CorpusCI:
		return "CI config"
	case "":
		where = CorpusAgentConfig
	}
	return where + " file"
}

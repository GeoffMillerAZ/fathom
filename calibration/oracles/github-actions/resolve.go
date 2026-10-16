package main

// resolve.go (LB-F457, wave lf8): actionlint's OWN resolution of a local
// `uses:`, not this program's localTarget rule. A Project rooted at the
// subject (actionlint.NewProject) gives actionlint's LocalActionsCache, which
// reads a step's `uses: ./dir` as the metadata file it finds in that
// directory (action.yaml before action.yml, its own order) and parses it into
// its ActionMetadata, and its LocalReusableWorkflowCache, which reads a job's
// `uses: ./path` as that file and accepts it only when its `on:` names
// workflow_call. -resolve-out writes what those two caches attest, at file
// granularity, in the tool-oracle shape (tier C: actionlint is an independent
// parser library, owner decision D4), so it can be compared edge for edge
// with the reading's cross_references (workflow_parser_oracle.mjs -xref-out).
//
// What it does not attest: actionlint lints workflows only, so a composite
// action's own steps are read by the yaml.v3 walk (action, main.go) and only
// their resolution is actionlint's; a directory holding a Dockerfile and no
// metadata file (which GitHub's runner accepts) resolves nothing; and the
// file order differs from the runner's (action.yml, then action.yaml) when a
// directory holds both.

import (
	"io"
	"path/filepath"
	"strings"

	"github.com/rhysd/actionlint"
)

// resolveStats counts what the two caches were asked and what they attested.
type resolveStats struct {
	StepUses       int `json:"step_uses"`
	StepResolved   int `json:"step_resolved"`
	StepInvalid    int `json:"step_invalid"`
	JobUses        int `json:"job_uses"`
	JobResolved    int `json:"job_resolved"`
	JobRejected    int `json:"job_rejected"`
	BothMetadata   int `json:"both_metadata_files"`
	Expression     int `json:"expression"`
	OutsideProject int `json:"outside_project"`
}

// resolver holds one subject's actionlint caches.
type resolver struct {
	root    string
	actions *actionlint.LocalActionsCache
	flows   *actionlint.LocalReusableWorkflowCache
	stats   resolveStats
}

func newResolver(root string) (*resolver, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	proj, err := actionlint.NewProject(abs)
	if err != nil {
		return nil, err
	}
	return &resolver{
		root:    abs,
		actions: actionlint.NewLocalActionsCache(proj, io.Discard),
		flows:   actionlint.NewLocalReusableWorkflowCache(proj, abs, io.Discard),
	}, nil
}

// local is a `uses:` value both caches take: one starting "./" (any other
// value, remote or docker://, they return nothing for).
func local(uses string) bool { return strings.HasPrefix(uses, "./") }

// rel is a file under the project root, slash-separated; "" outside it.
func (r *resolver) rel(abs string) string {
	p, err := filepath.Rel(r.root, abs)
	if err != nil || strings.HasPrefix(p, "..") {
		r.stats.OutsideProject++
		return ""
	}
	return filepath.ToSlash(p)
}

// step is the metadata file LocalActionsCache resolves a step's local uses
// to ("" when it finds none, or one it cannot parse).
func (r *resolver) step(uses string) string {
	if !local(uses) {
		return ""
	}
	r.stats.StepUses++
	if actionlint.ContainsExpression(uses) {
		r.stats.Expression++
		return ""
	}
	dir := filepath.Join(r.root, filepath.FromSlash(uses))
	if isFile(filepath.Join(dir, "action.yml")) && isFile(filepath.Join(dir, "action.yaml")) {
		r.stats.BothMetadata++
	}
	m, _, err := r.actions.FindMetadata(uses)
	switch {
	case err != nil:
		r.stats.StepInvalid++
		return ""
	case m == nil:
		return ""
	}
	r.stats.StepResolved++
	return r.rel(m.Path())
}

// job is the workflow file LocalReusableWorkflowCache resolves a job's local
// uses to ("" when the file is missing or names no workflow_call trigger).
func (r *resolver) job(uses string) string {
	if !local(uses) {
		return ""
	}
	r.stats.JobUses++
	if actionlint.ContainsExpression(uses) {
		r.stats.Expression++
		return ""
	}
	m, err := r.flows.FindMetadata(uses)
	if err != nil || m == nil {
		r.stats.JobRejected++
		return ""
	}
	r.stats.JobResolved++
	return r.rel(filepath.Join(r.root, filepath.FromSlash(uses)))
}

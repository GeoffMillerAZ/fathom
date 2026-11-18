package main

import (
	"bytes"
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"
)

type pathFilterScope struct {
	Workflow string
	Job      string
	Entries  []string
}

// Keep entry identities within the syntax accepted by the dependency reader.
var (
	pathFilterPathToken = regexp.MustCompile(`^[A-Za-z0-9._@+~ -]+(/[A-Za-z0-9._@+~ -]+)*$`)
	pathFilterWorkflow  = regexp.MustCompile(`^\.github/workflows/[A-Za-z0-9._-]+\.ya?ml$`)
	pathFilterJob       = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*$`)
)

func validPathFilterPath(name string) bool {
	return name != "." && len(name) <= 512 && fs.ValidPath(name) &&
		pathFilterPathToken.MatchString(name) && !pathFilterVCSPath(name)
}

func validPathFilterWorkflow(name string) bool {
	return validPathFilterPath(name) && pathFilterWorkflow.MatchString(name)
}

func pathFilterVCSPath(name string) bool {
	for part := range strings.SplitSeq(name, "/") {
		switch part {
		case ".git", ".hg", ".svn", ".bzr":
			return true
		}
	}
	return false
}

func parsePathFilterMap(data []byte) ([]pathFilterScope, error) {
	if !utf8.Valid(data) {
		return nil, errors.New("coverage map must be UTF-8")
	}
	doc, err := readPathFilterMapObject(data, "schema_version", "coverage")
	if err != nil {
		return nil, err
	}
	var version int
	var declarations []json.RawMessage
	if json.Unmarshal(doc["schema_version"], &version) != nil || version != 1 ||
		json.Unmarshal(doc["coverage"], &declarations) != nil || len(declarations) == 0 {
		return nil, errors.New("coverage map requires schema_version 1 and at least one declaration")
	}
	scopes := make([]pathFilterScope, 0, len(declarations))
	for i, row := range declarations {
		scope, err := parsePathFilterScope(row)
		if err != nil {
			return nil, fmt.Errorf("coverage declaration %d: %w", i+1, err)
		}
		scopes = append(scopes, scope)
	}
	slices.SortFunc(scopes, comparePathFilterScopes)
	for i := 1; i < len(scopes); i++ {
		if comparePathFilterScopes(scopes[i-1], scopes[i]) == 0 {
			return nil, errors.New("coverage map contains a duplicate workflow/job scope")
		}
	}
	return scopes, nil
}

func parsePathFilterScope(row []byte) (pathFilterScope, error) {
	var scope pathFilterScope
	fields, err := readPathFilterMapObject(row, "workflow", "job", "entries")
	if err != nil {
		return scope, err
	}
	if json.Unmarshal(fields["workflow"], &scope.Workflow) != nil || !validPathFilterWorkflow(scope.Workflow) {
		return scope, errors.New("workflow must be a file path under .github/workflows")
	}
	if job, ok := fields["job"]; ok && (json.Unmarshal(job, &scope.Job) != nil || !pathFilterJob.MatchString(scope.Job)) {
		return scope, errors.New("job must be a GitHub Actions job ID")
	}
	if json.Unmarshal(fields["entries"], &scope.Entries) != nil || len(scope.Entries) == 0 {
		return scope, errors.New("entries must be a nonempty list of repository-relative paths")
	}
	slices.Sort(scope.Entries)
	for i, entry := range scope.Entries {
		if !validPathFilterPath(entry) || (i > 0 && entry == scope.Entries[i-1]) {
			return scope, errors.New("entries must be distinct repository-relative paths")
		}
	}
	return scope, nil
}

// Read the exact fields before decoding their types. encoding/json's struct
// decoder otherwise accepts repeated keys and case aliases.
func readPathFilterMapObject(data []byte, allowed ...string) (map[string]json.RawMessage, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	if token, err := dec.Token(); err != nil || token != json.Delim('{') {
		return nil, errors.New("coverage map requires JSON objects")
	}
	fields := make(map[string]json.RawMessage, len(allowed))
	for dec.More() {
		token, err := dec.Token()
		key, ok := token.(string)
		if err != nil || !ok || !slices.Contains(allowed, key) || fields[key] != nil {
			return nil, errors.New("coverage map has an unknown or repeated field")
		}
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, errors.New("coverage map has an invalid JSON value")
		}
		fields[key] = raw
	}
	if token, err := dec.Token(); err != nil || token != json.Delim('}') {
		return nil, errors.New("coverage map has an unclosed JSON object")
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errors.New("coverage map has trailing JSON data")
	}
	return fields, nil
}

func comparePathFilterScopes(a, b pathFilterScope) int {
	return cmp.Or(cmp.Compare(a.Workflow, b.Workflow), cmp.Compare(a.Job, b.Job))
}

func selectPathFilterScopes(scopes []pathFilterScope, requested []string) []pathFilterScope {
	if len(requested) == 0 {
		return scopes
	}
	requested = slices.Clone(requested)
	slices.Sort(requested)
	var selected []pathFilterScope
	for _, workflow := range slices.Compact(requested) {
		start, _ := slices.BinarySearchFunc(scopes, workflow, func(scope pathFilterScope, name string) int {
			return cmp.Compare(scope.Workflow, name)
		})
		end := start
		for end < len(scopes) && scopes[end].Workflow == workflow {
			end++
		}
		if start < end {
			selected = append(selected, scopes[start:end]...)
		} else {
			selected = append(selected, pathFilterScope{Workflow: workflow})
		}
	}
	return selected
}

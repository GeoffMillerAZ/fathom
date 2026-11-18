package extractor

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// GitHubPathFilters is the static path-filter evidence from one workflow.
// Events and their patterns retain declaration order. Job values give the
// reason a job's execution is unknown, or an empty string.
type GitHubPathFilters struct {
	Events []GitHubPathEvent
	Jobs   map[string]string
}

type GitHubPathEvent struct {
	Name     string
	Patterns []GitHubPathPattern
	Ignore   bool
	// Condition names a non-path selector whose result depends on the event.
	Condition string
}

// Includes reports whether this one repository-relative path passes the
// event's path filter. It does not evaluate an event or job condition.
func (e GitHubPathEvent) Includes(path string) bool {
	included := e.Ignore || len(e.Patterns) == 0
	for _, p := range e.Patterns {
		if p.Match(path) {
			included = !e.Ignore && !p.negative
		}
	}
	return included
}

// ReadGitHubPathFilters reads GitHub push and pull_request path filters and
// job conditions through the document engine's YAML representation.
func ReadGitHubPathFilters(src []byte) (GitHubPathFilters, error) {
	// The generic reader coalesces duplicate keys and stores scalar text.
	// Check the YAML structure and the pattern scalar types before that loss.
	if err := validatePathFilterYAML(src); err != nil {
		return GitHubPathFilters{}, err
	}
	docs, partial, err := decodeDocReport("yaml", src, false, "")
	if err != nil || partial != "" || len(docs) != 1 || docs[0].kind != docMap {
		return GitHubPathFilters{}, errors.New("expected one readable workflow mapping")
	}
	doc := docs[0]
	on := doc.get("on")
	if on == nil {
		return GitHubPathFilters{}, errors.New("missing workflow event declaration")
	}

	var names []string
	switch on.kind {
	case docMap:
		names = on.keys
	case docSeq:
		for _, item := range on.items {
			if item.kind != docScalar {
				return GitHubPathFilters{}, errors.New("workflow event names must be strings")
			}
			names = append(names, item.scalar)
		}
	case docScalar:
		names = []string{on.scalar}
	}
	if len(names) == 0 {
		return GitHubPathFilters{}, errors.New("empty workflow event declaration")
	}

	result := GitHubPathFilters{Jobs: make(map[string]string)}
	seen := make(map[string]bool)
	for _, name := range names {
		if name == "" || strings.Contains(name, "${{") || seen[name] {
			return GitHubPathFilters{}, errors.New("empty, dynamic or repeated workflow event")
		}
		seen[name] = true
		if name != "push" && name != "pull_request" {
			continue
		}
		event, err := readGitHubPathEvent(name, on.get(name))
		if err != nil {
			return GitHubPathFilters{}, fmt.Errorf("%s: %w", name, err)
		}
		result.Events = append(result.Events, event)
	}
	if jobs := doc.get("jobs"); jobs != nil {
		if jobs.kind != docMap {
			return GitHubPathFilters{}, errors.New("jobs must be a mapping")
		}
		for i, job := range jobs.keys {
			if !gitHubPathFilterJobID.MatchString(job) {
				return GitHubPathFilters{}, errors.New("invalid job ID")
			}
			condition, err := readGitHubPathJob(jobs.vals[i])
			if err != nil {
				return GitHubPathFilters{}, err
			}
			result.Jobs[job] = condition
		}
	}
	return result, nil
}

var gitHubPathFilterJobID = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*$`)

func readGitHubPathJob(job *docNode) (string, error) {
	if job.kind != docMap {
		return "", errors.New("job must be a mapping")
	}
	var conditions []string
	conditional, err := unevaluatedGitHubIf(job.get("if"))
	if err != nil {
		return "", err
	}
	if conditional {
		conditions = append(conditions, "job if is false or unevaluated")
	}
	if job.get("needs") != nil {
		conditions = append(conditions, "job depends on another job's result")
	}
	if job.get("uses") != nil {
		conditions = append(conditions, "reusable workflow job is unevaluated")
	}
	if steps := job.get("steps"); steps != nil {
		if steps.kind != docSeq {
			return "", errors.New("job steps must be a sequence")
		}
		stepConditional := false
		for _, step := range steps.items {
			if step.kind != docMap {
				return "", errors.New("job step must be a mapping")
			}
			conditional, err := unevaluatedGitHubIf(step.get("if"))
			if err != nil {
				return "", err
			}
			stepConditional = stepConditional || conditional
		}
		if stepConditional {
			conditions = append(conditions, "step if is false or unevaluated")
		}
	}
	return strings.Join(conditions, ", "), nil
}

func unevaluatedGitHubIf(condition *docNode) (bool, error) {
	if condition == nil {
		return false, nil
	}
	if condition.kind != docScalar || strings.TrimSpace(condition.scalar) == "" {
		return false, errors.New("if must be a nonempty scalar")
	}
	return condition.scalar != "true", nil
}

func readGitHubPathEvent(name string, config *docNode) (GitHubPathEvent, error) {
	event := GitHubPathEvent{Name: name}
	if config == nil || config.kind == docNull {
		return event, nil
	}
	if config.kind != docMap {
		return event, errors.New("event must be a mapping or null")
	}
	paths, ignore := config.get("paths"), config.get("paths-ignore")
	if paths != nil && ignore != nil {
		return event, errors.New("paths and paths-ignore cannot be combined")
	}
	for _, key := range config.keys {
		if key != "paths" && key != "paths-ignore" {
			event.Condition = "event has non-path selectors"
		}
	}
	if ignore != nil {
		paths, event.Ignore = ignore, true
	}
	if paths == nil {
		return event, nil
	}
	if paths.kind != docSeq || len(paths.items) == 0 {
		return event, errors.New("path filter must be a nonempty sequence")
	}
	positive := false
	for _, item := range paths.items {
		if item.kind != docScalar {
			return event, errors.New("path filter patterns must be strings")
		}
		pattern, err := compileGitHubPathPattern(item.scalar)
		if err != nil || (pattern.negative && event.Ignore) {
			return event, errors.New("invalid or unsupported path filter pattern")
		}
		event.Patterns = append(event.Patterns, pattern)
		positive = positive || !pattern.negative
	}
	if !positive {
		return event, errors.New("paths needs a positive pattern")
	}
	return event, nil
}

func validatePathFilterYAML(src []byte) error {
	decoder := yaml.NewDecoder(bytes.NewReader(src))
	var doc yaml.Node
	if err := decoder.Decode(&doc); err != nil || len(doc.Content) != 1 {
		return errors.New("cannot decode workflow YAML")
	}
	for {
		var extra yaml.Node
		err := decoder.Decode(&extra)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil || len(extra.Content) != 1 || !emptyYAMLDoc(extra.Content[0]) {
			return errors.New("expected one workflow YAML document")
		}
	}
	root := doc.Content[0]
	if !unambiguousPathFilterYAML(root, 0) {
		return errors.New("workflow YAML contains ambiguous or unsupported nodes")
	}

	on := pathFilterYAMLField(root, "on")
	if on != nil && on.Kind == yaml.MappingNode {
		for i := 0; i < len(on.Content); i += 2 {
			switch on.Content[i].Value {
			case "push", "pull_request":
				config := on.Content[i+1]
				for _, key := range []string{"paths", "paths-ignore"} {
					if patterns := pathFilterYAMLField(config, key); patterns != nil && !pathFilterYAMLStrings(patterns) {
						return errors.New("path filter patterns must be strings")
					}
				}
			}
		}
	} else if on != nil && !pathFilterYAMLStrings(on) {
		return errors.New("workflow event names must be strings")
	}
	return nil
}

func unambiguousPathFilterYAML(node *yaml.Node, depth int) bool {
	if depth > 64 || node.Kind == yaml.AliasNode || node.Style&yaml.TaggedStyle != 0 {
		return false
	}
	if node.Kind == yaml.MappingNode {
		seen := make(map[string]bool)
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i]
			if key.Kind != yaml.ScalarNode || key.Tag != "!!str" || seen[key.Value] {
				return false
			}
			seen[key.Value] = true
		}
	}
	for _, child := range node.Content {
		if !unambiguousPathFilterYAML(child, depth+1) {
			return false
		}
	}
	return true
}

func pathFilterYAMLField(node *yaml.Node, field string) *yaml.Node {
	if node.Kind == yaml.MappingNode {
		for i := 0; i < len(node.Content); i += 2 {
			if node.Content[i].Value == field {
				return node.Content[i+1]
			}
		}
	}
	return nil
}

func pathFilterYAMLStrings(node *yaml.Node) bool {
	if node.Kind != yaml.SequenceNode {
		return node.Kind == yaml.ScalarNode && node.Tag == "!!str"
	}
	for _, item := range node.Content {
		if item.Kind != yaml.ScalarNode || item.Tag != "!!str" {
			return false
		}
	}
	return true
}

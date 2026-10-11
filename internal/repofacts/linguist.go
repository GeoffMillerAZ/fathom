package repofacts

// The linguist basis (RF-3): github-linguist/linguist's own answer to
// "is this file vendored, generated or documentation", read from its
// vendor.yml, documentation.yml and the path-only detectors of
// generated.rb, imported as plain data at a pinned revision the way LB-F48
// imported its languages (internal/langbench/linguist.go): ImportLinguist
// below, run by `go run ./internal/repofacts/linguistgen`, writes
// linguist.json, which is committed and embedded. Linguist is MIT licensed,
// Copyright (c) 2017 GitHub, Inc.; the data file carries the source, the
// revision, the licence and the copyright line.
//
// Only what Linguist decides from a path is imported. A generated.rb
// detector that reads the file's content, and a pattern Go's RE2 cannot
// compile (a lookbehind), is dropped and named in the data file's
// `dropped` list, never silently. The convention table (convention.go)
// imports none of this data.

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Linguist provenance recorded in the data file.
const (
	LinguistSource  = "github-linguist/linguist"
	LinguistLicence = "MIT"
)

// LinguistData is linguist.json.
type LinguistData struct {
	Source    string `json:"source"`
	Revision  string `json:"revision"`
	Licence   string `json:"licence"`
	Copyright string `json:"copyright"`
	// Vendor, Documentation and Generated are path regexps (RE2), each the
	// upstream pattern as written, a generated.rb file-name list rendered
	// as one.
	Vendor        []string `json:"vendor"`
	Documentation []string `json:"documentation"`
	Generated     []string `json:"generated"`
	// Dropped names every upstream rule not imported and why.
	Dropped []LinguistDrop `json:"dropped"`
}

// LinguistDrop is one upstream rule the import left out.
type LinguistDrop struct {
	From   string `json:"from"`
	Rule   string `json:"rule"`
	Reason string `json:"reason"`
}

var linguistRev = regexp.MustCompile(`^[0-9a-f]{40}$`)

// ImportLinguist builds the linguist basis' data from a Linguist checkout's
// lib/linguist/vendor.yml, lib/linguist/documentation.yml,
// lib/linguist/generated.rb and LICENSE at revision rev (40 hex).
func ImportLinguist(vendorYML, documentationYML, generatedRB, licence []byte, rev string) (LinguistData, error) {
	if !linguistRev.MatchString(rev) {
		return LinguistData{}, fmt.Errorf("linguist revision %q is not a 40-hex commit", rev)
	}
	if !bytes.Contains(licence, []byte("Permission is hereby granted, free of charge")) {
		return LinguistData{}, fmt.Errorf("linguist LICENSE is not the MIT licence text")
	}
	d := LinguistData{Source: LinguistSource, Revision: rev, Licence: LinguistLicence, Copyright: firstLine(licence, "Copyright")}
	var err error
	if d.Vendor, err = importPatternList("vendor.yml", vendorYML, &d.Dropped); err != nil {
		return LinguistData{}, err
	}
	if d.Documentation, err = importPatternList("documentation.yml", documentationYML, &d.Dropped); err != nil {
		return LinguistData{}, err
	}
	d.Generated = importGenerated(generatedRB, &d.Dropped)
	if len(d.Vendor) == 0 || len(d.Documentation) == 0 || len(d.Generated) == 0 {
		return LinguistData{}, fmt.Errorf("linguist import is empty (vendor %d, documentation %d, generated %d)", len(d.Vendor), len(d.Documentation), len(d.Generated))
	}
	return d, nil
}

func firstLine(b []byte, prefix string) string {
	for _, l := range strings.Split(string(b), "\n") {
		if l = strings.TrimSpace(l); strings.HasPrefix(l, prefix) {
			return l
		}
	}
	return ""
}

func importPatternList(from string, b []byte, dropped *[]LinguistDrop) ([]string, error) {
	var pats []string
	if err := yaml.Unmarshal(b, &pats); err != nil {
		return nil, fmt.Errorf("linguist %s: %w", from, err)
	}
	var out []string
	for _, p := range pats {
		if _, err := regexp.Compile(p); err != nil {
			*dropped = append(*dropped, LinguistDrop{From: from, Rule: p, Reason: "not RE2: " + err.Error()})
			continue
		}
		out = append(out, p)
	}
	return out, nil
}

var (
	rbDef        = regexp.MustCompile(`^    def ([a-z0-9_]+\?)\s*$`)
	rbNameMatch  = regexp.MustCompile(`^!!name\.match\(/(.*)/(i?)\)$`)
	rbExtInclude = regexp.MustCompile(`^\[((?:\s*'[^']+'\s*,?)+)\]\.include\?\(?extname\)?$`)
	rbCalled     = regexp.MustCompile(`([a-z0-9_]+\?)`)
)

// importGenerated reads generated.rb: the detectors generated? calls, each
// imported when its whole body is a path test (`!!name.match(/re/)`, joined
// by `||`, or `[...].include?(extname)`), dropped and named otherwise.
func importGenerated(rb []byte, dropped *[]LinguistDrop) []string {
	bodies := map[string][]string{}
	var cur string
	for _, raw := range strings.Split(string(rb), "\n") {
		if m := rbDef.FindStringSubmatch(raw); m != nil {
			cur = m[1]
			continue
		}
		if cur == "" {
			continue
		}
		if raw == "    end" {
			cur = ""
			continue
		}
		l := strings.TrimSpace(raw)
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		bodies[cur] = append(bodies[cur], l)
	}
	var called []string
	seen := map[string]bool{}
	for _, l := range bodies["generated?"] {
		for _, m := range rbCalled.FindAllString(l, -1) {
			if !seen[m] {
				seen[m] = true
				called = append(called, m)
			}
		}
	}
	var out []string
	for _, name := range called {
		body := bodies[name]
		re, reason := generatedPathRule(body)
		if reason != "" {
			*dropped = append(*dropped, LinguistDrop{From: "generated.rb", Rule: name, Reason: reason})
			continue
		}
		out = append(out, re)
	}
	return out
}

func generatedPathRule(body []string) (string, string) {
	if len(body) == 0 {
		return "", "no body found"
	}
	if len(body) == 1 {
		if m := rbExtInclude.FindStringSubmatch(body[0]); m != nil {
			var exts []string
			for _, e := range strings.Split(m[1], ",") {
				e = strings.Trim(strings.TrimSpace(e), "'")
				if e != "" {
					exts = append(exts, regexp.QuoteMeta(e))
				}
			}
			return "(?:" + strings.Join(exts, "|") + ")$", ""
		}
	}
	expr := strings.Join(body, " ")
	var alts []string
	for _, part := range strings.Split(expr, "||") {
		m := rbNameMatch.FindStringSubmatch(strings.TrimSpace(part))
		if m == nil {
			return "", "reads the file's content or an unparsed form"
		}
		re := m[1]
		if m[2] == "i" {
			re = "(?i:" + re + ")"
		}
		if _, err := regexp.Compile(re); err != nil {
			return "", "not RE2: " + err.Error()
		}
		alts = append(alts, re)
	}
	if len(alts) == 1 {
		return alts[0], ""
	}
	return "(?:" + strings.Join(alts, ")|(?:") + ")", ""
}

// MarshalLinguist renders the data file: sorted dropped rows, two-space
// indent, trailing newline, so a regeneration diffs cleanly.
func MarshalLinguist(d LinguistData) ([]byte, error) {
	sort.SliceStable(d.Dropped, func(i, j int) bool {
		if d.Dropped[i].From != d.Dropped[j].From {
			return d.Dropped[i].From < d.Dropped[j].From
		}
		return d.Dropped[i].Rule < d.Dropped[j].Rule
	})
	b, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

//go:embed linguist.json
var linguistJSON []byte

// linguistRes is the embedded data compiled once: one alternation per role.
var linguistRes = compileLinguist(linguistJSON)

type linguistMatchers struct {
	revision                      string
	vendor, generated, documented *regexp.Regexp
}

func compileLinguist(b []byte) linguistMatchers {
	var d LinguistData
	if err := json.Unmarshal(b, &d); err != nil {
		panic("repofacts: linguist.json: " + err.Error())
	}
	join := func(pats []string) *regexp.Regexp {
		if len(pats) == 0 {
			return nil
		}
		return regexp.MustCompile("(?:" + strings.Join(pats, ")|(?:") + ")")
	}
	return linguistMatchers{revision: d.Revision, vendor: join(d.Vendor), generated: join(d.Generated), documented: join(d.Documentation)}
}

// LinguistRevision is the pinned Linguist commit the linguist basis reads.
func LinguistRevision() string { return linguistRes.revision }

// linguistRole is the linguist basis: vendored, then generated, then
// documentation, the first whose upstream patterns match rel and whose
// attribute the repository did not clear.
func linguistRole(rel string, attrs Attributes) (RoleName, bool) {
	m := linguistRes
	switch {
	case attrs.allows(Vendored) && m.vendor != nil && m.vendor.MatchString(rel):
		return Vendored, true
	case attrs.allows(Generated) && m.generated != nil && m.generated.MatchString(rel):
		return Generated, true
	case attrs.allows(Docs) && m.documented != nil && m.documented.MatchString(rel):
		return Docs, true
	}
	return "", false
}

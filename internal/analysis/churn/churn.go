// Package churn analyses git churn data to identify hotspot files.
//
// The core computation (Hotspots, ParseGitLog) is pure and deterministic when
// given the same input; no filesystem or git process access is performed here.
// A thin loader (RunGitLog) is provided for callers that want to drive a real
// git process, but it is not exercised by the unit tests.
//
// All exported functions are safe for concurrent use.
package churn

import (
	"bufio"
	"bytes"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ─── Types ───────────────────────────────────────────────────────────────────

// Commit records a single file-level change from a git log entry.
type Commit struct {
	File      string
	Additions int
	Deletions int
	Author    string
	When      time.Time
}

// Hotspot aggregates churn statistics for a single file.
type Hotspot struct {
	File    string
	Commits int     // number of commits that touched this file
	Authors int     // number of distinct authors
	Churn   int     // total lines changed (additions + deletions)
	Score   float64 // recency-weighted score (higher → hotter)
}

// ─── Hotspots ────────────────────────────────────────────────────────────────

// Hotspots aggregates the supplied commits into per-file Hotspot records and
// returns them sorted descending by Score.
//
// Score formula:
//
//	score = Σ (churn_i × recencyWeight(age_i))
//
// where recencyWeight(age) = exp(-λ × age_in_days) with λ = ln(2)/180
// (half-life ≈ 180 days).  This is a pure function; now must be injected by
// the caller so results are deterministic in tests.
func Hotspots(commits []Commit, now time.Time) []Hotspot {
	const lambda = 0.693147180559945 / 180.0 // ln(2) / 180 days

	type agg struct {
		commits int
		authors map[string]struct{}
		churn   int
		score   float64
	}

	m := make(map[string]*agg)

	for _, c := range commits {
		a, ok := m[c.File]
		if !ok {
			a = &agg{authors: make(map[string]struct{})}
			m[c.File] = a
		}
		a.commits++
		if c.Author != "" {
			a.authors[c.Author] = struct{}{}
		}
		lines := c.Additions + c.Deletions
		a.churn += lines

		ageDays := now.Sub(c.When).Hours() / 24
		if ageDays < 0 {
			ageDays = 0
		}
		weight := math.Exp(-lambda * ageDays)
		a.score += float64(lines) * weight
	}

	out := make([]Hotspot, 0, len(m))
	for file, a := range m {
		out = append(out, Hotspot{
			File:    file,
			Commits: a.commits,
			Authors: len(a.authors),
			Churn:   a.churn,
			Score:   a.score,
		})
	}

	// Sort descending by Score; break ties lexicographically by File for
	// determinism.
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].File < out[j].File
	})

	return out
}

// ─── ParseGitLog ─────────────────────────────────────────────────────────────

// ParseGitLog parses the output of:
//
//	git log --numstat --format="author:%ae%ndate:%cI"
//
// The expected interleaved format is:
//
//	author:<email>
//	date:<RFC3339 timestamp>
//	<additions>\t<deletions>\t<file>
//	...blank line...
//	(next commit)
//
// Lines that cannot be parsed are silently skipped so the function is
// tolerant of git version differences.  The result slice is in the order the
// commits appear in the log output (most-recent-first when using the default
// git ordering).
func ParseGitLog(out []byte) []Commit {
	scanner := bufio.NewScanner(bytes.NewReader(out))

	var commits []Commit
	var curAuthor string
	var curWhen time.Time

	for scanner.Scan() {
		line := scanner.Text()

		switch {
		case strings.HasPrefix(line, "author:"):
			curAuthor = strings.TrimPrefix(line, "author:")

		case strings.HasPrefix(line, "date:"):
			raw := strings.TrimPrefix(line, "date:")
			t, err := time.Parse(time.RFC3339, raw)
			if err != nil {
				// Try without timezone offset (some git builds emit slightly
				// different formats).
				t, err = time.Parse("2006-01-02T15:04:05-07:00", raw)
				if err != nil {
					curWhen = time.Time{}
					continue
				}
			}
			curWhen = t

		case line == "" || strings.HasPrefix(line, "commit "):
			// Blank lines / commit hash lines: reset for next commit header.
			// (We don't use the hash.)

		default:
			// Numstat line: "<add>\t<del>\t<file>"
			parts := strings.SplitN(line, "\t", 3)
			if len(parts) != 3 {
				continue
			}
			add, err1 := strconv.Atoi(parts[0])
			del, err2 := strconv.Atoi(parts[1])
			if err1 != nil || err2 != nil {
				// Binary files show "-" — skip.
				continue
			}
			file := strings.TrimSpace(parts[2])
			if file == "" {
				continue
			}
			commits = append(commits, Commit{
				File:      file,
				Additions: add,
				Deletions: del,
				Author:    curAuthor,
				When:      curWhen,
			})
		}
	}

	return commits
}

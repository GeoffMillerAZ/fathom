package fusion

// git.go — the pipeline's git reads: realGitHead and realGitLog (the HEAD sha and the
// commit log of a service root) and parseGitLog (the log into store.Commit rows). Split
// out of pipeline.go unchanged.

import (
	"bytes"
	"context"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/geoffmilleraz/fathom/internal/gitro"
	"github.com/geoffmilleraz/fathom/internal/sdd"
	"github.com/geoffmilleraz/fathom/internal/store"
)

// realGitHead resolves HEAD for the given directory path from the ref files
// (RT-101: gitro.Head, no fork for a readable layout). Returns an empty
// string when the path is in no work tree or HEAD cannot be read. Never
// panics, never returns an error.
func realGitHead(path string) string {
	if path == "" {
		return ""
	}
	sha, err := gitro.Head(context.Background(), path)
	if err != nil {
		return ""
	}
	return sha
}

// realGitLog shells out to git to get the last 500 commits for the given
// directory path. Returns an empty string on any error (best-effort).
//
// Format: \x00<hash>\x1f<author>\x1f<date>\x00<full raw body, %B>\x00,
// followed by --name-only's file list before the next record's leading
// \x00. %B — not %s — captures the FULL commit message (subject + blank
// line + body + trailers), so E2-1 widens commit ingestion beyond the old
// subject-only capture (0/763 stored messages contained a newline before
// this).
//
// The framing byte is NUL (%x00) — the ONE byte git guarantees can never
// appear in a commit message (git rejects NUL at commit time; every other
// control byte, including \x02/\x03/\x1f, is preserved verbatim). An
// earlier draft framed records with \x02/\x03 sentinels, which an attacker
// could embed in a crafted commit message to inject a forged record — a
// forged hash, forged author, and forged Co-Authored-By trailer that the
// authorship oracle would then mint a TierVerified claim for. NUL framing
// closes that: no message, author name, date, or filename can contain the
// framing byte, so record boundaries are unforgeable (see parseGitLog,
// which adds full-length hash validation as belt-and-braces).
func realGitLog(path string) string {
	if path == "" {
		return ""
	}
	var out bytes.Buffer
	cmd := exec.Command("git", "-C", path, "log",
		"--pretty=format:%x00%H%x1f%an%x1f%aI%x00%B%x00",
		"--name-only",
		"-n", "500")
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return ""
	}
	return out.String()
}

// commitHashPattern validates a full git object name: 40 hex chars (SHA-1)
// or 64 (SHA-256 repos). %H always emits the full lowercase hash, so
// anything else in the hash field means a corrupt/truncated record —
// belt-and-braces on top of the unforgeable NUL framing (see realGitLog).
var commitHashPattern = regexp.MustCompile(`^[0-9a-f]{40}(?:[0-9a-f]{24})?$`)

// parseGitLog parses output of:
//
//	git log --pretty=format:%x00%H%x1f%an%x1f%aI%x00%B%x00 --name-only -n 500
//
// Raw shape per record: a leading NUL, the header (hash\x1fauthor\x1fdate),
// a NUL, the raw body (%B), a NUL, then --name-only's file list up to the
// next record's leading NUL. Splitting the whole output on NUL therefore
// yields one empty leading segment followed by positional TRIPLES of
// (header, body, files-chunk) — and because git guarantees NUL can never
// appear inside a commit message, author name, ISO date, or filename, no
// crafted commit content can shift those boundaries: bytes like \x02/\x03/
// \x1f inside a message are inert content here, not framing (the forged-
// record injection class the earlier STX/ETX framing was vulnerable to;
// TestParseGitLog_ForgedRecordInjection pins this closed). A record whose
// hash field fails commitHashPattern is skipped; a tail-truncated final
// record (fewer than three remaining segments) is dropped.
//
// Commit.Message carries the FULL raw commit message (subject, body, and
// trailers) so a downstream deterministic pass (E2-2's authorship oracle)
// can find trailers such as "Co-Authored-By:" wherever they fall.
func parseGitLog(serviceID, raw string) []store.Commit {
	if raw == "" {
		return nil
	}
	segs := strings.Split(raw, "\x00")
	var commits []store.Commit
	// segs[0] is whatever precedes the first record's leading NUL ("" in
	// well-formed output); records are the positional triples after it.
	for i := 1; i+2 < len(segs); i += 3 {
		header, body, filesChunk := segs[i], segs[i+1], segs[i+2]

		// header: hash\x1fauthor\x1fdate
		parts := strings.SplitN(header, "\x1f", 3)
		if len(parts) < 3 {
			continue
		}
		hash, author, dateStr := parts[0], parts[1], parts[2]
		if !commitHashPattern.MatchString(hash) {
			continue
		}
		// git's pretty format appends its own trailing newline after %B; the
		// body itself may or may not already end in one. Trim it so Message
		// never carries a spurious trailing blank line, while any INTERNAL
		// blank lines (paragraph breaks, the subject/body separator) survive.
		message := strings.TrimRight(body, "\n")

		var dateNano int64
		if t, err := time.Parse(time.RFC3339, dateStr); err == nil {
			dateNano = t.UnixNano()
		} else if t, err := time.Parse(time.RFC3339Nano, dateStr); err == nil {
			dateNano = t.UnixNano()
		}
		var files []string
		for _, l := range strings.Split(filesChunk, "\n") {
			l = strings.TrimSpace(l)
			if l != "" {
				files = append(files, l)
			}
		}
		// Scanning the full message (not just the subject) means a spec ID
		// mentioned only in the body is now found too — a strict widening of
		// the prior subject-only recall, never a narrowing.
		specIDs, _ := sdd.ExtractRefsFromText(message, "")
		commits = append(commits, store.Commit{
			ServiceID: serviceID,
			Hash:      hash,
			Author:    author,
			Date:      dateNano,
			Message:   message,
			Files:     files,
			SpecIDs:   specIDs,
		})
	}
	return commits
}

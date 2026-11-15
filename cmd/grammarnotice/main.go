// Command grammarnotice is an offline release tool. Source archives must already
// be present in the supplied directory as <lock tarball_sha256>.tar.gz. It never
// downloads sources, substitutes licence templates, or changes the runtime reader.
package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/extractor/tsengine"
)

const noticeName = "THIRD-PARTY-NOTICES.txt"
const maxNoticeBytes = 16 << 20

type sourceText struct {
	name string
	body []byte
}

func isHex(s string, size int) bool {
	if len(s) != size || s != strings.ToLower(s) {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

// GenerateNotice renders only third-party grammars (W1-3), in blob order. The
// source archive's lock digest authenticates the pinned bytes, including their
// attribution and licence text. No text is reconstructed from an SPDX id.
func GenerateNotice(lockBytes []byte, sourceCache string) ([]byte, error) {
	lock, err := tsengine.ParseGrammarLock(lockBytes)
	if err != nil {
		return nil, err
	}
	if len(lock.Grammars) == 0 {
		return nil, errors.New("empty grammar inventory")
	}
	sort.Slice(lock.Grammars, func(i, j int) bool { return lock.Grammars[i].Blob < lock.Grammars[j].Blob })
	canonical, err := json.Marshal(lock)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	fmt.Fprintf(&out, "Third-party grammar notices\nLock provenance (canonical JSON SHA-256): %x\n", sha256.Sum256(canonical))
	var problems []error
	for i, row := range lock.Grammars {
		if row.Blob == "" || strings.ContainsAny(row.Blob, "\r\n") || (i > 0 && row.Blob == lock.Grammars[i-1].Blob) {
			return nil, fmt.Errorf("invalid or duplicate grammar name %q", row.Blob)
		}
		parts := strings.Split(row.Repo, "/")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" || strings.ContainsAny(row.Repo, "\r\n?# \\:") || !isHex(row.Commit, 40) || !isHex(row.TarballSHA256, 64) || !tsengine.LicenceAllowed(row.Licence) {
			return nil, fmt.Errorf("%s: invalid source pin or licence", row.Blob)
		}
		texts, err := readPinnedSource(row, sourceCache)
		if err != nil {
			problems = append(problems, fmt.Errorf("%s: %w", row.Blob, err))
			continue
		}
		fmt.Fprintf(&out, "\n=== Grammar: %s ===\nUpstream: https://github.com/%s\nCommit: %s\nSPDX: %s\nSource archive SHA-256: %s\n", row.Blob, row.Repo, row.Commit, row.Licence, row.TarballSHA256)
		for _, text := range texts {
			fmt.Fprintf(&out, "\n--- %s (%d bytes, copied verbatim) ---\n", text.name, len(text.body))
			out.Write(text.body)
			out.WriteString("\n--- end source file ---\n")
		}
	}
	if len(problems) > 0 {
		return nil, errors.Join(problems...)
	}
	if out.Len() > maxNoticeBytes {
		return nil, errors.New("generated notice exceeds size limit")
	}
	return out.Bytes(), nil
}

func readPinnedSource(row tsengine.GrammarLockRow, cache string) ([]sourceText, error) {
	file := filepath.Join(cache, row.TarballSHA256+".tar.gz")
	f, err := os.Open(file)
	if err != nil {
		return nil, fmt.Errorf("missing pinned source archive %s: %w", file, err)
	}
	defer f.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, f); err != nil {
		return nil, err
	}
	if hex.EncodeToString(hash.Sum(nil)) != row.TarballSHA256 {
		return nil, fmt.Errorf("source archive %s SHA-256 differs from lock", file)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	var texts []sourceText
	names := map[string]bool{}
	licenceFound := false
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if h.Typeflag != tar.TypeReg && h.Typeflag != tar.TypeRegA {
			continue
		}
		// GitHub source archives have one enclosing directory. Nothing is extracted.
		_, name, ok := strings.Cut(h.Name, "/")
		if !ok || path.Clean(name) != name || strings.HasPrefix(name, "../") {
			continue
		}
		base := strings.ToUpper(path.Base(name))
		licence := strings.HasPrefix(base, "LICENSE") || strings.HasPrefix(base, "LICENCE") || strings.HasPrefix(base, "COPYING")
		notice := row.Licence == "Apache-2.0" && strings.HasPrefix(base, "NOTICE")
		if !licence && !notice {
			continue
		}
		// Match the existing build's top-level/src/queries licence scope, including
		// explicitly locked grammar subdirectories. Exclude unrelated vendored trees.
		dir := path.Dir(name)
		relevant := dir == "." || strings.HasPrefix(name, "src/") || strings.HasPrefix(name, "queries/")
		for _, sub := range row.Subdirs {
			if dir == sub || strings.HasPrefix(name, sub+"/src/") || strings.HasPrefix(name, sub+"/queries/") {
				relevant = true
			}
		}
		if !relevant {
			continue
		}
		if names[name] {
			return nil, fmt.Errorf("duplicate licence/NOTICE source file %s", name)
		}
		names[name] = true
		if h.Size <= 0 || h.Size > maxNoticeBytes {
			return nil, fmt.Errorf("empty or oversized licence/NOTICE source file %s", name)
		}
		body, err := io.ReadAll(io.LimitReader(tr, maxNoticeBytes+1))
		if err != nil {
			return nil, err
		}
		if len(bytes.TrimSpace(body)) == 0 {
			return nil, fmt.Errorf("empty licence/NOTICE source file %s", name)
		}
		texts = append(texts, sourceText{name, body})
		licenceFound = licenceFound || licence
	}
	if !licenceFound {
		return nil, fmt.Errorf("pinned source %s has no licence file", row.Repo)
	}
	// Consume the gzip trailer too: a corrupt cached archive is never evidence.
	if _, err := io.Copy(io.Discard, gz); err != nil {
		return nil, err
	}
	sort.Slice(texts, func(i, j int) bool { return texts[i].name < texts[j].name })
	return texts, nil
}

// CheckReleaseArchiveNotice requires exactly one regular, root-level notice
// whose bytes match the notice regenerated from the current lock and sources.
// Presence of a receipt, filename or SPDX identifier alone is insufficient.
func CheckReleaseArchiveNotice(archive string, expected []byte) error {
	check := func() error {
		f, err := os.Open(archive)
		if err != nil {
			return err
		}
		defer f.Close()
		gz, err := gzip.NewReader(f)
		if err != nil {
			return err
		}
		defer gz.Close()
		tr := tar.NewReader(gz)
		var notice []byte
		found := false
		for {
			h, err := tr.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				return err
			}
			if strings.TrimPrefix(h.Name, "./") != noticeName {
				continue
			}
			if found {
				return errors.New("duplicate notice")
			}
			found = true
			if h.Typeflag != tar.TypeReg && h.Typeflag != tar.TypeRegA {
				return errors.New("notice must be a regular file")
			}
			if h.Size < 0 || h.Size > maxNoticeBytes {
				return errors.New("notice exceeds size limit")
			}
			notice, err = io.ReadAll(io.LimitReader(tr, maxNoticeBytes+1))
			if err != nil {
				return err
			}
		}
		if _, err := io.Copy(io.Discard, gz); err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("missing notice %s", noticeName)
		}
		for _, line := range strings.Split(string(expected), "\n") {
			if strings.HasPrefix(line, "=== Grammar: ") && !bytes.Contains(notice, []byte(line+"\n")) {
				return fmt.Errorf("notice missing locked grammar entry %s", line)
			}
		}
		if !bytes.Equal(notice, expected) {
			return errors.New("notice differs from current lock and pinned source text (stale provenance or missing licence/NOTICE text)")
		}
		return nil
	}
	if err := check(); err != nil {
		return fmt.Errorf("%s: %w", archive, err)
	}
	return nil
}

func run(args []string) int {
	if len(args) < 4 || (args[0] != "generate" && args[0] != "check") || (args[0] == "generate" && len(args) != 4) {
		fmt.Fprintln(os.Stderr, "usage: grammarnotice generate <lock> <source-cache> <output> | check <lock> <source-cache> <archive>...")
		return 2
	}
	lock, err := os.ReadFile(args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	expected, err := GenerateNotice(lock, args[2])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if args[0] == "generate" {
		if err := os.MkdirAll(filepath.Dir(args[3]), 0755); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
		if err := os.WriteFile(args[3], expected, 0644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 2
		}
		return 0
	}
	status := 0
	for _, archive := range args[3:] {
		if err := CheckReleaseArchiveNotice(archive, expected); err != nil {
			fmt.Fprintln(os.Stderr, err)
			status = 1
		} else {
			fmt.Printf("%s: grammar notice verified\n", archive)
		}
	}
	return status
}

func main() { os.Exit(run(os.Args[1:])) }

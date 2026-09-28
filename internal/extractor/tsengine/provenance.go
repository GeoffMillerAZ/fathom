package tsengine

import (
	"fmt"
	"strings"
)

// The generated block's markers in grammars/PROVENANCE.md.
const (
	provenanceBegin = "<!-- BEGIN generated from grammars.lock.json by RenderProvenanceTables; do not edit -->"
	provenanceEnd   = "<!-- END generated -->"
)

// RenderProvenanceTables renders the lock as PROVENANCE.md's tables (LB-F28):
// the pinned sources and the committed blobs' digests. Rows follow the
// lock's order (sorted by blob, which VerifyGrammarLock enforces).
func RenderProvenanceTables(lock GrammarLock) string {
	var b strings.Builder
	b.WriteString("### Pinned sources\n\n")
	b.WriteString("| Blob | Repository | Tag | Commit | Licence | Entry points | Tarball sha256 |\n")
	b.WriteString("|---|---|---|---|---|---|---|\n")
	c := lock.Core
	fmt.Fprintf(&b, "| core | %s | %s | `%s` | %s | — | `%s` |\n", c.Repo, c.Tag, c.Commit, c.Licence, c.TarballSHA256)
	for _, r := range lock.Grammars {
		eps := make([]string, len(r.EntryPoints))
		for i, ep := range r.EntryPoints {
			eps[i] = "`" + ep + "`"
		}
		fmt.Fprintf(&b, "| `%s` | %s | %s | `%s` | %s | %s | `%s` |\n",
			r.Blob, r.Repo, r.Tag, r.Commit, r.Licence, strings.Join(eps, ", "), r.TarballSHA256)
	}

	b.WriteString("\n### Committed blobs\n\n")
	b.WriteString("| Blob | Grammar dirs | Scanner | ABI | Module bytes | `.wasm.gz` bytes | Module sha256 |\n")
	b.WriteString("|---|---|---|--:|--:|--:|---|\n")
	var raw, z int64
	for _, r := range lock.Grammars {
		dirs := "(root)"
		if len(r.Subdirs) > 0 {
			dirs = strings.Join(r.Subdirs, ", ")
		}
		scanner := "no"
		if r.Scanner {
			scanner = "yes"
		}
		fmt.Fprintf(&b, "| `%s` | %s | %s | %d | %s | %s | `%s` |\n",
			r.Blob, dirs, scanner, r.ABI, groupDigits(r.BlobBytes), groupDigits(r.GzBytes), r.BlobSHA256)
		raw += r.BlobBytes
		z += r.GzBytes
	}
	fmt.Fprintf(&b, "| **total** | | | | %s | %s | |\n", groupDigits(raw), groupDigits(z))

	t := lock.Toolchain
	fmt.Fprintf(&b, "\nToolchain that produced the committed blobs: zig %s, wasm-opt %s", t.Zig, t.WasmOpt)
	if t.TreeSitterCLI != "" {
		fmt.Fprintf(&b, ", tree-sitter-cli %s", t.TreeSitterCLI)
	}
	b.WriteString(".\n")
	return b.String()
}

// ReplaceProvenanceBlock swaps the generated block between the markers in
// doc for block.
func ReplaceProvenanceBlock(doc, block string) (string, error) {
	i := strings.Index(doc, provenanceBegin)
	j := strings.Index(doc, provenanceEnd)
	if i < 0 || j < i {
		return "", fmt.Errorf("tsengine: PROVENANCE.md lacks the generated-block markers")
	}
	return doc[:i+len(provenanceBegin)] + "\n\n" + block + "\n" + doc[j:], nil
}

// groupDigits formats n with thousands separators (1,483,348).
func groupDigits(n int64) string {
	s := fmt.Sprint(n)
	if n < 0 {
		return "-" + groupDigits(-n)
	}
	var out []byte
	for i := range len(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ',')
		}
		out = append(out, s[i])
	}
	return string(out)
}

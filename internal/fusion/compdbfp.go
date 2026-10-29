package fusion

// compdbfp.go — the identity of the compile databases an include-path
// resolution reads, for readings that must say whether a re-measure saw the
// same build inputs (indexbench lang-retime). It reuses compDatabase, so the
// fingerprint covers exactly what the resolver consumes.

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
)

// CompileDBIdentity is what the compile databases under a root contribute to
// extraction: how many were read, how many translation units they name, and a
// fingerprint of the per-file include roots and defines, independent of the
// absolute root. Fingerprint is "" when no database was read.
type CompileDBIdentity struct {
	Databases   int    `json:"databases"`
	Entries     int    `json:"entries"`
	Fingerprint string `json:"fingerprint,omitempty"`
}

// CompileDBFingerprint reads the compile databases under root the way the
// include resolver does and returns their identity.
func CompileDBFingerprint(root string) (CompileDBIdentity, error) {
	env := newResolveEnv(root, nil, nil)
	db := env.compDatabase()
	id := CompileDBIdentity{Databases: len(db.names), Entries: len(db.entries)}
	if id.Databases == 0 {
		return id, nil
	}
	lines := make([]string, 0, len(db.entries))
	for _, ce := range db.entries {
		if ce.file == "" {
			continue
		}
		var defs []string
		for name, m := range ce.defs {
			if m.defined {
				defs = append(defs, "D"+name+"="+m.value)
			} else {
				defs = append(defs, "U"+name)
			}
		}
		sort.Strings(defs)
		lines = append(lines, ce.file+"\tquote="+strings.Join(ce.quote, ",")+"\tangle="+strings.Join(ce.angle, ",")+"\tdefs="+strings.Join(defs, ","))
	}
	sort.Strings(lines)
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n") + "\n"))
	id.Fingerprint = hex.EncodeToString(sum[:])
	return id, nil
}

package index

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// IntegrityReport summarizes a structural + content-addressing check of a built
// index — everything verifiable from the index file alone, without the source
// dump. A clean report (OK) proves the corpus is internally consistent: every
// blob round-trips through zstd, every blob's bytes hash to its own
// content_hash (the content-addressing invariant), the FTS shadow table has one
// row per variant, and no variant points at a missing blob.
type IntegrityReport struct {
	Components       int      `json:"components"`
	Variants         int      `json:"variants"`
	Blobs            int      `json:"blobs"`
	FTSRows          int      `json:"fts_rows"`
	BlobsChecked     int      `json:"blobs_checked"`
	HashMismatch     int      `json:"hash_mismatch"`     // decoded bytes did not hash to content_hash
	LenMismatch      int      `json:"len_mismatch"`      // raw_len disagreed with decoded length
	DecodeErrors     int      `json:"decode_errors"`     // zstd failed to decompress
	DanglingVariants int      `json:"dangling_variants"` // variant.content_hash with no blob row
	Problems         []string `json:"problems,omitempty"`
}

// OK reports whether the index passed every integrity check.
func (r IntegrityReport) OK() bool {
	return r.HashMismatch == 0 && r.LenMismatch == 0 && r.DecodeErrors == 0 &&
		r.DanglingVariants == 0 && r.FTSRows == r.Variants
}

const maxVerifyProblems = 20

// Verify checks the index against itself: counts, FTS/variant parity, blob
// referential integrity, and — for every blob — that it decompresses and that
// its bytes hash back to the content_hash it is stored under.
func (ix *Index) Verify() (IntegrityReport, error) {
	var rep IntegrityReport
	var err error
	if rep.Components, rep.Variants, rep.Blobs, err = ix.Counts(); err != nil {
		return rep, err
	}
	if err = ix.db.QueryRow(`SELECT COUNT(*) FROM fts_variant`).Scan(&rep.FTSRows); err != nil {
		return rep, fmt.Errorf("twplus/index: fts count: %w", err)
	}
	if rep.FTSRows != rep.Variants {
		rep.Problems = append(rep.Problems, fmt.Sprintf("fts rows %d != variants %d", rep.FTSRows, rep.Variants))
	}
	if err = ix.db.QueryRow(`SELECT COUNT(*) FROM variant v
		LEFT JOIN blob b ON b.content_hash = v.content_hash
		WHERE b.content_hash IS NULL`).Scan(&rep.DanglingVariants); err != nil {
		return rep, fmt.Errorf("twplus/index: dangling check: %w", err)
	}
	if rep.DanglingVariants > 0 {
		rep.Problems = append(rep.Problems, fmt.Sprintf("%d variants reference a missing blob", rep.DanglingVariants))
	}

	rows, err := ix.db.Query(`SELECT content_hash, code_zstd, raw_len FROM blob`)
	if err != nil {
		return rep, fmt.Errorf("twplus/index: scan blobs: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			hash   string
			packed []byte
			rawLen int
		)
		if err := rows.Scan(&hash, &packed, &rawLen); err != nil {
			return rep, err
		}
		rep.BlobsChecked++
		code, derr := ix.dec.DecodeAll(packed, make([]byte, 0, rawLen))
		if derr != nil {
			rep.DecodeErrors++
			rep.note("blob %s: decode failed: %v", shortHash(hash), derr)
			continue
		}
		if len(code) != rawLen {
			rep.LenMismatch++
			rep.note("blob %s: raw_len %d != decoded %d", shortHash(hash), rawLen, len(code))
		}
		sum := sha256.Sum256(code)
		if got := hex.EncodeToString(sum[:]); got != hash {
			rep.HashMismatch++
			rep.note("blob %s: bytes hash to %s — content-addressing violated", shortHash(hash), shortHash(got))
		}
	}
	return rep, rows.Err()
}

func (r *IntegrityReport) note(format string, args ...any) {
	if len(r.Problems) < maxVerifyProblems {
		r.Problems = append(r.Problems, fmt.Sprintf(format, args...))
	}
}

// VariantKey identifies one variant by its component full_name and three axes.
type VariantKey struct {
	FullName        string
	Framework       string
	TailwindVersion int
	Mode            string
}

// VariantHashes maps every indexed variant to the content_hash recorded for it.
// Because content_hash is sha256(code), a caller that recomputes the hash from
// the source dump and finds it equal here has proven the indexed code is
// byte-identical to the source — without decompressing the blob.
func (ix *Index) VariantHashes() (map[VariantKey]string, error) {
	rows, err := ix.db.Query(`SELECT c.full_name, v.framework, v.tailwind_version, v.mode, v.content_hash
		FROM variant v JOIN component c ON c.id = v.component_id`)
	if err != nil {
		return nil, fmt.Errorf("twplus/index: variant hashes: %w", err)
	}
	defer rows.Close()
	out := make(map[VariantKey]string)
	for rows.Next() {
		var (
			k VariantKey
			h string
		)
		if err := rows.Scan(&k.FullName, &k.Framework, &k.TailwindVersion, &k.Mode, &h); err != nil {
			return nil, err
		}
		out[k] = h
	}
	return out, rows.Err()
}

func shortHash(h string) string {
	if len(h) > 12 {
		return h[:12]
	}
	return h
}

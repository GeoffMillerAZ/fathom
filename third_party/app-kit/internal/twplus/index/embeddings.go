package index

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/ncruces/go-sqlite3/driver"
	"github.com/ncruces/go-sqlite3/ext/vec1"

	"github.com/geoffmilleraz/app-kit/appkit/search"
	"github.com/geoffmilleraz/app-kit/internal/twplus/embed"
)

// Embeddings live in a SEPARATE SQLite file from the corpus index, not inside
// it: `appkit data build` stays embedder-free and deterministic, and adding (or
// re-running) `appkit data embed` never rewrites the corpus. The vec1 virtual
// table's rowid is the corpus variant.id, so a KNN result maps straight back to
// a variant. vec1 is a flat (brute-force) index — exact, no ANN tuning, correct
// at the corpus's ~10k vectors (component-pipeline.md §10).

// EmbedTarget is one variant to embed: its id (the vec1 rowid) and the text the
// embedder should encode (the same taxonomy+axes+classes string BM25 indexes).
type EmbedTarget struct {
	VariantID int64
	EmbedText string
}

// EmbedTargets enumerates every variant's id and embed-text, ordered by id, so
// `appkit data embed` can batch them through an Embedder.
func (ix *Index) EmbedTargets() ([]EmbedTarget, error) {
	rows, err := ix.db.Query(`SELECT id, embed_text FROM variant ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("twplus/index: embed targets: %w", err)
	}
	defer rows.Close()
	var out []EmbedTarget
	for rows.Next() {
		var t EmbedTarget
		if err := rows.Scan(&t.VariantID, &t.EmbedText); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// BuildEmbeddings embeds every variant's text with e and writes an embeddings
// file at outPath, batching requests and learning the dimensionality from the
// first response. It is the offline half of semantic search — `appkit data
// embed` is a thin wrapper. Returns the number of vectors written.
func (ix *Index) BuildEmbeddings(ctx context.Context, e embed.Embedder, outPath string, batch int) (int, error) {
	if batch <= 0 {
		batch = 64
	}
	targets, err := ix.EmbedTargets()
	if err != nil {
		return 0, err
	}
	if len(targets) == 0 {
		return 0, fmt.Errorf("twplus/index: no variants to embed (build the corpus first)")
	}
	var b *EmbedBuilder
	defer func() {
		if b != nil {
			b.Close()
		}
	}()
	for start := 0; start < len(targets); start += batch {
		end := min(start+batch, len(targets))
		chunk := targets[start:end]
		texts := make([]string, len(chunk))
		for i, t := range chunk {
			texts[i] = t.EmbedText
		}
		vecs, err := e.Embed(ctx, texts)
		if err != nil {
			return 0, err
		}
		if len(vecs) != len(chunk) {
			return 0, fmt.Errorf("twplus/index: embedder returned %d vectors for %d inputs", len(vecs), len(chunk))
		}
		if b == nil { // first batch establishes the dimensionality
			b, err = NewEmbedBuilder(outPath, e.Model(), len(vecs[0]), "")
			if err != nil {
				return 0, err
			}
		}
		for i, t := range chunk {
			if err := b.Add(t.VariantID, vecs[i]); err != nil {
				return 0, err
			}
		}
	}
	n, err := b.Finalize()
	b = nil
	return n, err
}

// EmbedBuilder writes an embeddings file: a meta row pinning the model and a
// vec1 table of one vector per variant.
type EmbedBuilder struct {
	db    *sql.DB
	tx    *sql.Tx
	ins   *sql.Stmt
	dims  int
	added int
}

// NewEmbedBuilder creates a fresh embeddings file pinned to model/dims/digest.
func NewEmbedBuilder(path, model string, dims int, digest string) (*EmbedBuilder, error) {
	for _, p := range []string{path, path + "-wal", path + "-shm"} {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("twplus/index: clear %s: %w", p, err)
		}
	}
	db, err := driver.Open("file:"+path, vec1.Register)
	if err != nil {
		return nil, fmt.Errorf("twplus/index: open embeddings: %w", err)
	}
	db.SetMaxOpenConns(1)
	for _, p := range []string{"PRAGMA synchronous=OFF", "PRAGMA journal_mode=MEMORY"} {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, fmt.Errorf("twplus/index: %s: %w", p, err)
		}
	}
	stmts := []string{
		`CREATE TABLE meta (model TEXT NOT NULL, dims INTEGER NOT NULL, digest TEXT NOT NULL)`,
		`CREATE VIRTUAL TABLE vec USING vec1`,
		`INSERT INTO vec(cmd, vector) VALUES('rebuild', '{index:"flat"}')`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			db.Close()
			return nil, fmt.Errorf("twplus/index: embeddings ddl: %w", err)
		}
	}
	if _, err := db.Exec(`INSERT INTO meta(model, dims, digest) VALUES(?,?,?)`, model, dims, digest); err != nil {
		db.Close()
		return nil, fmt.Errorf("twplus/index: write meta: %w", err)
	}
	tx, err := db.Begin()
	if err != nil {
		db.Close()
		return nil, err
	}
	ins, err := tx.Prepare(`INSERT INTO vec(rowid, vector) VALUES(?, vec1_from_json(?))`)
	if err != nil {
		tx.Rollback()
		db.Close()
		return nil, err
	}
	return &EmbedBuilder{db: db, tx: tx, ins: ins, dims: dims}, nil
}

// Add stores one variant's vector. The vector must match the pinned dims.
func (b *EmbedBuilder) Add(variantID int64, v embed.Vector) error {
	if len(v) != b.dims {
		return fmt.Errorf("twplus/index: variant %d vector has %d dims, want %d", variantID, len(v), b.dims)
	}
	j, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if _, err := b.ins.Exec(variantID, string(j)); err != nil {
		return fmt.Errorf("twplus/index: insert vector %d: %w", variantID, err)
	}
	b.added++
	return nil
}

// Finalize commits and closes. Returns the number of vectors written.
func (b *EmbedBuilder) Finalize() (int, error) {
	if err := b.tx.Commit(); err != nil {
		b.Close()
		return 0, fmt.Errorf("twplus/index: commit embeddings: %w", err)
	}
	b.tx = nil
	n := b.added
	return n, b.Close()
}

// Close rolls back any open transaction and releases the database (idempotent).
func (b *EmbedBuilder) Close() error {
	if b.db == nil {
		return nil
	}
	if b.tx != nil {
		_ = b.tx.Rollback()
		b.tx = nil
	}
	db := b.db
	b.db = nil
	return db.Close()
}

// EmbeddingStore is a read-only handle to an embeddings file.
type EmbeddingStore struct {
	db     *sql.DB
	model  string
	dims   int
	digest string
}

// OpenEmbeddings opens an embeddings file read-only and reads its pin.
func OpenEmbeddings(path string) (*EmbeddingStore, error) {
	db, err := driver.Open("file:"+path+"?mode=ro&immutable=1", vec1.Register)
	if err != nil {
		return nil, fmt.Errorf("twplus/index: open embeddings %s: %w", path, err)
	}
	s := &EmbeddingStore{db: db}
	if err := db.QueryRow(`SELECT model, dims, digest FROM meta LIMIT 1`).Scan(&s.model, &s.dims, &s.digest); err != nil {
		db.Close()
		return nil, fmt.Errorf("twplus/index: read embeddings meta %s: %w", path, err)
	}
	return s, nil
}

// Model, Dims, and Digest report the pin the embeddings were built with, so a
// query embedded by a different model can be rejected before fusion.
func (s *EmbeddingStore) Model() string  { return s.model }
func (s *EmbeddingStore) Dims() int      { return s.dims }
func (s *EmbeddingStore) Digest() string { return s.digest }

// Close releases the database.
func (s *EmbeddingStore) Close() error { return s.db.Close() }

// Neighbor is one KNN hit: a corpus variant id and its distance to the query.
type Neighbor struct {
	VariantID int64
	Distance  float64
}

// KNN returns the k nearest variants to q, nearest first.
func (s *EmbeddingStore) KNN(q embed.Vector, k int) ([]Neighbor, error) {
	if k <= 0 {
		k = 20
	}
	if len(q) != s.dims {
		return nil, fmt.Errorf("twplus/index: query vector has %d dims, embeddings are %d", len(q), s.dims)
	}
	j, err := json.Marshal(q)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.Query(`SELECT rowid, distance FROM vec(vec1_from_json(?)) LIMIT ?`, string(j), k)
	if err != nil {
		return nil, fmt.Errorf("twplus/index: knn: %w", err)
	}
	defer rows.Close()
	var out []Neighbor
	for rows.Next() {
		var n Neighbor
		if err := rows.Scan(&n.VariantID, &n.Distance); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// SearchHybrid fuses BM25 and vector KNN by RRF. It degrades to pure BM25
// (SearchComponents) whenever there is no usable semantic signal — a nil store,
// an empty/zero query vector, or a dims mismatch — so the caller can always
// pass whatever it has and never lose the text path (ADR-0010: semantic is an
// enhancement, never a dependency).
func (ix *Index) SearchHybrid(query string, qvec embed.Vector, store *EmbeddingStore, f Filters, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 20
	}
	if store == nil || len(qvec) == 0 || len(qvec) != store.dims {
		return ix.SearchComponents(query, f, limit)
	}
	// Over-fetch each arm so fusion has depth and post-filtering doesn't starve.
	pool := limit * 4
	bm25, err := ix.rankedVariantIDs(query, f, pool)
	if err != nil {
		return nil, err
	}
	neighbors, err := store.KNN(qvec, pool)
	if err != nil {
		return nil, err
	}
	knn := make([]int64, len(neighbors))
	for i, n := range neighbors {
		knn[i] = n.VariantID
	}
	fused := search.IDs(search.FuseRRF([][]int64{bm25, knn}, search.DefaultRRFK))
	return ix.resultsByVariantIDs(fused, f, limit)
}

// rankedVariantIDs returns variant ids in BM25 order (or structural order for an
// empty query), honoring the structural filters.
func (ix *Index) rankedVariantIDs(query string, f Filters, limit int) ([]int64, error) {
	match := ftsQuery(query)
	if match == "" {
		var args []any
		q := `SELECT v.id FROM variant v JOIN component c ON c.id = v.component_id
			WHERE 1=1` + f.where("v", &args) + `
			ORDER BY c.full_name, v.framework, v.tailwind_version, v.mode LIMIT ?`
		args = append(args, limit)
		return ix.scanIDs(q, args)
	}
	args := []any{match}
	q := `SELECT v.id FROM fts_variant
		JOIN variant v   ON v.id = fts_variant.rowid
		JOIN component c ON c.id = v.component_id
		WHERE fts_variant MATCH ?` + f.where("v", &args) + `
		ORDER BY bm25(fts_variant, 10.0, 8.0, 3.0, 2.0) LIMIT ?`
	args = append(args, limit)
	return ix.scanIDs(q, args)
}

func (ix *Index) scanIDs(q string, args []any) ([]int64, error) {
	rows, err := ix.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("twplus/index: rank ids: %w", err)
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// resultsByVariantIDs loads SearchResult rows for the given variant ids,
// preserving the order of ids, applying the structural filters, and capping at
// limit. Ids that don't pass the filters are skipped (KNN ids weren't filtered).
func (ix *Index) resultsByVariantIDs(ids []int64, f Filters, limit int) ([]SearchResult, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, 0, len(ids)+4)
	for _, id := range ids {
		args = append(args, id)
	}
	q := `SELECT v.id, c.full_name, c.section, c.name, v.framework, v.tailwind_version, v.mode, v.embed_text
		FROM variant v JOIN component c ON c.id = v.component_id
		WHERE v.id IN (` + placeholders + `)` + f.where("v", &args)
	rows, err := ix.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("twplus/index: load fused: %w", err)
	}
	defer rows.Close()
	byID := make(map[int64]SearchResult, len(ids))
	for rows.Next() {
		var (
			id int64
			r  SearchResult
		)
		if err := rows.Scan(&id, &r.FullName, &r.Section, &r.Name, &r.Framework, &r.TailwindVersion, &r.Mode, &r.MatchText); err != nil {
			return nil, err
		}
		byID[id] = r
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]SearchResult, 0, limit)
	for _, id := range ids {
		if r, ok := byID[id]; ok {
			out = append(out, r)
			if len(out) >= limit {
				break
			}
		}
	}
	return out, nil
}

// Package index builds and queries the kit's TailwindPlus corpus as one
// SQLite file: structural columns for browsing, contentless FTS5 for BM25
// search, and zstd-compressed, content-addressed code blobs. It is cgo-free:
// the pinned ncruces/go-sqlite3 translates its SQLite Wasm to Go with wasm2go,
// so this package builds a single binary with no native libraries or wazero
// runtime dependency. Optional semantic search uses a separate vec1 embeddings
// file and hybrid rank fusion; the corpus build itself stays deterministic and
// embedder-free.
package index

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	"github.com/klauspost/compress/zstd"
	"github.com/ncruces/go-sqlite3"
	"github.com/ncruces/go-sqlite3/driver"
	"github.com/ncruces/go-sqlite3/ext/fts5"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/internal/twplus/normalize"
)

// ddl is the index schema, one statement per element.
var ddl = []string{
	`CREATE TABLE component (
		id                 INTEGER PRIMARY KEY,
		section            TEXT NOT NULL,
		"group"            TEXT NOT NULL,
		category           TEXT NOT NULL,
		name               TEXT NOT NULL,
		full_name          TEXT NOT NULL UNIQUE
	)`,
	`CREATE TABLE blob (
		content_hash TEXT PRIMARY KEY,
		code_zstd    BLOB NOT NULL,
		raw_len      INTEGER NOT NULL
	)`,
	`CREATE TABLE variant (
		id                 INTEGER PRIMARY KEY,
		component_id       INTEGER NOT NULL REFERENCES component(id),
		framework          TEXT NOT NULL,
		tailwind_version   INTEGER NOT NULL,
		mode               TEXT NOT NULL,
		language           TEXT NOT NULL,
		supports_dark_mode INTEGER NOT NULL,
		content_hash       TEXT NOT NULL REFERENCES blob(content_hash),
		embed_text         TEXT NOT NULL,
		UNIQUE (component_id, framework, tailwind_version, mode)
	)`,
	`CREATE INDEX variant_axes ON variant (framework, tailwind_version, mode)`,
	// Contentless FTS5 over the retrieval signals. rowid == variant.id.
	// tokenchars keeps "grid-cols-3", "dark:", "aria-checked:" whole.
	`CREATE VIRTUAL TABLE fts_variant USING fts5(
		full_name, taxonomy, embed_text, classes,
		content='', tokenize="unicode61 tokenchars '-_:'"
	)`,
}

// Builder accumulates components into a fresh index file inside one
// transaction. Not safe for concurrent use.
type Builder struct {
	db      *sql.DB
	tx      *sql.Tx
	enc     *zstd.Encoder
	seen    map[string]bool // content hashes already written to blob
	stmts   builderStmts
	nComp   int
	nVar    int
	nBlob   int
	rawLen  int64
	zstdLen int64
}

type builderStmts struct {
	comp *sql.Stmt
	blob *sql.Stmt
	vrnt *sql.Stmt
	fts  *sql.Stmt
}

// NewBuilder removes any existing file at path and creates the schema.
func NewBuilder(path string) (*Builder, error) {
	for _, p := range []string{path, path + "-wal", path + "-shm"} {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("twplus/index: clear %s: %w", p, err)
		}
	}
	db, err := driver.Open("file:"+path, func(c *sqlite3.Conn) error { return fts5.Register(c) })
	if err != nil {
		return nil, fmt.Errorf("twplus/index: open: %w", err)
	}
	// Build-time speed; the file is rebuilt from source, durability not needed.
	db.SetMaxOpenConns(1)
	for _, p := range []string{"PRAGMA synchronous=OFF", "PRAGMA journal_mode=MEMORY"} {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, fmt.Errorf("twplus/index: %s: %w", p, err)
		}
	}
	for _, s := range ddl {
		if _, err := db.Exec(s); err != nil {
			db.Close()
			return nil, fmt.Errorf("twplus/index: ddl: %w", err)
		}
	}
	enc, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedBetterCompression))
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("twplus/index: zstd writer: %w", err)
	}
	tx, err := db.Begin()
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("twplus/index: begin: %w", err)
	}
	b := &Builder{db: db, tx: tx, enc: enc, seen: map[string]bool{}}
	prep := func(q string) *sql.Stmt {
		if err != nil {
			return nil
		}
		var st *sql.Stmt
		st, err = tx.Prepare(q)
		return st
	}
	b.stmts.comp = prep(`INSERT INTO component(section,"group",category,name,full_name) VALUES(?,?,?,?,?)`)
	b.stmts.blob = prep(`INSERT INTO blob(content_hash,code_zstd,raw_len) VALUES(?,?,?) ON CONFLICT(content_hash) DO NOTHING`)
	b.stmts.vrnt = prep(`INSERT INTO variant(component_id,framework,tailwind_version,mode,language,supports_dark_mode,content_hash,embed_text) VALUES(?,?,?,?,?,?,?,?)`)
	b.stmts.fts = prep(`INSERT INTO fts_variant(rowid,full_name,taxonomy,embed_text,classes) VALUES(?,?,?,?,?)`)
	if err != nil {
		tx.Rollback()
		db.Close()
		return nil, fmt.Errorf("twplus/index: prepare: %w", err)
	}
	return b, nil
}

// AddComponent inserts one component and its variant rows.
func (b *Builder) AddComponent(c normalize.Component, rows []normalize.VariantRow) error {
	res, err := b.stmts.comp.Exec(c.Section, c.Group, c.Category, c.Name, c.FullName)
	if err != nil {
		return fmt.Errorf("twplus/index: insert component %q: %w", c.FullName, err)
	}
	cid, err := res.LastInsertId()
	if err != nil {
		return err
	}
	b.nComp++

	for _, r := range rows {
		if !b.seen[r.ContentHash] {
			packed := b.enc.EncodeAll([]byte(r.Code), nil)
			if _, err := b.stmts.blob.Exec(r.ContentHash, packed, len(r.Code)); err != nil {
				return fmt.Errorf("twplus/index: insert blob %s: %w", r.ContentHash, err)
			}
			b.seen[r.ContentHash] = true
			b.nBlob++
			b.rawLen += int64(len(r.Code))
			b.zstdLen += int64(len(packed))
		}
		vres, err := b.stmts.vrnt.Exec(cid, r.Framework, r.TailwindVersion, r.Mode, r.Language, boolToInt(r.SupportsDarkMode), r.ContentHash, r.EmbedText)
		if err != nil {
			return fmt.Errorf("twplus/index: insert variant %s [%s/%d/%s]: %w", r.FullName, r.Framework, r.TailwindVersion, r.Mode, err)
		}
		vid, err := vres.LastInsertId()
		if err != nil {
			return err
		}
		if _, err := b.stmts.fts.Exec(vid, r.FullName, taxonomySpaced(r), r.EmbedText, r.Classes); err != nil {
			return fmt.Errorf("twplus/index: insert fts %s: %w", r.FullName, err)
		}
		b.nVar++
	}
	return nil
}

// Stats is the tally returned after a build (also fed into the manifest).
type Stats struct {
	Components int   `json:"components"`
	Variants   int   `json:"variants"`
	Blobs      int   `json:"blobs"`
	RawBytes   int64 `json:"raw_code_bytes"`
	ZstdBytes  int64 `json:"zstd_code_bytes"`
}

// Finalize optimizes the FTS index, commits, vacuums, and closes the builder.
func (b *Builder) Finalize() (Stats, error) {
	if _, err := b.tx.Exec(`INSERT INTO fts_variant(fts_variant) VALUES('optimize')`); err != nil {
		b.Close()
		return Stats{}, fmt.Errorf("twplus/index: fts optimize: %w", err)
	}
	if err := b.tx.Commit(); err != nil {
		b.Close()
		return Stats{}, fmt.Errorf("twplus/index: commit: %w", err)
	}
	b.tx = nil // committed; nothing to roll back
	stats := Stats{Components: b.nComp, Variants: b.nVar, Blobs: b.nBlob, RawBytes: b.rawLen, ZstdBytes: b.zstdLen}
	if _, err := b.db.Exec(`VACUUM`); err != nil {
		b.Close()
		return stats, fmt.Errorf("twplus/index: vacuum: %w", err)
	}
	return stats, b.Close()
}

// Close rolls back any open transaction and releases the encoder and database.
// It is idempotent — safe to defer alongside Finalize (which leaves nothing to
// do) and on every error path.
func (b *Builder) Close() error {
	if b.db == nil {
		return nil
	}
	if b.tx != nil {
		_ = b.tx.Rollback() // ErrTxDone if already committed — ignore
		b.tx = nil
	}
	if b.enc != nil {
		b.enc.Close()
		b.enc = nil
	}
	db := b.db
	b.db = nil
	return db.Close()
}

func taxonomySpaced(r normalize.VariantRow) string {
	parts := make([]string, 0, 4)
	for _, p := range []string{r.Section, r.Group, r.Category, r.Name} {
		if p != "" {
			parts = append(parts, strings.ReplaceAll(p, ".", " "))
		}
	}
	return strings.Join(parts, " ")
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

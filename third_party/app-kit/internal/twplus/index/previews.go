package index

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"os"

	"github.com/klauspost/compress/zstd"
	"github.com/ncruces/go-sqlite3/driver"
)

// Previews is the optional sibling artifact to the corpus index: the full
// preview HTML for each variant, content-addressed and zstd-compressed, in its
// own SQLite file so the (embedded) corpus index stays lean. It needs no FTS5.

var previewDDL = []string{
	`CREATE TABLE preview_blob (
		content_hash TEXT PRIMARY KEY,
		html_zstd    BLOB NOT NULL,
		raw_len      INTEGER NOT NULL
	)`,
	`CREATE TABLE preview (
		full_name        TEXT NOT NULL,
		framework        TEXT NOT NULL,
		tailwind_version INTEGER NOT NULL,
		mode             TEXT NOT NULL,
		content_hash     TEXT NOT NULL REFERENCES preview_blob(content_hash),
		PRIMARY KEY (full_name, framework, tailwind_version, mode)
	)`,
}

// PreviewBuilder accumulates previews into a fresh file in one transaction.
type PreviewBuilder struct {
	db       *sql.DB
	tx       *sql.Tx
	enc      *zstd.Encoder
	seen     map[string]bool
	insBlob  *sql.Stmt
	insPrev  *sql.Stmt
	nPreview int
	nBlob    int
	rawLen   int64
	zstdLen  int64
}

// PreviewStats is returned after a previews build.
type PreviewStats struct {
	Previews  int   `json:"previews"`
	Blobs     int   `json:"blobs"`
	RawBytes  int64 `json:"raw_html_bytes"`
	ZstdBytes int64 `json:"zstd_html_bytes"`
}

// NewPreviewBuilder removes any existing file at path and creates the schema.
func NewPreviewBuilder(path string) (*PreviewBuilder, error) {
	for _, p := range []string{path, path + "-wal", path + "-shm"} {
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("twplus/index: clear %s: %w", p, err)
		}
	}
	db, err := driver.Open("file:" + path)
	if err != nil {
		return nil, fmt.Errorf("twplus/index: open previews: %w", err)
	}
	db.SetMaxOpenConns(1)
	for _, p := range []string{"PRAGMA synchronous=OFF", "PRAGMA journal_mode=MEMORY"} {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, fmt.Errorf("twplus/index: %s: %w", p, err)
		}
	}
	for _, s := range previewDDL {
		if _, err := db.Exec(s); err != nil {
			db.Close()
			return nil, fmt.Errorf("twplus/index: preview ddl: %w", err)
		}
	}
	enc, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedBetterCompression))
	if err != nil {
		db.Close()
		return nil, err
	}
	tx, err := db.Begin()
	if err != nil {
		db.Close()
		return nil, err
	}
	b := &PreviewBuilder{db: db, tx: tx, enc: enc, seen: map[string]bool{}}
	if b.insBlob, err = tx.Prepare(`INSERT INTO preview_blob(content_hash,html_zstd,raw_len) VALUES(?,?,?) ON CONFLICT(content_hash) DO NOTHING`); err != nil {
		b.Close()
		return nil, err
	}
	if b.insPrev, err = tx.Prepare(`INSERT INTO preview(full_name,framework,tailwind_version,mode,content_hash) VALUES(?,?,?,?,?) ON CONFLICT DO NOTHING`); err != nil {
		b.Close()
		return nil, err
	}
	return b, nil
}

// Add stores one variant's preview HTML.
func (b *PreviewBuilder) Add(fullName, framework string, tw int, mode, html string) error {
	if html == "" {
		return nil
	}
	sum := sha256.Sum256([]byte(html))
	hash := hex.EncodeToString(sum[:])
	if !b.seen[hash] {
		packed := b.enc.EncodeAll([]byte(html), nil)
		if _, err := b.insBlob.Exec(hash, packed, len(html)); err != nil {
			return fmt.Errorf("twplus/index: insert preview blob: %w", err)
		}
		b.seen[hash] = true
		b.nBlob++
		b.rawLen += int64(len(html))
		b.zstdLen += int64(len(packed))
	}
	if _, err := b.insPrev.Exec(fullName, framework, tw, mode, hash); err != nil {
		return fmt.Errorf("twplus/index: insert preview %s [%s/%d/%s]: %w", fullName, framework, tw, mode, err)
	}
	b.nPreview++
	return nil
}

// Finalize commits, vacuums, and closes.
func (b *PreviewBuilder) Finalize() (PreviewStats, error) {
	if err := b.tx.Commit(); err != nil {
		b.Close()
		return PreviewStats{}, fmt.Errorf("twplus/index: preview commit: %w", err)
	}
	b.tx = nil
	stats := PreviewStats{Previews: b.nPreview, Blobs: b.nBlob, RawBytes: b.rawLen, ZstdBytes: b.zstdLen}
	if _, err := b.db.Exec(`VACUUM`); err != nil {
		b.Close()
		return stats, fmt.Errorf("twplus/index: preview vacuum: %w", err)
	}
	return stats, b.Close()
}

// Close is idempotent: rolls back any open tx and releases resources.
func (b *PreviewBuilder) Close() error {
	if b.db == nil {
		return nil
	}
	if b.tx != nil {
		_ = b.tx.Rollback()
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

// PreviewStore is a read-only handle to a previews artifact.
type PreviewStore struct {
	db  *sql.DB
	dec *zstd.Decoder
}

// OpenPreviews opens an existing previews file read-only.
func OpenPreviews(path string) (*PreviewStore, error) {
	db, err := driver.Open("file:" + path + "?mode=ro&immutable=1")
	if err != nil {
		return nil, fmt.Errorf("twplus/index: open previews %s: %w", path, err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("twplus/index: open previews %s: %w", path, err)
	}
	dec, err := zstd.NewReader(nil)
	if err != nil {
		db.Close()
		return nil, err
	}
	return &PreviewStore{db: db, dec: dec}, nil
}

// Close releases the store.
func (p *PreviewStore) Close() error {
	p.dec.Close()
	return p.db.Close()
}

// PreviewResult is one variant's preview HTML plus the axes it resolved to.
type PreviewResult struct {
	FullName        string `json:"full_name"`
	Framework       string `json:"framework"`
	TailwindVersion int    `json:"tailwind_version"`
	Mode            string `json:"mode"`
	HTML            string `json:"html"`
}

// GetPreview returns the preview HTML for a variant. Framework, tw and mode are
// optional; when they leave more than one match the lexically-first is
// returned (same convention as GetComponent).
func (p *PreviewStore) GetPreview(fullName, framework string, tw int, mode string) (*PreviewResult, error) {
	args := []any{fullName}
	var where string
	if framework != "" {
		where += " AND pv.framework = ?"
		args = append(args, framework)
	}
	if tw != 0 {
		where += " AND pv.tailwind_version = ?"
		args = append(args, tw)
	}
	if mode != "" {
		where += " AND pv.mode = ?"
		args = append(args, mode)
	}
	q := `SELECT pv.framework, pv.tailwind_version, pv.mode, pb.html_zstd, pb.raw_len
		FROM preview pv JOIN preview_blob pb ON pb.content_hash = pv.content_hash
		WHERE pv.full_name = ?` + where + `
		ORDER BY pv.framework, pv.tailwind_version, pv.mode LIMIT 1`
	var (
		res    PreviewResult
		packed []byte
		rawLen int
	)
	res.FullName = fullName
	err := p.db.QueryRow(q, args...).Scan(&res.Framework, &res.TailwindVersion, &res.Mode, &packed, &rawLen)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, describe(fullName, framework, tw, mode))
	}
	if err != nil {
		return nil, fmt.Errorf("twplus/index: get preview %s: %w", fullName, err)
	}
	html, err := p.dec.DecodeAll(packed, make([]byte, 0, rawLen))
	if err != nil {
		return nil, fmt.Errorf("twplus/index: decompress preview %s: %w", fullName, err)
	}
	res.HTML = string(html)
	return &res, nil
}

// Spike: prove cgo-free SQLite (ncruces) + FTS5 BM25 + klauspost zstd work in
// this environment before the twplus pipeline is built on them. Throwaway.
package main

import (
	"fmt"
	"os"

	"github.com/klauspost/compress/zstd"
	"github.com/ncruces/go-sqlite3"
	"github.com/ncruces/go-sqlite3/driver"
	"github.com/ncruces/go-sqlite3/ext/fts5"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "spike FAIL:", err)
		os.Exit(1)
	}
	fmt.Println("spike OK: ncruces sqlite + FTS5 BM25 + zstd all work")
}

func run() error {
	db, err := driver.Open(":memory:", func(conn *sqlite3.Conn) error {
		return fts5.Register(conn)
	})
	if err != nil {
		return err
	}
	defer db.Close()

	if _, err := db.Exec(`CREATE VIRTUAL TABLE fts USING fts5(
		full_name, taxonomy, classes,
		tokenize="unicode61 tokenchars '-_:'");`); err != nil {
		return fmt.Errorf("create fts5: %w", err)
	}

	rows := []struct{ name, tax, cls string }{
		{"Application UI.Elements.Badges.Flat", "Application UI Elements Badges Flat", "inline-flex items-center rounded-md px-2 py-1 text-xs font-medium ring-1"},
		{"Application UI.Forms.Input groups.With label", "Application UI Forms Input groups", "block w-full rounded-md border-0 py-1.5 text-gray-900 ring-1 ring-inset"},
		{"Marketing.Sections.Pricing.Three tiers", "Marketing Sections Pricing", "grid grid-cols-1 gap-8 lg:grid-cols-3"},
	}
	for _, r := range rows {
		if _, err := db.Exec(`INSERT INTO fts(full_name, taxonomy, classes) VALUES(?,?,?)`, r.name, r.tax, r.cls); err != nil {
			return fmt.Errorf("insert: %w", err)
		}
	}

	// BM25 ranking: lower bm25() == better. Weight columns.
	q, err := db.Query(`SELECT full_name, bm25(fts, 10.0, 5.0, 1.0) AS score
		FROM fts WHERE fts MATCH ? ORDER BY score LIMIT 5`, `"badge" OR "rounded-md" OR "pricing"`)
	if err != nil {
		return fmt.Errorf("match query: %w", err)
	}
	defer q.Close()
	fmt.Println("--- BM25 results for 'badge OR rounded-md OR pricing' ---")
	n := 0
	for q.Next() {
		var name string
		var score float64
		if err := q.Scan(&name, &score); err != nil {
			return err
		}
		fmt.Printf("  %8.4f  %s\n", score, name)
		n++
	}
	if err := q.Err(); err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("FTS5 MATCH returned no rows (expected matches)")
	}

	// zstd roundtrip.
	enc, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedBetterCompression))
	if err != nil {
		return err
	}
	orig := []byte("<div class=\"inline-flex items-center rounded-md\">Badge</div>")
	packed := enc.EncodeAll(orig, nil)
	enc.Close()
	dec, err := zstd.NewReader(nil)
	if err != nil {
		return err
	}
	got, err := dec.DecodeAll(packed, nil)
	dec.Close()
	if err != nil {
		return fmt.Errorf("zstd decode: %w", err)
	}
	if string(got) != string(orig) {
		return fmt.Errorf("zstd roundtrip mismatch")
	}
	fmt.Printf("--- zstd: %d bytes -> %d bytes -> roundtrip ok ---\n", len(orig), len(packed))

	// sqlite version + compile options sanity.
	var ver string
	if err := db.QueryRow(`SELECT sqlite_version()`).Scan(&ver); err != nil {
		return err
	}
	fmt.Println("sqlite_version:", ver)
	return nil
}

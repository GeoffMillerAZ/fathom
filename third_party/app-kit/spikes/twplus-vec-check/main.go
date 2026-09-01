// Spike: prove cgo-free vector KNN via ncruces ext/vec1 (the Go-native path
// for D10 semantic search, replacing the asg017/sqlite-vec C alpha). Synthetic
// vectors, no Ollama. Throwaway.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ncruces/go-sqlite3/driver"
	"github.com/ncruces/go-sqlite3/ext/vec1"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "spike FAIL:", err)
		os.Exit(1)
	}
	fmt.Println("spike OK: ncruces ext/vec1 cgo-free vector KNN works")
}

func run() error {
	db, err := driver.Open(":memory:", vec1.Register)
	if err != nil {
		return err
	}
	defer db.Close()

	if _, err := db.Exec(`CREATE VIRTUAL TABLE vt USING vec1;
		INSERT INTO vt(cmd, vector) VALUES('rebuild', '{index:"flat"}');`); err != nil {
		return fmt.Errorf("create vec1: %w", err)
	}

	// Four 8-dim vectors; #3 is the target we'll query near.
	vecs := map[int][]float64{
		1: {1, 0, 0, 0, 0, 0, 0, 0},
		2: {0, 1, 0, 0, 0, 0, 0, 0},
		3: {0, 0, 0, 0, 0, 0, 1, 1},
		4: {0, 0, 1, 0, 0, 0, 0, 0},
	}
	for id, v := range vecs {
		j, _ := json.Marshal(v)
		if _, err := db.Exec(`INSERT INTO vt(rowid, vector) VALUES(?, vec1_from_json(?))`, id, string(j)); err != nil {
			return fmt.Errorf("insert vec %d: %w", id, err)
		}
	}

	probe, _ := json.Marshal([]float64{0, 0, 0, 0, 0, 0, 0.9, 0.9}) // closest to #3
	// vec1 KNN is a table-valued function: pass the query vector as an argument
	// to the virtual table; rows come back nearest-first with a `distance`.
	rows, err := db.Query(
		`SELECT rowid, distance FROM vt(vec1_from_json(?)) LIMIT 2`,
		string(probe),
	)
	if err != nil {
		return fmt.Errorf("knn query: %w", err)
	}
	defer rows.Close()
	fmt.Println("--- 2-NN of probe near vector #3 ---")
	var top int
	n := 0
	for rows.Next() {
		var id int
		var dist float64
		if err := rows.Scan(&id, &dist); err != nil {
			return err
		}
		if n == 0 {
			top = id
		}
		fmt.Printf("  rowid=%d distance=%.4f\n", id, dist)
		n++
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("KNN returned no rows")
	}
	if top != 3 {
		return fmt.Errorf("nearest rowid = %d, want 3", top)
	}
	return nil
}

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCmdReceiptVerifyDuplicateKeys: encoding/json keeps the last of a
// repeated (or case-folded) key; verify refuses the file before decoding.
func TestCmdReceiptVerifyDuplicateKeys(t *testing.T) {
	repo, r := round3CallersReceipt(t)
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	var obj map[string]any
	if err := json.Unmarshal(data, &obj); err != nil {
		t.Fatal(err)
	}
	anchor := obj["facts"].([]any)[0].(map[string]any)["anchors"].([]any)[0].(map[string]any)
	honestExcerpt, _ := json.Marshal(anchor["excerpt"])
	honestSymbol, _ := json.Marshal(obj["symbol"])
	cases := map[string][2]string{
		"excerpt-case-fold": {`"excerpt":` + string(honestExcerpt), `"excerpt":"return rm_rf()","Excerpt":` + string(honestExcerpt)},
		"symbol-case-fold":  {`"symbol":` + string(honestSymbol), `"symbol":"Lonely","SYMBOL":` + string(honestSymbol)},
		"exact-duplicate":   {`"symbol":` + string(honestSymbol), `"symbol":` + string(honestSymbol) + `,"symbol":` + string(honestSymbol)},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			raw := string(data)
			if !strings.Contains(raw, c[0]) {
				t.Fatalf("fixture lacks %s", c[0])
			}
			raw = strings.Replace(raw, c[0], c[1], 1)
			file := filepath.Join(t.TempDir(), "receipt.json")
			if err := os.WriteFile(file, []byte(raw), 0o644); err != nil {
				t.Fatal(err)
			}
			var stdout, stderr bytes.Buffer
			code := cmdReceipt([]string{"verify", "--repo", repo.root, file}, &stdout, &stderr)
			if code == 0 || strings.Contains(stdout.String(), "receipt verify: OK") || !strings.Contains(stderr.String(), "repeats") {
				t.Fatalf("duplicate key accepted: exit %d: %s%s", code, &stdout, &stderr)
			}
		})
	}
	verifyRound3Receipt(t, repo.root, r, 0, "receipt verify: OK")
}

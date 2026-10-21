package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCmdTestsIngest_UnityFlagsRequireExplicitNativePair(t *testing.T) {
	for _, args := range [][]string{{"--unity-nunit", "results.xml"}, {"--unity-receipt", "receipt.json"}, {"--unity-nunit", "results.xml", "--unity-receipt", "receipt.json", "--results", "envelope.json"}} {
		var out, err bytes.Buffer
		if code := cmdTestsIngest(args, &out, &err); code == 0 || !strings.Contains(err.String(), "requires --unity-nunit and --unity-receipt") {
			t.Fatalf("args=%v code=%d err=%s", args, code, err.String())
		}
	}
}

func TestCmdTestsIngest_UnityDoesNotAcceptHistoricalXMLWithoutReceipt(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "results.xml")
	if err := os.WriteFile(path, []byte(`<test-run result="Passed"/>`), 0600); err != nil {
		t.Fatal(err)
	}
	var out, err bytes.Buffer
	code := cmdTestsIngest([]string{"--service", "unity", "--store", filepath.Join(t.TempDir(), "scratch.sqlite"), "--manifest", root, "--unity-nunit", path, "--unity-receipt", filepath.Join(root, "absent.json")}, &out, &err)
	if code == 0 || !strings.Contains(err.String(), "Unity NUnit evidence refused") {
		t.Fatalf("code=%d err=%s", code, err.String())
	}
}

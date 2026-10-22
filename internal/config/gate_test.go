package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGateConfigDefaultsAndLayering(t *testing.T) {
	l, err := Load("", "")
	if err != nil {
		t.Fatal(err)
	}
	if l.Config.Gate.AcceptReceipts {
		t.Fatal("receipts default on")
	}
	user := filepath.Join(t.TempDir(), "user.toml")
	project := filepath.Join(t.TempDir(), "project.toml")
	if err := os.WriteFile(user, []byte("[gate]\naccept_receipts = true\nreceipts_dir = 'receipts'\nhost_label = 'helper'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(project, []byte("[gate]\naccept_receipts = false\n"), 0600); err != nil {
		t.Fatal(err)
	}
	l, err = Load(user, project)
	if err != nil {
		t.Fatal(err)
	}
	if l.Config.Gate.AcceptReceipts || l.Config.Gate.ReceiptsDir != "receipts" || l.Config.Gate.HostLabel != "helper" || l.Provenance["gate.accept_receipts"].Source != SourceProject {
		t.Fatalf("%+v", l)
	}
}

func TestGateConfigRejectsNonBooleanAcceptance(t *testing.T) {
	for _, key := range []string{"accept_receipts", "allow_receipt_acceptance"} {
		p := filepath.Join(t.TempDir(), "config.toml")
		if err := os.WriteFile(p, []byte("[gate]\n"+key+" = 'yes'\n"), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load("", p); err == nil {
			t.Fatalf("non boolean %s", key)
		}
	}
}

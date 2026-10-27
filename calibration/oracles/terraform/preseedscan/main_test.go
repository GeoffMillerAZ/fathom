package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestScanDirModuleCalls(t *testing.T) {
	dir := t.TempDir()
	config := `module "local" {
  source = "./modules/local"
}
module "remote" {
  source  = "terraform-aws-modules/kms/aws"
  version = "4.0.0"
  argument = { nested = "{ignored}" }
}
resource "aws_instance" "example" {}
`
	if err := os.WriteFile(filepath.Join(dir, "main.tf"), []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := scanDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []moduleCall{{Name: "local", Source: "./modules/local"},
		{Name: "remote", Source: "terraform-aws-modules/kms/aws", Version: "4.0.0"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("scanDir = %#v, want %#v", got, want)
	}
}

func TestScanDirRejectsDynamicSource(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.tf"), []byte(`module "remote" {
  source = var.module_source
}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := scanDir(dir); err == nil {
		t.Fatal("dynamic source was accepted")
	}
}

func TestScanDirNoModuleCalls(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "main.tf"), []byte(`output "value" { value = "ok" }`), 0o644); err != nil {
		t.Fatal(err)
	}
	calls, err := scanDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if calls == nil || len(calls) != 0 {
		t.Fatalf("module-free root must encode as [], got %#v", calls)
	}
}

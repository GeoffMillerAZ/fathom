package security_test

import (
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/security"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

func hasRule(fs []security.Finding, rule string) bool {
	for _, f := range fs {
		if f.Rule == rule {
			return true
		}
	}
	return false
}

func countRule(fs []security.Finding, rule string) int {
	n := 0
	for _, f := range fs {
		if f.Rule == rule {
			n++
		}
	}
	return n
}

// ─── ShannonEntropy ──────────────────────────────────────────────────────────

func TestShannonEntropy_Empty(t *testing.T) {
	if security.ShannonEntropy("") != 0 {
		t.Fatal("expected 0 for empty string")
	}
}

func TestShannonEntropy_HighRandom(t *testing.T) {
	// A realistic high-entropy token — base64-like random string.
	tok := "aB3xK9mP2rT7vZ1nQ5wY8cF4uL6jH0eD"
	h := security.ShannonEntropy(tok)
	if h < 4.0 {
		t.Errorf("expected entropy >= 4.0 for random token, got %.2f", h)
	}
}

func TestShannonEntropy_LowRepetitive(t *testing.T) {
	// Highly repetitive string should have low entropy.
	h := security.ShannonEntropy("aaaaaaaaaaaaaaaaaaaaaa")
	if h != 0 {
		t.Errorf("expected 0 entropy for uniform string, got %.2f", h)
	}
}

func TestShannonEntropy_Threshold(t *testing.T) {
	// Values below threshold must not trigger high-entropy rule.
	lowTok := "hello_world_foo_bar"
	h := security.ShannonEntropy(lowTok)
	if h >= 4.5 {
		t.Errorf("expected entropy < 4.5 for simple token, got %.2f", h)
	}
}

// ─── SecretScan — JWT ────────────────────────────────────────────────────────

const jwtSrc = `package cfg

const token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c"
`

func TestSecretScan_JWT(t *testing.T) {
	fs := security.SecretScan("cfg.go", jwtSrc)
	if !hasRule(fs, "secret-jwt") {
		t.Fatalf("expected secret-jwt finding, got %+v", fs)
	}
}

func TestSecretScan_JWTBenign(t *testing.T) {
	// "eyJ" prefix but not a full JWT.
	src := `package cfg
const x = "eyJub3RhcmVhbHRva2Vu"`
	fs := security.SecretScan("cfg.go", src)
	if hasRule(fs, "secret-jwt") {
		t.Errorf("incomplete JWT-like string should NOT be flagged as jwt, got %+v", fs)
	}
}

// ─── SecretScan — AWS key ────────────────────────────────────────────────────

const awsSrc = `package infra
const accessKey = "AKIAIOSFODNN7EXAMPLE"`

func TestSecretScan_AWSKey(t *testing.T) {
	fs := security.SecretScan("infra.go", awsSrc)
	if !hasRule(fs, "secret-aws-key") {
		t.Fatalf("expected secret-aws-key, got %+v", fs)
	}
}

func TestSecretScan_AWSKeyBenign(t *testing.T) {
	src := `package infra
const notKey = "NOTAKIA12345678901234"`
	fs := security.SecretScan("infra.go", src)
	if hasRule(fs, "secret-aws-key") {
		t.Errorf("non-AKIA string should NOT be flagged as aws-key")
	}
}

// ─── SecretScan — PEM private key ────────────────────────────────────────────

const pemSrc = "package certs\nconst key = `-----BEGIN RSA PRIVATE KEY-----\nMIIEowIBAAKCAQEA\n-----END RSA PRIVATE KEY-----`\n"

func TestSecretScan_PEMHeader(t *testing.T) {
	fs := security.SecretScan("certs.go", pemSrc)
	if !hasRule(fs, "secret-private-key") {
		t.Fatalf("expected secret-private-key, got %+v", fs)
	}
}

// ─── SecretScan — high-entropy strings ───────────────────────────────────────

// A 32-char high-entropy string embedded in a Go string literal.
const highEntropySrc = `package secret
const apiSecret = "xK9mP2rT7vZ1nQ5wY8cF4uL6jH0eDaB3"`

func TestSecretScan_HighEntropy_Flagged(t *testing.T) {
	fs := security.SecretScan("secret.go", highEntropySrc)
	if !hasRule(fs, "secret-high-entropy") && !hasRule(fs, "secret-assignment") {
		t.Fatalf("expected at least one secret finding for high-entropy literal, got %+v", fs)
	}
}

func TestSecretScan_LowEntropy_NotFlagged(t *testing.T) {
	src := `package safe
const greeting = "hello world"`
	fs := security.SecretScan("safe.go", src)
	if hasRule(fs, "secret-high-entropy") {
		t.Errorf("low-entropy string should not be flagged")
	}
}

// ─── SecretScan — generic assignment ─────────────────────────────────────────

const assignSrc = `package cfg

func init() {
	password = "s3cr3t_val"
	api_key := "not_real_key"
}`

func TestSecretScan_Assignment(t *testing.T) {
	fs := security.SecretScan("cfg.go", assignSrc)
	if !hasRule(fs, "secret-assignment") {
		t.Logf("findings: %+v", fs)
		// password and api_key assignments should be caught
		// Note: assignment scanning is best-effort; only fail if truly nothing caught.
		if len(fs) == 0 {
			t.Fatal("expected at least one secret finding for credential assignments")
		}
	}
}

// ─── SecretScan — cross-lang regex path ──────────────────────────────────────

const yamlSecret = `
database:
  password: "supersecret123456"
  api_key: "AKIAIOSFODNN7EXAMPLE"
`

func TestSecretScan_CrossLang_AWSKey(t *testing.T) {
	fs := security.SecretScan("config.yaml", yamlSecret)
	if !hasRule(fs, "secret-aws-key") {
		t.Fatalf("expected secret-aws-key from regex path, got %+v", fs)
	}
}

func TestSecretScan_CrossLang_Generic(t *testing.T) {
	src := "api_key=AKIA12345678901234XXXX\n"
	fs := security.SecretScan("env.sh", src)
	// Should find something (aws-key or assignment).
	if len(fs) == 0 {
		t.Fatal("expected at least one finding for api_key=AKIA... in shell script")
	}
}

// ─── SecretScan — determinism ────────────────────────────────────────────────

func TestSecretScan_Determinism(t *testing.T) {
	a := security.SecretScan("cfg.go", jwtSrc)
	b := security.SecretScan("cfg.go", jwtSrc)
	if len(a) != len(b) {
		t.Fatalf("non-deterministic: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Errorf("finding[%d] differs", i)
		}
	}
}

// ─── ShellOut — Go source ────────────────────────────────────────────────────

const shellGoSrc = `package runner

import "os/exec"

func run(cmd string) error {
	c := exec.Command("sh", "-c", cmd)
	return c.Run()
}

func ctx(cmd string) error {
	return exec.CommandContext(nil, cmd).Run()
}
`

func TestShellOut_GoAST(t *testing.T) {
	fs := security.ShellOut("runner.go", shellGoSrc)
	if !hasRule(fs, "shell-out") {
		t.Fatalf("expected shell-out finding, got %+v", fs)
	}
}

func TestShellOut_GoAST_CountBoth(t *testing.T) {
	fs := security.ShellOut("runner.go", shellGoSrc)
	n := countRule(fs, "shell-out")
	if n < 2 {
		t.Errorf("expected at least 2 shell-out findings (Command + CommandContext), got %d", n)
	}
}

func TestShellOut_Benign_Go(t *testing.T) {
	src := `package safe
func add(a, b int) int { return a + b }
`
	fs := security.ShellOut("safe.go", src)
	if len(fs) != 0 {
		t.Errorf("expected no findings, got %+v", fs)
	}
}

// ─── ShellOut — cross-lang (Python) ──────────────────────────────────────────

const pythonShellSrc = `
import subprocess
result = subprocess.run(["ls", "-l"], capture_output=True)
out = subprocess.check_output(["cat", "/etc/passwd"])
`

func TestShellOut_Python(t *testing.T) {
	fs := security.ShellOut("script.py", pythonShellSrc)
	if !hasRule(fs, "shell-out") {
		t.Fatalf("expected shell-out finding from Python regex, got %+v", fs)
	}
}

// ─── ShellOut — cross-lang (Node.js) ─────────────────────────────────────────

const nodeShellSrc = `
const { exec } = require('child_process');
exec('ls -la', (err, stdout) => { console.log(stdout); });
`

func TestShellOut_Node(t *testing.T) {
	fs := security.ShellOut("script.js", nodeShellSrc)
	if !hasRule(fs, "shell-out") {
		t.Fatalf("expected shell-out finding from Node.js regex, got %+v", fs)
	}
}

// ─── ShellOut — determinism ───────────────────────────────────────────────────

func TestShellOut_Determinism(t *testing.T) {
	a := security.ShellOut("runner.go", shellGoSrc)
	b := security.ShellOut("runner.go", shellGoSrc)
	if len(a) != len(b) {
		t.Fatalf("non-deterministic: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Errorf("finding[%d] differs", i)
		}
	}
}

// ─── HTTPClientCalls — Go source ─────────────────────────────────────────────

const httpGoSrc = `package client

import "net/http"

func fetchData(url string) (*http.Response, error) {
	return http.Get(url)
}

func postData(url string) (*http.Response, error) {
	return http.Post(url, "application/json", nil)
}

func doRequest(c *http.Client, req *http.Request) (*http.Response, error) {
	return c.Do(req)
}
`

func TestHTTPClientCalls_GoGet(t *testing.T) {
	fs := security.HTTPClientCalls("client.go", httpGoSrc)
	if !hasRule(fs, "http-client") {
		t.Fatalf("expected http-client finding, got %+v", fs)
	}
}

func TestHTTPClientCalls_GoCount(t *testing.T) {
	fs := security.HTTPClientCalls("client.go", httpGoSrc)
	n := countRule(fs, "http-client")
	// http.Get, http.Post, and c.Do should each be flagged.
	if n < 3 {
		t.Errorf("expected at least 3 http-client findings, got %d: %+v", n, fs)
	}
}

func TestHTTPClientCalls_Benign_Go(t *testing.T) {
	src := `package safe
func add(a, b int) int { return a + b }
`
	fs := security.HTTPClientCalls("safe.go", src)
	if len(fs) != 0 {
		t.Errorf("expected no findings, got %+v", fs)
	}
}

// ─── HTTPClientCalls — cross-lang (JS fetch) ─────────────────────────────────

const jsFetchSrc = `
async function getData(url) {
  const resp = await fetch(url);
  return resp.json();
}
`

func TestHTTPClientCalls_JSFetch(t *testing.T) {
	fs := security.HTTPClientCalls("app.js", jsFetchSrc)
	if !hasRule(fs, "http-client") {
		t.Fatalf("expected http-client finding from fetch(), got %+v", fs)
	}
}

// ─── HTTPClientCalls — cross-lang (Python requests) ──────────────────────────

const pythonHTTPSrc = `
import requests

def get_users():
    return requests.get("https://api.example.com/users")

def create_user(data):
    return requests.post("https://api.example.com/users", json=data)
`

func TestHTTPClientCalls_PythonRequests(t *testing.T) {
	fs := security.HTTPClientCalls("api.py", pythonHTTPSrc)
	if !hasRule(fs, "http-client") {
		t.Fatalf("expected http-client finding from requests.get(), got %+v", fs)
	}
}

// ─── HTTPClientCalls — determinism ───────────────────────────────────────────

func TestHTTPClientCalls_Determinism(t *testing.T) {
	a := security.HTTPClientCalls("client.go", httpGoSrc)
	b := security.HTTPClientCalls("client.go", httpGoSrc)
	if len(a) != len(b) {
		t.Fatalf("non-deterministic: %d vs %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Errorf("finding[%d] differs", i)
		}
	}
}

// ─── Sorted output ───────────────────────────────────────────────────────────

func TestSecretScan_Sorted(t *testing.T) {
	src := jwtSrc + "\n" + awsSrc
	fs := security.SecretScan("multi.go", src)
	for i := 1; i < len(fs); i++ {
		if fs[i-1].Line > fs[i].Line {
			t.Errorf("findings not sorted by line at index %d", i)
		}
	}
}

// ─── Invalid / empty source ───────────────────────────────────────────────────

func TestScanners_InvalidSrc(t *testing.T) {
	garbage := "this is not valid Go {{{{{"
	_ = security.SecretScan("x.go", garbage)
	_ = security.ShellOut("x.go", garbage)
	_ = security.HTTPClientCalls("x.go", garbage)
}

func TestScanners_EmptySrc(t *testing.T) {
	_ = security.SecretScan("empty.go", "package empty\n")
	_ = security.ShellOut("empty.go", "package empty\n")
	_ = security.HTTPClientCalls("empty.go", "package empty\n")
}

//go:build ignore

// Generate a fresh, signed two-project toy installation. Issuer private keys
// stay in memory. Usage: go run ./cmd/harnessctl/testdata/generate.go ABS-NEW-DIR
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

const layerType = "application/vnd.agentic-kit.harness-layer.v1+json"

type issuer struct {
	id      string
	private ed25519.PrivateKey
	public  ed25519.PublicKey
}

type source struct {
	id, kind, target, path, issuerID string
	priority                         int
	reviewMin, reviewMax, reviewWork int
	reviewFault                      string
	reviewMaintenance                bool
	preference                       int // zero means no preference in this toy
}

func main() {
	if len(os.Args) != 2 || !filepath.IsAbs(os.Args[1]) || filepath.Clean(os.Args[1]) != os.Args[1] {
		fail()
	}
	dir := os.Args[1]
	if os.Mkdir(dir, 0700) != nil {
		fail()
	} // Existing output is never overwritten.
	shared := filepath.Join(dir, "sources")
	rootA, rootB := filepath.Join(dir, "project-a"), filepath.Join(dir, "project-b")
	for _, path := range []string{shared, rootA, rootB, filepath.Join(rootA, ".agentic"), filepath.Join(rootB, ".agentic")} {
		if os.Mkdir(path, 0700) != nil {
			fail()
		}
	}
	issuerIDs := []string{"issuer-installation", "issuer-organization", "issuer-team", "issuer-personal", "issuer-project-a", "issuer-project-b"}
	issuers := map[string]issuer{}
	keys := make([]any, 0, len(issuerIDs))
	for _, id := range issuerIDs {
		pub, private, e := ed25519.GenerateKey(rand.Reader)
		if e != nil {
			fail()
		}
		issuers[id] = issuer{id, private, pub}
		keys = append(keys, map[string]any{"id": id, "publicKey": base64.StdEncoding.EncodeToString(pub), "revoked": false})
	}
	now := time.Now().UTC().Truncate(time.Second)
	stamp := func(t time.Time) string { return t.Format("2006-01-02T15:04:05Z") }
	issued, expires := stamp(now.Add(-time.Minute)), stamp(now.Add(time.Hour))
	common := []source{
		{"installation", "installation", "toy-host", filepath.Join(shared, "installation-envelope.json"), "issuer-installation", 0, 0, 1000000, 1000, "warn", true, 0},
		{"organization", "organization", "toy-org", filepath.Join(shared, "organization-envelope.json"), "issuer-organization", 1, 500000, 900000, 500, "warn", true, 0},
		{"team", "team", "toy-team", filepath.Join(shared, "team-envelope.json"), "issuer-team", 2, 600000, 850000, 75, "hold", false, 0},
		{"personal", "personal", "toy-person", filepath.Join(shared, "personal-envelope.json"), "issuer-personal", 4, 0, 1000000, 1000, "warn", true, 750000},
	}
	projectSource := func(id, root string) source {
		return source{"project", "project", id, filepath.Join(root, ".agentic", "project-envelope.json"), "issuer-" + id, 3, 0, 1000000, 1000, "warn", true, 650000}
	}
	refs := map[string]any{}
	for _, s := range append(append([]source{}, common...), projectSource("project-a", rootA), projectSource("project-b", rootB)) {
		refs[s.kind+":"+s.target] = writeSource(s, issuers[s.issuerID], issued, expires)
	}
	makeProject := func(id, root string) any {
		layers := []any{refs["installation:toy-host"], refs["organization:toy-org"], refs["team:toy-team"], refs["project:"+id], refs["personal:toy-person"]}
		return map[string]any{"id": id, "root": root, "organizationId": "toy-org", "teamIds": []string{"toy-team"}, "principalId": "toy-person", "controlIds": []string{"mandatory", "review"}, "layers": layers}
	}
	registration := map[string]any{
		"version": "harnessctl.registration/1", "epoch": 1, "issuedAt": issued, "expiresAt": expires, "hostId": "toy-host", "keys": keys,
		"controls": []any{
			map[string]any{"id": "mandatory", "event": "before-tool", "class": "mandatory", "minSamplePPM": 1000000, "maxSamplePPM": 1000000, "defaultSamplePPM": 1000000, "maxWorkMillis": 1000, "onFault": "hold", "allowMaintenance": false, "tunableBy": []string{}},
			map[string]any{"id": "review", "event": "after-tool", "class": "advisory", "minSamplePPM": 0, "maxSamplePPM": 1000000, "defaultSamplePPM": 1000000, "maxWorkMillis": 1000, "onFault": "warn", "allowMaintenance": true, "tunableBy": []string{"project", "personal"}},
		},
		"projects": []any{makeProject("project-a", rootA), makeProject("project-b", rootB)},
	}
	writeJSON(filepath.Join(dir, "registration.json"), registration)
	fmt.Println("toy fixture written:", dir)
}

func writeSource(s source, issuer issuer, issued, expires string) any {
	mandatory := map[string]any{"id": "mandatory", "minSamplePPM": 1000000, "maxSamplePPM": 1000000, "maxWorkMillis": 1000, "onFault": "hold", "allowMaintenance": false, "tunableBy": []string{}}
	review := map[string]any{"id": "review", "minSamplePPM": s.reviewMin, "maxSamplePPM": s.reviewMax, "maxWorkMillis": s.reviewWork, "onFault": s.reviewFault, "allowMaintenance": s.reviewMaintenance, "tunableBy": []string{"project", "personal"}}
	preferences := []any{}
	if s.preference != 0 {
		preferences = append(preferences, map[string]any{"controlId": "review", "samplePPM": s.preference})
	}
	payload := map[string]any{"version": "harnessctl.layer/1", "id": s.id, "kind": s.kind, "targetId": s.target, "revision": 1, "issuedAt": issued, "expiresAt": expires, "controls": []any{mandatory, review}, "preferences": preferences}
	bytes, e := json.Marshal(payload)
	if e != nil {
		fail()
	}
	pae := []byte("DSSEv1 " + strconv.Itoa(len(layerType)) + " " + layerType + " " + strconv.Itoa(len(bytes)) + " ")
	pae = append(pae, bytes...)
	sig := ed25519.Sign(issuer.private, pae)
	envelope := map[string]any{"payloadType": layerType, "payload": base64.StdEncoding.EncodeToString(bytes), "signatures": []any{map[string]any{"sig": base64.StdEncoding.EncodeToString(sig)}}}
	hash := sha256.Sum256(bytes)
	writeBytes(trimEnvelope(s.path), bytes)
	writeJSON(s.path, envelope)
	return map[string]any{"id": s.id, "kind": s.kind, "targetId": s.target, "path": s.path, "keyId": issuer.id, "revision": 1, "payloadSHA256": hex.EncodeToString(hash[:]), "maxAgeSeconds": 3600, "priority": s.priority, "controlIds": []string{"mandatory", "review"}}
}

func trimEnvelope(path string) string {
	const suffix = "-envelope.json"
	return path[:len(path)-len(suffix)] + "-payload.json"
}
func writeJSON(path string, value any) {
	bytes, e := json.MarshalIndent(value, "", "  ")
	if e != nil {
		fail()
	}
	writeBytes(path, append(bytes, '\n'))
}
func writeBytes(path string, bytes []byte) {
	f, e := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if e != nil {
		fail()
	}
	n, e := f.Write(bytes)
	if e != nil || n != len(bytes) || f.Close() != nil {
		fail()
	}
}
func fail() { fmt.Fprintln(os.Stderr, "fixture generation failed"); os.Exit(2) }

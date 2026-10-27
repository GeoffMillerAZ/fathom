package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

const authored = `_format_version: "3.0"
services:
- name: app
  host: pool
  routes:
  - name: nested
    paths: ["/nested"]
  plugins:
  - name: cors
- name: unused
  host: unused.invalid
routes:
- name: flat
  paths: ["/flat"]
  service:
    name: app
upstreams:
- name: pool
  targets:
  - target: 127.0.0.1:19001
plugins:
- name: key-auth
  route: flat
`

func complete(t *testing.T, source string) result {
	t.Helper()
	got := observe([]byte(source))
	if got.Status != "native_authored_probe_complete_ungraded" || got.HTTPRequests != 0 {
		t.Fatalf("native whole input refused: %+v", got)
	}
	return got
}
func TestWholeNativeTypedPopulationUnusedAndExplicitVersusNested(t *testing.T) {
	got := complete(t, authored)
	if len(got.Nodes) != 8 || got.ServiceCallbacks != 2 || len(got.References) != 4 {
		t.Fatalf("native population: %+v", got)
	}
	resolved := map[string]string{}
	for _, ref := range got.References {
		if ref.Outcome == "owner_registry_resolved" {
			resolved[ref.FromKind+":"+ref.From+"/"+ref.Field] = ref.To
		}
	}
	if resolved["service:app/host"] != "pool" || resolved["route:flat/service"] != "app" || resolved["plugin:key-auth/route"] != "flat" || len(resolved) != 3 {
		t.Fatalf("native resolved fields: %+v", got.References)
	}
	for _, ref := range got.References {
		if ref.From == "nested" || ref.From == "cors" {
			t.Fatalf("implicit containment became written reference: %+v", ref)
		}
	}
}
func TestURLDerivedHostDoesNotBecomeWrittenHost(t *testing.T) {
	source := `_format_version: "3.0"
services:
- name: app
  url: http://pool:19001
upstreams:
- name: pool
`
	got := complete(t, source)
	if len(got.References) != 0 || got.Nodes[0].WrittenHost != nil || got.Nodes[0].WrittenURL == nil {
		t.Fatalf("URL provenance lost: %+v", got)
	}
	got = complete(t, strings.Replace(source, "  url:", "  host: pool\n  url:", 1))
	if len(got.References) != 1 || got.References[0].To != "pool" {
		t.Fatalf("explicit host missing: %+v", got)
	}
}
func TestJSONAndYAMLPreserveEquivalentNativeFields(t *testing.T) {
	source := `{"_format_version":"3.0","services":[{"name":"app","url":"http://pool:19001"}],"upstreams":[{"name":"pool"}]}`
	got := complete(t, source)
	if got.ServiceCallbacks != 1 || got.Nodes[0].WrittenHost != nil || got.Nodes[0].WrittenURL == nil || len(got.References) != 0 {
		t.Fatalf("JSON typed hook: %+v", got)
	}
}
func TestNativeInvalidTargetAndDuplicateNamesRefuseWholeInput(t *testing.T) {
	for _, source := range []string{
		strings.Replace(authored, "    name: app", "    name: absent", 1),
		strings.Replace(authored, "- name: unused", "- name: app", 1),
	} {
		got := observe([]byte(source))
		if got.Status != "refused" {
			t.Fatalf("invalid native config admitted: %+v", got)
		}
	}
}
func TestNativeInputSensitivityChangesEvaluatedTarget(t *testing.T) {
	a := complete(t, authored)
	b := complete(t, strings.ReplaceAll(authored, "pool", "second"))
	if a.References[0].To != "pool" || b.References[0].To != "second" {
		t.Fatalf("changed native fact did not move target: %+v / %+v", a.References, b.References)
	}
}

func TestNativeSourceCapPlusOneRefusesWholeInput(t *testing.T) {
	got := observe(bytes.Repeat([]byte{'x'}, maxSource+1))
	if got.Status != "refused" || !strings.Contains(got.Error, "source cap") || len(got.Nodes) != 0 {
		t.Fatalf("cap did not refuse before population: %+v", got)
	}
}
func TestNativePopulationCapPlusOneRefusesWholeInput(t *testing.T) {
	source := "_format_version: \"3.0\"\nupstreams:\n"
	for i := 0; i < maxEntities+1; i++ {
		source += "- name: unused" + strings.Repeat("x", i%3) + "\n"
	}
	got := observe([]byte(source))
	if got.Status != "refused" || !strings.Contains(got.Error, "population cap") {
		t.Fatalf("cap failed: %+v", got)
	}
}
func TestMissingNativePluginConfigRefusesWithoutAccess(t *testing.T) {
	got := observe([]byte("_format_version: \"3.0\"\nplugins:\n- name: cors\n  _config: unavailable.json\n"))
	if got.Status != "refused" || !strings.Contains(got.Error, "_plugin_config") || got.HTTPRequests != 0 {
		t.Fatalf("unsafe source admitted: %+v", got)
	}
}
func TestReceiptsNeverExposeGeneratedUUIDs(t *testing.T) {
	a := complete(t, authored)
	b := complete(t, authored)
	left, _ := json.Marshal(a)
	right, _ := json.Marshal(b)
	if !bytes.Equal(left, right) {
		t.Fatalf("native summaries vary with generated IDs")
	}
}

func TestNativePluginScopesUnderConsumerAndGroupAreEnumerated(t *testing.T) {
	consumer := complete(t, "_format_version: \"3.0\"\nconsumers:\n- username: authored\n  plugins:\n  - name: cors\n")
	if len(consumer.Nodes) != 1 || consumer.Nodes[0].Scope != "consumer:authored" {
		t.Fatalf("native consumer plugin omitted: %+v", consumer)
	}
	group := observe([]byte("_format_version: \"3.0\"\nconsumer_groups:\n- name: authored\n  plugins:\n  - name: rate-limiting\n    config:\n      minute: 1\n"))
	if len(group.Nodes) != 1 || group.Nodes[0].Scope != "consumer_group:authored" {
		t.Fatalf("native group plugin omitted: %+v", group)
	}
}

func TestNativeOutputCapPlusOneRefusesWholeOutput(t *testing.T) {
	base := result{Status: "native_authored_probe_complete_ungraded", Error: "x"}
	raw, err := encodeResult(base)
	if err != nil {
		t.Fatal(err)
	}
	base.Error = strings.Repeat("x", maxOutput-len(raw)+1)
	if raw, err = encodeResult(base); err != nil || len(raw) != maxOutput {
		t.Fatalf("exact output cap: bytes=%d error=%v", len(raw), err)
	}
	base.Error += "x"
	if raw, err = encodeResult(base); err == nil || raw != nil {
		t.Fatalf("cap+1 should refuse whole output")
	}
}

func TestWrittenHostUsesStrictNativeNameNotUpstreamIDAlias(t *testing.T) {
	source := `_format_version: "3.0"
services:
- name: app
  host: 14c17325-b575-4b97-9bfa-bd3ab230bfc9
upstreams:
- name: pool
  id: 14c17325-b575-4b97-9bfa-bd3ab230bfc9
`
	got := complete(t, source)
	if len(got.References) != 1 || got.References[0].Outcome != "literal_host_without_named_upstream" {
		t.Fatalf("registry ID alias became literal host/name binding: %+v", got.References)
	}
}

func TestKonnectRoutingMetadataDoesNotDropGatewayFields(t *testing.T) {
	plain := "_format_version: \"3.0\"\nservices:\n- name: app\n  host: pool\nupstreams:\n- name: pool\n"
	metadata := strings.Replace(plain, "services:", "_konnect:\n  runtime_group_name: authored\nservices:", 1)
	a := complete(t, plain)
	b := complete(t, metadata)
	left, _ := json.Marshal(a)
	right, _ := json.Marshal(b)
	if !bytes.Equal(left, right) {
		t.Fatalf("routing metadata changed Gateway fields")
	}
}

func TestNativeInlinePluginConfigsRemainOfflineAndKeepPopulation(t *testing.T) {
	source := "_format_version: \"3.0\"\n_plugin_configs:\n  shared:\n    origins: [\"*\"]\nplugins:\n- name: cors\n  _config: shared\nconsumer_groups:\n- name: group\n  plugins:\n  - name: cors\n    _config: shared\n"
	got := complete(t, source)
	if len(got.Nodes) != 2 || got.HTTPRequests != 0 {
		t.Fatalf("inline native population lost: %+v", got)
	}
}

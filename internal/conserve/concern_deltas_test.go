package conserve

import (
	"context"
	"reflect"
	"testing"
)

func TestConcernDeltasNeverRegistersAPlane(t *testing.T) {
	var names []string
	for _, p := range Planes() {
		names = append(names, p.Name())
	}
	if !reflect.DeepEqual(names, []string{"absence", "certificates", "governance", "integrity", "resurrection", "surface"}) {
		t.Fatalf("Planes changed: %v", names)
	}
	// No index is an operational error, never an empty successful delta file.
	if _, err := ConcernDeltas(context.Background(), Input{}); err == nil {
		t.Fatal("missing store accepted")
	}
	for i, p := range Planes() {
		if p.Name() != names[i] {
			t.Fatal("ConcernDeltas changed registry")
		}
	}
}

func TestConcernDeltaYAMLDecoding(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
		want bool
	}{
		{"manifest", "kind: Deployment\nmetadata: {name: api}\nspec: {replicas: 1}\n", true},
		{"helm-map-key", "kind: Deployment\nmetadata:\n  name: {{ .Values.name }}\n", false},
		{"helm-syntax", "{{- if .Values.enabled }}\nkind: Deployment\n{{- end }}\n", false},
		{"duplicate-keys", "kind: Deployment\nmetadata: {name: api}\nspec: {replicas: 3, replicas: 1}\n", true},
		{"non-manifest", "answer: 42\n", true},
		{"empty", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := concernYAMLDecodes([]byte(tc.src)); got != tc.want {
				t.Fatalf("decodes = %v, want %v", got, tc.want)
			}
		})
	}
}

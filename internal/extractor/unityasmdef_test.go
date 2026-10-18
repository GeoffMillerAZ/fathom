package extractor

import (
	"reflect"
	"testing"
)

func TestUnityAsmdefDeclaredFacts(t *testing.T) {
	x, err := NewDocDialectExtractor("unity-asmdef")
	if err != nil {
		t.Fatal(err)
	}
	r, err := x.ExtractDoc("fixture", "Assets/Adapters/Adapters.asmdef", []byte(`{
 "name":"Fixture.Adapters",
 "references":["Fixture.Rules","GUID:0123456789abcdef0123456789abcdef"],
 "noEngineReferences":false,
 "autoReferenced":true
}`))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, n := range r.Nodes {
		got = append(got, string(n.Kind)+":"+n.Name)
		if len(n.Calls) != 0 {
			t.Errorf("declaration became calls: %+v", n)
		}
	}
	want := []string{"UnityAssemblyDefinition:Fixture.Adapters", "UnityAssemblyReference:Fixture.Rules", "UnityAssemblyReference:GUID:0123456789abcdef0123456789abcdef"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("nodes=%q, want %q", got, want)
	}
	if r.Nodes[0].Signature != "noEngineReferences=false; autoReferenced=true" {
		t.Errorf("signature=%q", r.Nodes[0].Signature)
	}
	var facts []string
	for _, f := range r.Facts {
		facts = append(facts, f.Key+"="+f.Value)
	}
	if !reflect.DeepEqual(facts, []string{"reference=Fixture.Rules", "reference=GUID:0123456789abcdef0123456789abcdef", "noEngineReferences=false"}) {
		t.Errorf("facts=%q", facts)
	}
	if len(r.CrossFile) != 0 {
		t.Fatal("declared references must not become resolved/typed edges")
	}
}

func TestUnityAsmdefMissingFlagIsNotFalseEvidence(t *testing.T) {
	x, err := NewDocDialectExtractor("unity-asmdef")
	if err != nil {
		t.Fatal(err)
	}
	r, err := x.ExtractDoc("fixture", "Assets/Core/Core.asmdef", []byte(`{"name":"Fixture.Core"}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Nodes) != 1 || len(r.Facts) != 0 || r.Nodes[0].Signature != "" {
		t.Fatalf("absent declaration was invented: %+v", r)
	}
	broken, err := x.ExtractDoc("fixture", "Assets/Broken.asmdef", []byte(`{"name":`))
	if err != nil {
		t.Fatal(err)
	}
	if len(broken.Nodes) != 0 {
		t.Fatal("malformed JSON produced declarations")
	}
}

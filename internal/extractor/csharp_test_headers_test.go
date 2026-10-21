package extractor

import (
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/core"
)

func TestCSharpMethodHeadersKeepCompletePolicyWithoutChangingDisplay(t *testing.T) {
	for _, policy := range []string{"", "[Retry(2)]", "[NUnit.Framework.Repeat(2)]"} {
		t.Run(policy, func(t *testing.T) {
			t.Parallel() // Concurrent header reads must not mutate shared display rules.
			src := []byte("namespace Workshop { class BowlTests { [TestCase(1)] [TestCase(2)] [Category(\"" + strings.Repeat("category", 45) + "\")] " + policy + " public void Fits(int n) {} } }")
			ordinary, err := NewTSExtractor("csharp")
			if err != nil {
				t.Fatal(err)
			}
			defer ordinary.Close()
			before, err := ordinary.Extract("svc", "Assets/BowlTests.cs", src)
			if err != nil {
				t.Fatal(err)
			}
			headers, err := CSharpMethodHeaders("svc", "Assets/BowlTests.cs", src)
			if err != nil || len(headers) != 1 {
				t.Fatalf("headers=%+v err=%v", headers, err)
			}
			h := headers[0]
			if h.Name != "BowlTests.Fits" || h.Package != "Workshop" || h.Line != 1 || h.EndLine != 1 || !h.IsTest() || !strings.HasSuffix(h.Signature, "public void Fits(int n)") {
				t.Fatalf("incomplete or wrong method policy: %+v", h)
			}
			if policy != "" && !strings.Contains(h.Signature, policy) {
				t.Fatal("a policy beyond the display boundary was lost")
			}
			after, err := ordinary.Extract("svc", "Assets/BowlTests.cs", src)
			if err != nil || len(after) != len(before) {
				t.Fatalf("ordinary extractor changed: %+v %v", after, err)
			}
			for i := range before {
				if before[i].Signature != after[i].Signature {
					t.Fatal("read-side header extraction mutated the shared display policy")
				}
				if before[i].Kind == core.KindCSharpMethod && (!strings.HasSuffix(before[i].Signature, "…") || before[i].IsTest()) {
					t.Fatal("the ordinary clipped display must stay fail-closed")
				}
			}
		})
	}
}

func TestCSharpMethodHeadersRefuseOverboundPolicyAndSource(t *testing.T) {
	for _, value := range []string{strings.Repeat("x", csharpTestHeaderLimit+1), strings.Repeat("界", csharpTestHeaderLimit/3+1)} {
		src := []byte("namespace Workshop { class BowlTests { [TestCase(1)] [Category(\"" + value + "\")] public void Fits(int n) {} [Test] public void Other() {} } }")
		headers, err := CSharpMethodHeaders("svc", "Assets/BowlTests.cs", src)
		if err != nil || len(headers) != 1 || headers[0].Name != "BowlTests.Other" {
			t.Fatalf("oversize policy must not hide a small sibling or become a positive header: %+v %v", headers, err)
		}
	}
	if _, err := CSharpMethodHeaders("svc", "Assets/BowlTests.cs", make([]byte, csharpTestSourceLimit+1)); err == nil {
		t.Fatal("overbound source accepted")
	}
}

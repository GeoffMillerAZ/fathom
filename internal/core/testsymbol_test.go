package core

import "testing"

func TestCSharpTestAttributeIdentity(t *testing.T) {
	for _, signature := range []string{
		"[Test] public void KeepsStock()", "[TestAttribute] void KeepsStock()",
		"[NUnit.Framework.Test] void KeepsStock()", "[global::NUnit.Framework.TestAttribute] void KeepsStock()",
		"[TestCase(1, 2)] void KeepsStock(int a, int b)", "[TestCaseSource(nameof(Data))] void KeepsStock(int a)",
		"[UnityTest] IEnumerator Places()", "[UnityEngine.TestTools.UnityTestAttribute] IEnumerator Places()",
		"[Description(\"], [Other.Test]\"), NUnit.Framework.Test] void KeepsStock()",
		"[Description(@\"[Test]\")][Test] void KeepsStock()", "[method: Test] void KeepsStock()",
		"[Theory] void KeepsStock()",
	} {
		n := SymbolNode{Name: "Cases.KeepsStock", Kind: KindCSharpMethod, Signature: signature}
		if !n.IsTest() || n.TestIdentity() == "" {
			t.Errorf("missing syntactic identity: %s", signature)
		}
	}
	for _, signature := range []string{
		"void KeepsStock()", "void TestKeepsStock()", "[SetUp] void KeepsStock()", "[TearDown] void KeepsStock()",
		"[OneTimeSetUp] void KeepsStock()", "[Other.Test] void KeepsStock()", "[Other.TestAttribute] void KeepsStock()",
		"[Description(\"[Test]\")] void KeepsStock()", "[Description(@\"[Test]\")] void KeepsStock()",
		"[return: Test] object KeepsStock()", "[Te st] void KeepsStock()", "[Description(\"unterminated",
		"[TestCase(1, 2)", "[TestFixture] void KeepsStock()", "[TestCaseData] void KeepsStock()",
	} {
		if n := (SymbolNode{Name: "TestCases.TestKeepsStock", Kind: KindCSharpMethod, Signature: signature}); n.IsTest() {
			t.Errorf("non-test or unreadable syntax counted: %s", signature)
		}
	}
	if n := (SymbolNode{Name: "TestCases", Kind: KindCSharpClass, Signature: "[TestFixture] class TestCases"}); n.IsTest() {
		t.Fatal("fixture class is not a test case")
	}
	if n := (SymbolNode{Name: "TestKeepsStock", Kind: KindGoFunc}); !n.IsTest() || n.TestIdentity() != "name-prefix" {
		t.Fatal("legacy Go naming changed")
	}
}

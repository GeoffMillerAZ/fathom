package langbench

import "testing"

// TestScipBareNameJava pins scip-java's naming (LB-F71): a constructor,
// written `<init>`, is named as its class — which is also how new Foo(...)
// is written as a call — and every other leaf reads as it does for any
// producer.
func TestScipBareNameJava(t *testing.T) {
	const p = "semanticdb maven maven/com.google.guava/guava 33.7.1-jre "
	for sym, want := range map[string]string{
		p + "com/google/common/hash/MessageDigestHashFunction#`<init>`().":     "MessageDigestHashFunction",
		p + "com/google/common/hash/MessageDigestHashFunction#`<init>`(+1).":   "MessageDigestHashFunction",
		p + "com/google/common/graph/MapRetrievalCache#CacheEntry#`<init>`().": "CacheEntry",
		p + "com/google/common/base/Preconditions#checkNotNull().":             "checkNotNull",
		p + "com/google/common/base/Preconditions#checkNotNull(+1).":           "checkNotNull",
		p + "com/google/common/base/Optional#":                                 "Optional",
		p + "com/google/common/base/Optional#absent().":                        "absent",
		p + "com/google/common/collect/ImmutableList#EMPTY.":                   "EMPTY",
		p + "com/google/common/hash/BloomFilter#Strategy#`<init>`().":          "Strategy",
		p + "com/google/common/base/Absent#`<clinit>`().":                      "<clinit>",
	} {
		if got, _ := scipBareName(sym); got != want {
			t.Errorf("scipBareName(%q) = %q, want %q", sym, got, want)
		}
	}
	if _, callable := scipBareName(p + "com/google/common/base/Optional#`<init>`()."); !callable {
		t.Error("a constructor is callable: new Foo() is a call to it")
	}
	// A symbol of another producer keeps its leaf: `<init>` is not renamed outside scip-java's.
	if got, _ := scipBareName("scip-other pkg v 1 a/Foo#`<init>`()."); got != "<init>" {
		t.Errorf("another producer's `<init>` = %q, want it untouched", got)
	}
}

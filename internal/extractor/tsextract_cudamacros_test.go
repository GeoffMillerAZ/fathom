package extractor

import (
	"strings"
	"testing"
)

func TestCudaMacroTextsBlanksAlignAtEqualLength(t *testing.T) {
	src := []byte("typedef struct __align__(16) {\n  float x;\n} Pair;\n__shared__ int __align__(8) s[4];\n// __align__(2) in a comment\nconst char *m = \"__align__(3)\";\n")
	texts := cudaMacroTexts(src)
	if len(texts) == 0 {
		t.Fatal("no candidate text for a file that holds __align__")
	}
	got := string(texts[len(texts)-1])
	if len(got) != len(src) {
		t.Fatalf("length %d, want %d: the step must keep every byte offset", len(got), len(src))
	}
	if strings.Count(got, "\n") != strings.Count(string(src), "\n") {
		t.Fatal("the step moved a line")
	}
	if strings.Contains(strings.SplitN(got, "\n", 2)[0], "__align__") || strings.Contains(strings.Split(got, "\n")[3], "__align__") {
		t.Fatalf("an __align__ in code was left:\n%s", got)
	}
	for _, kept := range []string{"// __align__(2) in a comment", "\"__align__(3)\""} {
		if !strings.Contains(got, kept) {
			t.Fatalf("%q was rewritten:\n%s", kept, got)
		}
	}
}

func TestCudaMacroTextsLeavesPlainFilesToTheCppStep(t *testing.T) {
	if texts := cudaMacroTexts([]byte("__global__ void k(int *p) { p[0] = 1; }\n")); texts != nil {
		t.Fatalf("a file with no macro and no conditional got candidates: %q", texts)
	}
}

func TestCudaMacroTextsReadsTypeidAsSizeof(t *testing.T) {
	src := []byte("template <typename R> void f() {\n  if (typeid(R) == typeid(double)) {}\n}\n")
	texts := cudaMacroTexts(src)
	if len(texts) == 0 {
		t.Fatal("no candidate text for typeid")
	}
	got := string(texts[len(texts)-1])
	if len(got) != len(src) || !strings.Contains(got, "sizeof(R) == sizeof(double)") {
		t.Fatalf("typeid was not read as sizeof at equal length:\n%s", got)
	}
}

func TestCudaMacroTextsBlanksExplicitInstantiations(t *testing.T) {
	src := []byte("template <typename T> struct Box { T v; };\ntemplate class Box<float>;\nextern template struct Box<int>;\n")
	texts := cudaMacroTexts(src)
	if len(texts) == 0 {
		t.Fatal("no candidate text for an explicit instantiation")
	}
	got := string(texts[len(texts)-1])
	if len(got) != len(src) || strings.Contains(got, "Box<float>") || strings.Contains(got, "Box<int>") || strings.Contains(got, "extern") {
		t.Fatalf("instantiations were not blanked at equal length:\n%s", got)
	}
	if !strings.Contains(got, "template <typename T> struct Box { T v; };") {
		t.Fatalf("the template definition was rewritten:\n%s", got)
	}
}

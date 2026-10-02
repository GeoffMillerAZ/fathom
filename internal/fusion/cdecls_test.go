package fusion

import (
	"reflect"
	"sort"
	"testing"
)

// TestScanDeclaredFunctions: the names a header's function prototypes declare,
// from its text — no parse, so it must tell a prototype from everything else a
// header holds at file scope.
func TestScanDeclaredFunctions(t *testing.T) {
	cases := []struct {
		name, src string
		want      []string
	}{
		{"plain", "int f(int a);\nvoid g(void);\n", []string{"f", "g"}},
		{"export-macro", "UV_EXTERN int uv_loop_init(uv_loop_t* loop);\nCURL_EXTERN CURLcode curl_easy_perform(CURL *curl);\n", []string{"curl_easy_perform", "uv_loop_init"}},
		{"pointer-return", "const char* uv_strerror(int err);\nchar **names(void);\n", []string{"names", "uv_strerror"}},
		{"multi-line", "int long_one(int a,\n             int b,\n             int c);\n", []string{"long_one"}},
		{"extern-c-block", "extern \"C\" {\nint inside(int);\n}\nint after(int);\n", []string{"after", "inside"}},
		{"extern-c-single", "extern \"C\" int single(int);\n", []string{"single"}},
		{"namespace-block", "namespace ns {\nint in_ns(int);\n}\n", []string{"in_ns"}},
		{"attribute", "__attribute__((noreturn)) void die(const char *m);\nint warn(const char *f, ...) __attribute__((format(printf, 1, 2)));\n", []string{"die", "warn"}},
		{"comments-and-strings", "/* int not_this(int); */\n// int nor_this(int);\nint real(int); /* trailing */\nconst char *s = \"int in_string(int);\";\n", []string{"real"}},
		{"preprocessor", "#define MACRO(x) int in_macro(x);\n#if FOO\nint guarded(int);\n#endif\n#define LONG \\\n  int in_continued(int);\n", []string{"guarded"}},
		{"not-prototypes", "typedef int (*cb)(int);\nvoid (*hook)(int);\nstruct ops { int (*run)(int); int n; };\nenum e { A, B };\nint var = 3;\nstatic_assert(sizeof(int) == 4, \"x\");\nMACRO_CALL(arg);\n", nil},
		{"function-bodies-skipped", "static inline int body(int v) { return helper(v); }\nint decl_after(int);\n", []string{"decl_after"}},
		{"struct-body-skipped", "struct s {\n  int a;\n  void (*f)(int);\n};\nint after_struct(struct s *p);\n", []string{"after_struct"}},
		{"unterminated", "int open_ended(int", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var got []string
			for n := range scanDeclaredFunctions([]byte(c.src)) {
				got = append(got, n)
			}
			sort.Strings(got)
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("scanDeclaredFunctions = %v, want %v", got, c.want)
			}
		})
	}
}

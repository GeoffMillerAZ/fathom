package iacfacts

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"testing/fstest"
)

func tfValue(t *testing.T, src string) (Attr, error) {
	t.Helper()
	set, err := ReadTerraformFile("a.tf", []byte(src), WithValues())
	if err != nil {
		return Attr{}, err
	}
	if len(set.Resources) != 1 {
		t.Fatalf("%+v", set)
	}
	return attrOf(t, set.Resources[0], "x"), nil
}

func pulumiProgram(config, vars, prop string) string {
	return "configuration:\n" + config + "variables:\n" + vars + "resources:\n  r:\n    type: a:b:C\n    properties:\n      p: " + prop + "\n"
}

func pulumiValue(t *testing.T, src string) (Attr, error) {
	t.Helper()
	set, err := ReadPulumiYAMLFile("Pulumi.yaml", []byte(src), WithValues())
	if err != nil {
		return Attr{}, err
	}
	if len(set.Resources) != 1 {
		t.Fatalf("%+v", set)
	}
	return attrOf(t, set.Resources[0], "p"), nil
}

// IAC37: the value budgets charged a float64 eight bytes and a json.Number its text, but a template prints a number
// positionally: "${1e-300}" is 302 bytes. 100,000 such parts returned a value of 123.5 MB with no refusal, against
// MaxValueBytes of 1 MiB counted as a consumer would serialise it. A list of int64 marshals longer than 8 bytes each.
func TestValueBudgetsChargeANumberAtItsPrintedLength(t *testing.T) {
	for name, part := range map[string]string{
		"float64 1e308":   "${1e308}",
		"float64 1e-300":  "${1e-300}",
		"json.Number":     "${1.0000000000000000000000000000001e300}",
		"json.Number low": "${1.0000000000000000000000000000001e-300}",
	} {
		// 6,000 parts print to well over 1 MiB, in a file well under MaxFileBytes.
		src := "resource \"t\" \"n\" {\n  x = \"" + strings.Repeat(part, 6000) + "\"\n}\n"
		if _, err := tfValue(t, src); !errors.Is(err, ErrLimit) {
			t.Errorf("%s x 6000: want ErrLimit, got %v", name, err)
		}
		// A handful still print.
		a, err := tfValue(t, "resource \"t\" \"n\" {\n  x = \"a"+strings.Repeat(part, 3)+"\"\n}\n")
		if err != nil || a.Value == nil {
			t.Errorf("%s x 3: %v %#v", name, err, a.Value)
		}
	}
	// A list of int64 is charged at its decimal length, so 100,000 of them is past MaxValueBytes though it was 800 KB.
	list := "resource \"t\" \"n\" {\n  x = [" + strings.Repeat("9223372036854775807,", 100_000) + "]\n}\n"
	if _, err := tfValue(t, list); !errors.Is(err, ErrLimit) {
		t.Errorf("100,000 int64: want ErrLimit, got %v", err)
	}
	small := "resource \"t\" \"n\" {\n  x = [" + strings.Repeat("7,", 100_000) + "]\n}\n"
	if _, err := tfValue(t, small); err != nil {
		t.Errorf("100,000 one-digit ints: %v", err)
	}
}

// IAC38: a number literal over 1,024 bytes of text is never parsed by big.Float: it reads as its source text (the
// IAC11 rule) and no template makes a value of it. big.ParseFloat of 900 KB allocates 2 GB, and a Pulumi variable
// that held it ran the parse once for every use.
func TestALongNumberLiteralReadsAsTextAndIsNoTemplateValue(t *testing.T) {
	digits := func(n int) string { return "0." + strings.Repeat("1", n-2) }
	if v, ok := numberValue(digits(1024)).(string); ok {
		t.Errorf("1,024 bytes of number text reads as text %q", v[:10])
	}
	if _, ok := numberValue(digits(1025)).(string); !ok {
		t.Errorf("1,025 bytes of number text must read as its text, got %T", numberValue(digits(1025)))
	}
	long := digits(900_000)
	a, err := tfValue(t, "resource \"t\" \"n\" {\n  x = "+long+"\n}\n")
	if err != nil || a.Value != long {
		t.Errorf("a 900 KB literal: %v, value is text %v", err, a.Value == long)
	}
	a, err = tfValue(t, "resource \"t\" \"n\" {\n  x = \"a${"+long+"}\"\n}\n")
	if err != nil || a.Value != nil {
		t.Errorf("a template over a 900 KB literal: %v, want no value, got %d bytes", err, len(fmt.Sprint(a.Value)))
	}
	p := pulumiProgram("  n:\n    default: "+long+"\n", "  v: \"a${n}\"\n", "\"${v}\"")
	cpuWithin(t, 5e9, "pulumi, a 900 KB number interpolated", func() {
		a, err := pulumiValue(t, p)
		if err != nil || a.Value != nil {
			t.Errorf("pulumi, a template over a 900 KB number: %v %v", err, a.Value != nil)
		}
	})
	// Pulumi's interpolation holds the string it builds, as Terraform's does, even when a list that holds it and a
	// reference drops it: 30 variables that each build a 900 KB string are 27 MB that no emit sees.
	big := strings.Repeat("s", 900_000)
	var vars, props strings.Builder
	for i := 0; i < 30; i++ {
		fmt.Fprintf(&vars, "  v%d: [\"${s}%d\", \"${r.id}\"]\n", i, i)
		fmt.Fprintf(&props, "      q%d: ${v%d}\n", i, i)
	}
	prog := "configuration:\n  s:\n    default: " + big + "\nvariables:\n" + vars.String() +
		"resources:\n  r:\n    type: a:b:C\n    properties:\n      p: ${s}\n" + props.String()
	if _, err := ReadPulumiYAMLFile("Pulumi.yaml", []byte(prog), WithValues()); !errors.Is(err, ErrLimit) {
		t.Errorf("pulumi, 30 variables of 900 KB: want ErrLimit, got %v", err)
	}
}

// IAC39: pulumi-yaml prints an interpolated number with %v of a float64 (run.go: NumberExpr returns the float64, and
// evaluateInterpolations writes it with %v), so 1e6 is 1e+06 and 0.00001 is 1e-05, where OpenTofu prints digits. An
// entry typed integer casts its default to an int (run.go: the Int case of the configuration), which prints whole.
// Checked against pulumi-yaml v1.36.0's source, not by running its engine.
func TestPulumiInterpolatesNumbersAsAFloat64(t *testing.T) {
	for _, tc := range []struct{ name, config, want string }{
		{"1e6", "default: 1e6", "a1e+06"},
		{"0.00001", "default: 0.00001", "a1e-05"},
		{"12345678901234567890", "default: 12345678901234567890", "a1.2345678901234567e+19"},
		{"1234567 untyped", "default: 1234567", "a1.234567e+06"},
		{"100000", "default: 100000", "a100000"},
		{"1.5", "default: 1.5", "a1.5"},
		{"1234567 integer", "type: integer\n    default: 1234567", "a1234567"},
		{"1234567 int", "type: int\n    default: 1234567", "a1234567"},
		{"1e6 integer", "type: Integer\n    default: 1e6", "a1000000"},
		{"1234567 number", "type: number\n    default: 1234567", "a1.234567e+06"},
		{"true", "default: true", "atrue"},
	} {
		prog := pulumiProgram("  n:\n    "+tc.config+"\n", "", "\"a${n}\"")
		a, err := pulumiValue(t, prog)
		if err != nil || a.Value != tc.want {
			t.Errorf("%s: %v %#v, want %q", tc.name, err, a.Value, tc.want)
		}
		// Through a variable, the printing is the same.
		a, err = pulumiValue(t, pulumiProgram("  n:\n    "+tc.config+"\n", "  v: ${n}\n", "\"a${v}\""))
		if err != nil || a.Value != tc.want {
			t.Errorf("%s through a variable: %v %#v, want %q", tc.name, err, a.Value, tc.want)
		}
	}
	// A number literal in a variable is a float64 too.
	a, err := pulumiValue(t, pulumiProgram("", "  v: 1234567\n", "\"a${v}\""))
	if err != nil || a.Value != "a1.234567e+06" {
		t.Errorf("a literal variable: %v %#v", err, a.Value)
	}
	// Terraform keeps printing digits.
	if a, err := tfValue(t, "resource \"t\" \"n\" {\n  x = \"a${1e6}\"\n}\n"); err != nil || a.Value != "a1000000" {
		t.Errorf("terraform 1e6: %v %#v", err, a.Value)
	}
}

// IAC40: a Terraform number past float64's range, or -0, used in a template has no value. tofu prints the positional
// digits of a 512-bit number (1e400 is a 1 and 400 zeros, 320 nines print as themselves rounded at 512 bits) and -0
// as -0, which no Go value this reader returns for the literal says, so the template reads no value and does not
// splice the source text. Numbers in range keep agreeing with tofu.
func TestTerraformNumbersPastFloat64HaveNoTemplateValue(t *testing.T) {
	for _, expr := range []string{"1e309", "1e400", strings.Repeat("9", 320), "-0", "1e-2000", "-1e309"} {
		src := fmt.Sprintf("resource \"t\" \"n\" {\n  x = \"a${%s}\"\n}\n", expr)
		a, err := tfValue(t, src)
		if err != nil || a.Value != nil {
			t.Errorf("a${%s}: %v value %#v, want none", expr, err, a.Value)
		}
	}
	// A local and a default carry the same flag.
	mod := fstest.MapFS{"main.tf": {Data: []byte("locals {\n  n = 1e400\n  z = -0\n}\nvariable \"d\" {\n  default = 1e400\n}\n" +
		"resource \"t\" \"n\" {\n  a = \"x${local.n}\"\n  b = \"x${local.z}\"\n  c = \"x${var.d}\"\n}\n")}}
	set, err := ReadTerraformModule(mod, ".", WithValues())
	if err != nil || len(set.Resources) != 1 {
		t.Fatalf("%v %+v", err, set)
	}
	for _, p := range []string{"a", "b", "c"} {
		if a := attrOf(t, set.Resources[0], p); a.Value != nil {
			t.Errorf("%s: %#v, want no value", p, a.Value)
		}
	}
	// The number alone still reads, as its text and as -0's int64.
	if a, err := tfValue(t, "resource \"t\" \"n\" {\n  x = 1e400\n}\n"); err != nil || a.Value != "1e400" {
		t.Errorf("1e400 alone: %v %#v", err, a.Value)
	}
	// In range, the digits agree with tofu (files_tofu_test pins the rest).
	for expr, want := range map[string]string{
		"-0.0": "a-0", "0.0": "a0", "-1": "a-1", "5e-324": "a0." + strings.Repeat("0", 323) + "5",
		"1.5e-300": "a0." + strings.Repeat("0", 299) + "15", "1e308": "a1" + strings.Repeat("0", 308),
	} {
		a, err := tfValue(t, fmt.Sprintf("resource \"t\" \"n\" {\n  x = \"a${%s}\"\n}\n", expr))
		if err != nil || a.Value != want {
			t.Errorf("a${%s}: %v %#v, want %q", expr, err, a.Value, want)
		}
	}
}

// IAC33: one tfReader serves a whole module, so MaxNodes and the reference search's byte budget are the module's and
// not a file's. Four files that each read alone refuse together, and the limits' comments say so.
func TestTerraformNodeBudgetIsTheModules(t *testing.T) {
	body := func(i int) string {
		return fmt.Sprintf("resource \"t\" \"n%d\" {\n  x = [%s]\n}\n", i, strings.Repeat("1,", 300_000))
	}
	if _, err := ReadTerraformModule(fstest.MapFS{"a.tf": {Data: []byte(body(0))}}, ".", WithValues()); err != nil {
		t.Fatalf("one file of 300,000 items: %v", err)
	}
	m := fstest.MapFS{}
	for i := 0; i < 4; i++ {
		m[fmt.Sprintf("f%d.tf", i)] = &fstest.MapFile{Data: []byte(body(i))}
	}
	if _, err := ReadTerraformModule(m, ".", WithValues()); !errors.Is(err, ErrLimit) {
		t.Errorf("four such files: want ErrLimit from the module's node budget, got %v", err)
	}
}

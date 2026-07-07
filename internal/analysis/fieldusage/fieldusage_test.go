package fieldusage_test

import (
	"reflect"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/analysis/fieldusage"
)

const basicSrc = `package foo

type User struct {
	ID    int
	Name  string
	Email string
	Age   int
}

func doSomething(u User) string {
	_ = u.ID
	return u.Name
}
`

const embeddedSrc = `package foo

type Base struct {
	X int
}

type Child struct {
	Base
	Y int
}

func use(c Child) {
	_ = c.Y
}
`

const unusedSrc = `package foo

type Config struct {
	Host string
	Port int
	TLS  bool
}

func nothing() {}
`

const noSuchStructSrc = `package foo

type Other struct {
	Val int
}
`

const multiStructSrc = `package foo

type Alpha struct {
	A int
	B string
}

type Beta struct {
	C int
	D string
}

func f(a Alpha, b Beta) {
	_ = a.A
	_ = b.D
}
`

// ─── Scan ─────────────────────────────────────────────────────────────────────

func TestScan_BasicUsed(t *testing.T) {
	got := fieldusage.Scan("user.go", basicSrc, "User")
	want := []fieldusage.FieldUse{
		{Field: "ID", Used: true},
		{Field: "Name", Used: true},
		{Field: "Email", Used: false},
		{Field: "Age", Used: false},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Scan(User) =\n  %v\nwant\n  %v", got, want)
	}
}

func TestScan_AllUnused(t *testing.T) {
	got := fieldusage.Scan("cfg.go", unusedSrc, "Config")
	want := []fieldusage.FieldUse{
		{Field: "Host", Used: false},
		{Field: "Port", Used: false},
		{Field: "TLS", Used: false},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Scan(Config) = %v, want %v", got, want)
	}
}

func TestScan_StructNotFound(t *testing.T) {
	got := fieldusage.Scan("f.go", noSuchStructSrc, "Missing")
	if got != nil {
		t.Errorf("Scan(Missing) = %v, want nil", got)
	}
}

func TestScan_EmbeddedField(t *testing.T) {
	got := fieldusage.Scan("child.go", embeddedSrc, "Child")
	// Fields: Base (embedded, unused via selector on Child), Y (used)
	if len(got) != 2 {
		t.Fatalf("Scan(Child) len = %d, want 2; got %v", len(got), got)
	}
	// Y should be used.
	var yUsed bool
	for _, fu := range got {
		if fu.Field == "Y" {
			yUsed = fu.Used
		}
	}
	if !yUsed {
		t.Errorf("Scan(Child): expected Y to be used, got %v", got)
	}
}

func TestScan_MultipleStructs_TargetAlpha(t *testing.T) {
	got := fieldusage.Scan("multi.go", multiStructSrc, "Alpha")
	want := []fieldusage.FieldUse{
		{Field: "A", Used: true},
		{Field: "B", Used: false},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Scan(Alpha) = %v, want %v", got, want)
	}
}

func TestScan_MultipleStructs_TargetBeta(t *testing.T) {
	got := fieldusage.Scan("multi.go", multiStructSrc, "Beta")
	want := []fieldusage.FieldUse{
		{Field: "C", Used: false},
		{Field: "D", Used: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Scan(Beta) = %v, want %v", got, want)
	}
}

func TestScan_ParseError(t *testing.T) {
	got := fieldusage.Scan("bad.go", "this is not valid Go {{{{", "Foo")
	if got != nil {
		t.Errorf("Scan(invalid Go) = %v, want nil", got)
	}
}

func TestScan_Determinism(t *testing.T) {
	a := fieldusage.Scan("user.go", basicSrc, "User")
	b := fieldusage.Scan("user.go", basicSrc, "User")
	if !reflect.DeepEqual(a, b) {
		t.Errorf("Scan not deterministic: %v vs %v", a, b)
	}
}

// ─── ScanSorted ───────────────────────────────────────────────────────────────

func TestScanSorted_AlphaOrder(t *testing.T) {
	got := fieldusage.ScanSorted("user.go", basicSrc, "User")
	// Should be in alphabetical order: Age, Email, ID, Name.
	names := make([]string, len(got))
	for i, fu := range got {
		names[i] = fu.Field
	}
	want := []string{"Age", "Email", "ID", "Name"}
	if !reflect.DeepEqual(names, want) {
		t.Errorf("ScanSorted fields = %v, want %v", names, want)
	}
}

package iacfacts

import (
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode"
)

// IAC22, IAC23: pulumi-yaml matches the fields of its records with strings.EqualFold, and the reader folded with
// strings.ToLower, which is another relation: it leaves U+017F (long s) alone and maps U+0130 to i. foldKey is the
// key EqualFold's equality has, so the two agree on every rune.
func TestFoldKeyIsEqualFold(t *testing.T) {
	probes := []string{"i", "I", "s", "S", "k", "K", "\u0130", "\u0131", "\u017f", "\u212a", "\u03c3", "\u03c2", "\u03a3"}
	for r := rune(0); r < 0x20000; r++ {
		if !unicode.IsPrint(r) && r > 0x7f {
			continue
		}
		a := string(r)
		others := append([]string{string(unicode.SimpleFold(r)), string(unicode.ToLower(r)), string(unicode.ToUpper(r))}, probes...)
		for _, b := range others {
			if (foldKey(a) == foldKey(b)) != strings.EqualFold(a, b) {
				t.Fatalf("%U vs %q: foldKey says %v, EqualFold says %v", r, b, foldKey(a) == foldKey(b), strings.EqualFold(a, b))
			}
		}
	}
	for _, c := range [][2]string{{"secret", "\u017fecret"}, {"SECRET", "\u017fECRET"}, {"resources", "re\u017fources"}, {"properties", "propertie\u017f"}, {"kind", "\u212aind"}} {
		if foldKey(c[0]) != foldKey(c[1]) {
			t.Errorf("%q and %q must fold alike", c[0], c[1])
		}
	}
	if foldKey("properties") == foldKey("propert\u0130es") {
		t.Error("U+0130 is not an i to EqualFold")
	}
}

func TestPulumiFieldsFoldAsEqualFoldDoes(t *testing.T) {
	read := func(src string) *ResourceSet {
		set, err := ReadPulumiYAMLFile("Pulumi.yaml", []byte(src), WithValues())
		if err != nil {
			t.Fatal(err)
		}
		return set
	}
	cfg := func(flag string) string {
		return "configuration:\n  pw:\n    default: hunter2\n    " + flag + ": true\nresources:\n  r:\n    type: a:b:C\n    properties:\n      x: ${pw}\n"
	}
	for _, flag := range []string{"\u017fecret", "\u017fECRET", "SECRET"} {
		set := read(cfg(flag))
		if len(set.Resources) != 1 {
			t.Fatalf("%q: %+v", flag, set)
		}
		for _, a := range set.Resources[0].Attrs {
			if a.Value != nil {
				t.Errorf("%q: %s returns %#v", flag, a.Path, a.Value)
			}
		}
	}
	// IAC23: the sections read whatever spelling EqualFold accepts, and a spelling it rejects reads nothing.
	body := "name: p\nruntime: yaml\n%s:\n  r:\n    type: a:b:C\n    %s:\n      x: kept\n"
	for _, c := range []struct {
		res, props string
		want       bool
	}{
		{"resources", "properties", true},
		{"re\u017fources", "properties", true},
		{"resources", "propertie\u017f", true},
		{"RE\u017fOURCES", "PROPERTIE\u017f", true},
		{"resources", "propert\u0130es", false},
	} {
		set := read(fmt.Sprintf(body, c.res, c.props))
		got := len(set.Resources) == 1 && len(set.Resources[0].Attrs) == 1
		if got != c.want {
			t.Errorf("%q / %q: reads %v, want %v: %+v", c.res, c.props, got, c.want, set)
		}
	}
}

// IAC30: pulumi-yaml's GetConfig prioritises `config` over `configuration`, entry by entry, and the reader took
// `configuration` first, so a secret flag in `config` was ignored and a default read from the wrong section.
func TestPulumiConfigEntriesOutrankConfigurationEntries(t *testing.T) {
	const tail = "resources:\n  r:\n    type: a:b:C\n    properties:\n      x: ${pw}\n"
	read := func(src string) []Attr {
		set, err := ReadPulumiYAMLFile("Pulumi.yaml", []byte(src), WithValues())
		if err != nil || len(set.Resources) != 1 {
			t.Fatalf("%v %+v", err, set)
		}
		return set.Resources[0].Attrs
	}
	for name, src := range map[string]string{
		"config declared second": "configuration:\n  pw:\n    default: hunter2\nconfig:\n  pw:\n    default: hunter2\n    secret: true\n" + tail,
		"config declared first":  "config:\n  pw:\n    default: hunter2\n    secret: true\nconfiguration:\n  pw:\n    default: hunter2\n" + tail,
	} {
		for _, a := range read(src) {
			if a.Value != nil {
				t.Errorf("%s: %s returns %#v", name, a.Path, a.Value)
			}
		}
	}
	for name, src := range map[string]string{
		"config declared second": "configuration:\n  pw:\n    default: a\nconfig:\n  pw:\n    default: b\n" + tail,
		"config declared first":  "config:\n  pw:\n    default: b\nconfiguration:\n  pw:\n    default: a\n" + tail,
	} {
		if as := read(src); len(as) != 1 || as[0].Value != "b" {
			t.Errorf("%s: want the default of config, got %+v", name, as)
		}
	}
	// An entry only `configuration` has still reads.
	if as := read("configuration:\n  pw:\n    default: a\nconfig:\n  other:\n    default: b\n" + tail); len(as) != 1 || as[0].Value != "a" {
		t.Errorf("an entry of configuration alone must read: %+v", as)
	}
}

// IAC20: node lookups were indexed but isSecretFlag, called for every ${name}, still scanned the whole config entry
// for a merge key. A 2 MB file with an entry of 85,000 keys and 180,000 interpolations took 1m13s. The test goes
// through the reader, which is what the node-level test could not see.
func TestPulumiConfigEntryWithManyKeysIsReadInTime(t *testing.T) {
	var b strings.Builder
	b.WriteString("name: p\nruntime: yaml\nconfig:\n  pw:\n    default: d\n")
	for i := 0; i < 85_000; i++ {
		fmt.Fprintf(&b, "    k%d: 1\n", i)
	}
	b.WriteString("resources:\n  r:\n    type: a:b:C\n    properties:\n      x: \"" + strings.Repeat("${pw}", 180_000) + "\"\n")
	within(t, 15*time.Second, "pulumi config", func() {
		set, err := ReadPulumiYAMLFile("Pulumi.yaml", []byte(b.String()))
		if err != nil || len(set.Resources) != 1 {
			t.Errorf("%v", err)
		}
	})
}

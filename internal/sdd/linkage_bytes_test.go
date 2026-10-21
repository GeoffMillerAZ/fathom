package sdd_test

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/sdd"
)

func TestExtractRefsBytesEquivalent(t *testing.T) {
	long := "[FR-" + strings.Repeat("7", 32<<10) + "]"
	cases := []string{
		"", "no refs", "[US-1][EP-22][FR-333][NFR-4] [FR-333]",
		"[X-1] [FR-x] [FR-] [fr-1] [FR-1x] [OTHER-2] [NFR-02]",
		"\xff[FR-1]\x00[EP-3]\xfe", "x[FR-1]y [[US-2]] [NFR-3]\n",
		strings.Repeat("[FR-1][US-2]", 16384),
		strings.Repeat("x", 8190) + "[NFR-7]" + long,
	}
	for i, text := range cases {
		if got, want := sdd.ExtractRefsBytes([]byte(text)), sdd.ExtractRefs(text); !reflect.DeepEqual(got, want) {
			t.Fatalf("case %d: got %v, want %v", i, got, want)
		}
	}
}

func TestExtractRefsBytesOwnsTokensFromReusedBuffer(t *testing.T) {
	long := "[FR-" + strings.Repeat("3", 32<<10) + "]"
	buffer := []byte("[US-1]" + strings.Repeat("[NFR-2]", 8192) + long)
	want := sdd.ExtractRefs(string(buffer))
	got := sdd.ExtractRefsBytes(buffer)
	for i := range buffer {
		buffer[i] = 'x'
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("mutation changed previously extracted owned tokens: got %v, want %v", got, want)
	}
	copy(buffer, []byte("[EP-9]"))
	if next := sdd.ExtractRefsBytes(buffer); !reflect.DeepEqual(next, []string{"[EP-9]"}) {
		t.Fatalf("reused buffer extraction = %v", next)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatal("a later extraction changed the earlier result")
	}
	if sdd.ExtractRefsBytes(bytes.Repeat([]byte{'x'}, 1<<20)) != nil {
		t.Fatal("reference-free source produced refs")
	}
}

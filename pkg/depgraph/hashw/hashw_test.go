package hashw

import (
	"bytes"
	"testing"
)

// TestHashWriterRefusesConcatenationCollisions pins the property the
// writer exists for: every write is tagged
// with its kind and length-prefixed, so no two DIFFERENT write
// sequences can produce the same digest by re-slicing the same bytes.
// Each collision pair below is a real ambiguity a bare sha256.Write
// would accept.
func TestHashWriterRefusesConcatenationCollisions(t *testing.T) {
	seq := func(ops ...func(*Writer)) func(*Writer) {
		return func(w *Writer) {
			for _, op := range ops {
				op(w)
			}
		}
	}
	str := func(s string) func(*Writer) { return func(w *Writer) { w.Str(s) } }
	raw := func(b []byte) func(*Writer) { return func(w *Writer) { w.Bytes(b) } }
	u32 := func(v uint32) func(*Writer) { return func(w *Writer) { w.U32(v) } }

	cases := []struct {
		name string
		a, b func(*Writer)
	}{
		{"boundary shift ab|c vs a|bc", seq(str("ab"), str("c")), seq(str("a"), str("bc"))},
		{"kind tag: Str vs Bytes over the same payload", str("a"), raw([]byte("a"))},
		{"kind tag: Str vs U32 over the same four bytes", str("abcd"), u32(0x61626364)},
		{"an empty write is a write", seq(str(""), str("a")), str("a")},
		{"empty Bytes vs nothing written", raw(nil), seq()},
		{"embedded separator byte cannot fake a boundary", str("a\x00b"), seq(str("a"), str("b"))},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wa, wb := New(), New()
			tc.a(wa)
			tc.b(wb)
			if bytes.Equal(wa.Sum(), wb.Sum()) {
				t.Errorf("two distinct write sequences collided: %x", wa.Sum())
			}
		})
	}

	// The same sequence twice must agree — the writer feeds a cache
	// key, so nondeterminism here is a cold fleet.
	one, two := New(), New()
	for _, w := range []*Writer{one, two} {
		w.Str("acme")
		w.U32(7)
		w.Bytes([]byte{0xde, 0xad})
	}
	if !bytes.Equal(one.Sum(), two.Sum()) {
		t.Errorf("identical sequences produced different sums: %x vs %x", one.Sum(), two.Sum())
	}
	if !bytes.Equal(one.Sum(), one.Sum()) {
		t.Error("Sum is not idempotent")
	}
	if len(one.Sum()) != 32 {
		t.Errorf("Sum length = %d, want 32 (sha256)", len(one.Sum()))
	}
}

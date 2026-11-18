package hashw

import "encoding/binary"

// U64 frames a fixed-width unsigned value; tag 0x04 is reserved for this type.
func (w *Writer) U64(v uint64) {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], v)
	w.frame(0x04, b[:])
}

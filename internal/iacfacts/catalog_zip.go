package iacfacts

import (
	"bufio"
	"compress/flate"
	"encoding/binary"
	"errors"
	"hash"
	"hash/crc32"
	"io"
)

// catZip reads a catalog's zip without materializing its directory. archive/zip builds one object per entry up
// front, which for a catalog near the 500,000 entry bound costs far more than I48's point ceiling, so a lookup
// here streams the central directory and keeps nothing per entry. It reads only what the catalog writer (stdlib
// archive/zip) produces: stored or deflated entries, sizes under 4 GiB, and zip64 end records for the entry count.
type catZip struct {
	r       io.ReaderAt
	size    int64
	entries uint64
	cdOff   int64
	cdSize  int64
}

const (
	sigEOCD      = 0x06054b50
	sigEOCD64    = 0x06064b50
	sigLoc64     = 0x07064b50
	sigCentral   = 0x02014b50
	sigLocal     = 0x04034b50
	eocdFixed    = 22
	centralFixed = 46
	localFixed   = 30
)

type catZipEntry struct {
	method   uint16
	flags    uint16
	crc      uint32
	csize    uint64
	usize    uint64
	localOff uint64
}

func corruptf(format string, args ...any) *IaCRefusal { return refuse(RefuseCorrupt, format, args...) }

func openCatZip(r io.ReaderAt, size int64, lim IaCLimits) (*catZip, error) {
	if size > lim.CatalogBytes {
		return nil, refuse(RefuseCatalogBytes, "the catalog is %d bytes, over the %d byte bound", size, lim.CatalogBytes)
	}
	if size < eocdFixed {
		return nil, corruptf("the file is %d bytes, too short to be a zip", size)
	}
	tailLen := int64(eocdFixed + 0xFFFF)
	if tailLen > size {
		tailLen = size
	}
	tail := make([]byte, tailLen)
	if _, err := r.ReadAt(tail, size-tailLen); err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	eocd := -1
	for i := len(tail) - eocdFixed; i >= 0; i-- {
		if binary.LittleEndian.Uint32(tail[i:]) != sigEOCD {
			continue
		}
		if i+eocdFixed+int(binary.LittleEndian.Uint16(tail[i+20:])) == len(tail) {
			eocd = i
			break
		}
	}
	if eocd < 0 {
		return nil, corruptf("no zip end record")
	}
	z := &catZip{r: r, size: size}
	rec := tail[eocd:]
	z.entries = uint64(binary.LittleEndian.Uint16(rec[10:]))
	z.cdSize = int64(binary.LittleEndian.Uint32(rec[12:]))
	z.cdOff = int64(binary.LittleEndian.Uint32(rec[16:]))
	if z.entries == 0xFFFF || uint32(z.cdSize) == 0xFFFFFFFF || uint32(z.cdOff) == 0xFFFFFFFF {
		eocdAbs := size - tailLen + int64(eocd)
		if eocdAbs < 20 {
			return nil, corruptf("a zip64 marker without a zip64 locator")
		}
		var loc [20]byte
		if _, err := r.ReadAt(loc[:], eocdAbs-20); err != nil || binary.LittleEndian.Uint32(loc[:]) != sigLoc64 {
			return nil, corruptf("a zip64 marker without a zip64 locator")
		}
		off := int64(binary.LittleEndian.Uint64(loc[8:]))
		var e64 [56]byte
		if off < 0 || off+56 > size {
			return nil, corruptf("the zip64 end record is out of range")
		}
		if _, err := r.ReadAt(e64[:], off); err != nil || binary.LittleEndian.Uint32(e64[:]) != sigEOCD64 {
			return nil, corruptf("the zip64 end record is unreadable")
		}
		z.entries = binary.LittleEndian.Uint64(e64[32:])
		z.cdSize = int64(binary.LittleEndian.Uint64(e64[40:]))
		z.cdOff = int64(binary.LittleEndian.Uint64(e64[48:]))
	}
	if z.entries > uint64(lim.Entries) {
		return nil, refuse(RefuseEntries, "the catalog has %d entries, over the %d entry bound", z.entries, lim.Entries)
	}
	if z.cdOff < 0 || z.cdSize < 0 || z.cdOff+z.cdSize > size || uint64(z.cdSize) < z.entries*centralFixed {
		return nil, corruptf("the central directory (%d entries at %d, %d bytes) does not fit the file", z.entries, z.cdOff, z.cdSize)
	}
	return z, nil
}

// each streams every central directory record to fn. name is only valid during the call. fn returns true to stop.
func (z *catZip) each(fn func(name []byte, e catZipEntry) (stop bool, err error)) error {
	br := bufio.NewReaderSize(io.NewSectionReader(z.r, z.cdOff, z.cdSize), 64<<10)
	var fixed [centralFixed]byte
	name := make([]byte, 0, 128)
	for i := uint64(0); i < z.entries; i++ {
		if _, err := io.ReadFull(br, fixed[:]); err != nil {
			return corruptf("the central directory ends at entry %d of %d", i, z.entries)
		}
		if binary.LittleEndian.Uint32(fixed[:]) != sigCentral {
			return corruptf("central directory entry %d has no signature", i)
		}
		nameLen := int(binary.LittleEndian.Uint16(fixed[28:]))
		skip := int(binary.LittleEndian.Uint16(fixed[30:])) + int(binary.LittleEndian.Uint16(fixed[32:]))
		if cap(name) < nameLen {
			name = make([]byte, 0, nameLen)
		}
		name = name[:nameLen]
		if _, err := io.ReadFull(br, name); err != nil {
			return corruptf("central directory entry %d is truncated", i)
		}
		if _, err := br.Discard(skip); err != nil {
			return corruptf("central directory entry %d is truncated", i)
		}
		e := catZipEntry{
			flags:    binary.LittleEndian.Uint16(fixed[8:]),
			method:   binary.LittleEndian.Uint16(fixed[10:]),
			crc:      binary.LittleEndian.Uint32(fixed[16:]),
			csize:    uint64(binary.LittleEndian.Uint32(fixed[20:])),
			usize:    uint64(binary.LittleEndian.Uint32(fixed[24:])),
			localOff: uint64(binary.LittleEndian.Uint32(fixed[42:])),
		}
		stop, err := fn(name, e)
		if err != nil {
			return err
		}
		if stop {
			return nil
		}
	}
	return nil
}

// find returns the entry called name. A name that appears twice is corrupt: two readers could take different ones.
func (z *catZip) find(name string) (catZipEntry, bool, error) {
	var found catZipEntry
	var ok bool
	err := z.each(func(n []byte, e catZipEntry) (bool, error) {
		if string(n) != name {
			return false, nil
		}
		if ok {
			return true, corruptf("entry %q appears twice", name)
		}
		found, ok = e, true
		return false, nil
	})
	return found, ok, err
}

// entryReader yields an entry's bytes, checks them against the directory's size and CRC at the end, and fails
// the moment an entry yields more than it declared.
type entryReader struct {
	r      io.Reader
	remain uint64
	crc    hash.Hash32
	want   uint32
	name   string
	done   bool
}

func (e *entryReader) Read(p []byte) (int, error) {
	if e.done {
		return 0, io.EOF
	}
	n, err := e.r.Read(p)
	if uint64(n) > e.remain {
		return 0, corruptf("entry %q yields more than its declared %d bytes", e.name, e.remain+uint64(n))
	}
	e.remain -= uint64(n)
	e.crc.Write(p[:n])
	if errors.Is(err, io.EOF) {
		e.done = true
		if e.remain != 0 {
			return n, corruptf("entry %q is %d bytes short", e.name, e.remain)
		}
		if e.crc.Sum32() != e.want {
			return n, corruptf("entry %q fails its CRC", e.name)
		}
		return n, io.EOF
	}
	return n, err
}

// open returns a reader over the entry's bytes. Its declared size over limit refuses with code before a byte is read.
func (z *catZip) open(name string, e catZipEntry, limit int64, code string) (io.Reader, error) {
	if e.flags&1 != 0 {
		return nil, corruptf("entry %q is encrypted", name)
	}
	if e.usize > uint64(limit) {
		return nil, refuse(code, "%s is %d bytes, over the %d byte bound", name, e.usize, limit)
	}
	if e.csize == 0xFFFFFFFF || e.usize == 0xFFFFFFFF {
		return nil, corruptf("entry %q needs zip64 sizes", name)
	}
	var loc [localFixed]byte
	if int64(e.localOff)+localFixed > z.cdOff {
		return nil, corruptf("entry %q starts past the data area", name)
	}
	if _, err := z.r.ReadAt(loc[:], int64(e.localOff)); err != nil || binary.LittleEndian.Uint32(loc[:]) != sigLocal {
		return nil, corruptf("entry %q has no local header", name)
	}
	start := int64(e.localOff) + localFixed + int64(binary.LittleEndian.Uint16(loc[26:])) + int64(binary.LittleEndian.Uint16(loc[28:]))
	if start+int64(e.csize) > z.cdOff {
		return nil, corruptf("entry %q runs past the data area", name)
	}
	raw := io.NewSectionReader(z.r, start, int64(e.csize))
	var body io.Reader
	switch e.method {
	case 0:
		if e.csize != e.usize {
			return nil, corruptf("stored entry %q has two sizes", name)
		}
		body = raw
	case 8:
		body = flate.NewReader(raw)
	default:
		return nil, corruptf("entry %q uses compression method %d", name, e.method)
	}
	return &entryReader{r: body, remain: e.usize, crc: crc32.NewIEEE(), want: e.crc, name: name}, nil
}

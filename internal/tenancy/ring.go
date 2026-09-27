package tenancy

// ring.go — the resource ring (design §13.1, RT-47). One fixed-size file
// beside the lifecycle ledger, ~/.fathom/daemon/<id>/resources.ring, that
// the governor writes in place: one sample a minute for 24 h plus every
// ladder transition. Readers (`fathom resources`, doctor, the primer) need
// no daemon: the ring is how a stopped, wedged or crash-looping daemon is
// still visible (I38 — never nothing).
//
// Layout: a 160 B header, then RingMinuteSlots minute slots, then
// RingTransitionSlots transition slots, each a 160 B little-endian record
// ending in a CRC-32 of the rest. A minute sample lands in the slot of its
// minute of the day, so the file locates itself with no write cursor and
// never holds more than 24 h of minutes; transitions round-robin through
// their own region (the next slot is found at open), so a flapping ladder
// can never push the minute history out. A slot whose CRC does not match
// (a kill mid-write) is skipped and counted, never read as a zero sample.

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Ring geometry.
const (
	RingMinuteSlots     = 24 * 60
	RingTransitionSlots = 256
	RingRecordSize      = 160
	ringHeaderSize      = RingRecordSize
	// RingFileSize is the ring's fixed size: header plus every slot.
	RingFileSize = ringHeaderSize + (RingMinuteSlots+RingTransitionSlots)*RingRecordSize

	ringVersion   = 1
	ringToolBytes = 32
)

// RingFileName is the ring's name inside a daemon's state directory.
const RingFileName = "resources.ring"

var ringMagic = [8]byte{'F', 'T', 'H', 'M', 'R', 'I', 'N', 'G'}

// RingRecord is one ring slot: a sample, or a ladder transition
// (Transition true, written the moment the ladder moves). Byte fields keep
// Unknown (-1) when the host could not read them.
type RingRecord struct {
	At         time.Time
	Transition bool
	Level      Level
	SafeMode   bool
	LadderOff  bool

	Footprint, Children  int64
	GoTotalMinusReleased int64
	HeapLive             int64
	GCCPUFraction        float64
	Pressure             PressureLevel
	SwapUsed             int64
	StoreFree, TmpFree   int64
	SwapVolumeFree       int64
	DiskWritten          int64
	InFlight             [numClasses]uint16
	OldestTool           string // truncated to 32 B
	OldestClass          Class
	OldestAge            time.Duration // millisecond resolution
	Rejected, Abandoned  uint32        // process-lifetime counts
}

// Combined is footprint + children, the ladder's shed/refuse measure; an
// unknown child reading counts as zero, an unknown footprint is Unknown.
func (r RingRecord) Combined() int64 {
	if r.Footprint < 0 {
		return Unknown
	}
	if r.Children > 0 {
		return r.Footprint + r.Children
	}
	return r.Footprint
}

// InFlightTotal is the sum of in-flight work over every class.
func (r RingRecord) InFlightTotal() int {
	n := 0
	for _, v := range r.InFlight {
		n += int(v)
	}
	return n
}

// Record flags.
const (
	flagTransition = 1 << iota
	flagSafeMode
	flagLadderOff
)

func encodeRecord(buf *[RingRecordSize]byte, r *RingRecord) {
	*buf = [RingRecordSize]byte{}
	le := binary.LittleEndian
	le.PutUint64(buf[0:], uint64(r.At.UnixNano()))
	le.PutUint64(buf[8:], uint64(r.Footprint))
	le.PutUint64(buf[16:], uint64(r.Children))
	le.PutUint64(buf[24:], uint64(r.GoTotalMinusReleased))
	le.PutUint64(buf[32:], uint64(r.HeapLive))
	le.PutUint64(buf[40:], math.Float64bits(r.GCCPUFraction))
	le.PutUint64(buf[48:], uint64(r.SwapUsed))
	le.PutUint64(buf[56:], uint64(r.StoreFree))
	le.PutUint64(buf[64:], uint64(r.TmpFree))
	le.PutUint64(buf[72:], uint64(r.SwapVolumeFree))
	le.PutUint64(buf[80:], uint64(r.DiskWritten))
	for i, v := range r.InFlight {
		le.PutUint16(buf[88+2*i:], v)
	}
	age := r.OldestAge.Milliseconds()
	if age < 0 {
		age = 0
	}
	if age > math.MaxUint32 {
		age = math.MaxUint32
	}
	le.PutUint32(buf[100:], uint32(age))
	le.PutUint32(buf[104:], r.Rejected)
	le.PutUint32(buf[108:], r.Abandoned)
	buf[112] = byte(r.Level)
	buf[113] = byte(r.Pressure)
	buf[114] = byte(r.OldestClass)
	var flags byte
	if r.Transition {
		flags |= flagTransition
	}
	if r.SafeMode {
		flags |= flagSafeMode
	}
	if r.LadderOff {
		flags |= flagLadderOff
	}
	buf[115] = flags
	n := copy(buf[120:120+ringToolBytes], r.OldestTool)
	buf[116] = byte(n)
	le.PutUint16(buf[152:], ringVersion)
	le.PutUint32(buf[156:], crc32.ChecksumIEEE(buf[:156]))
}

// decodeRecord returns the record in buf; ok is false for a slot never
// written (all zero) and torn for one whose checksum does not match.
func decodeRecord(buf []byte) (r RingRecord, ok, torn bool) {
	le := binary.LittleEndian
	if bytes.Equal(buf, make([]byte, RingRecordSize)) {
		return r, false, false
	}
	if crc32.ChecksumIEEE(buf[:156]) != le.Uint32(buf[156:]) || le.Uint16(buf[152:]) != ringVersion {
		return r, false, true
	}
	r.At = time.Unix(0, int64(le.Uint64(buf[0:]))).UTC()
	r.Footprint = int64(le.Uint64(buf[8:]))
	r.Children = int64(le.Uint64(buf[16:]))
	r.GoTotalMinusReleased = int64(le.Uint64(buf[24:]))
	r.HeapLive = int64(le.Uint64(buf[32:]))
	r.GCCPUFraction = math.Float64frombits(le.Uint64(buf[40:]))
	r.SwapUsed = int64(le.Uint64(buf[48:]))
	r.StoreFree = int64(le.Uint64(buf[56:]))
	r.TmpFree = int64(le.Uint64(buf[64:]))
	r.SwapVolumeFree = int64(le.Uint64(buf[72:]))
	r.DiskWritten = int64(le.Uint64(buf[80:]))
	for i := range r.InFlight {
		r.InFlight[i] = le.Uint16(buf[88+2*i:])
	}
	r.OldestAge = time.Duration(le.Uint32(buf[100:])) * time.Millisecond
	r.Rejected = le.Uint32(buf[104:])
	r.Abandoned = le.Uint32(buf[108:])
	r.Level = Level(buf[112])
	r.Pressure = PressureLevel(buf[113])
	r.OldestClass = Class(buf[114])
	flags := buf[115]
	r.Transition = flags&flagTransition != 0
	r.SafeMode = flags&flagSafeMode != 0
	r.LadderOff = flags&flagLadderOff != 0
	n := int(buf[116])
	if n > ringToolBytes {
		n = ringToolBytes
	}
	r.OldestTool = string(buf[120 : 120+n])
	return r, true, false
}

func ringHeader() []byte {
	h := make([]byte, ringHeaderSize)
	copy(h, ringMagic[:])
	le := binary.LittleEndian
	le.PutUint32(h[8:], ringVersion)
	le.PutUint32(h[12:], RingRecordSize)
	le.PutUint32(h[16:], RingMinuteSlots)
	le.PutUint32(h[20:], RingTransitionSlots)
	return h
}

func checkHeader(h []byte) error {
	if len(h) < ringHeaderSize || !bytes.Equal(h[:8], ringMagic[:]) {
		return errors.New("not a fathom resource ring (bad magic)")
	}
	le := binary.LittleEndian
	if v, rs, m, tr := le.Uint32(h[8:]), le.Uint32(h[12:]), le.Uint32(h[16:]), le.Uint32(h[20:]); v != ringVersion || rs != RingRecordSize || m != RingMinuteSlots || tr != RingTransitionSlots {
		return fmt.Errorf("resource ring geometry v%d/%d B/%d+%d slots is not this build's v%d/%d B/%d+%d", v, rs, m, tr, ringVersion, RingRecordSize, RingMinuteSlots, RingTransitionSlots)
	}
	return nil
}

// Ring is an open ring the governor writes. Not safe for concurrent use
// (the governor writes under its own lock).
type Ring struct {
	f         *os.File
	nextTrans int
	buf       [RingRecordSize]byte
}

// OpenRing opens (creating at its fixed size) the ring at path. A file of
// another geometry or not a ring is rewritten as an empty ring: the ring
// is a cache of recent history, and its readers disclose that it is empty.
func OpenRing(path string) (*Ring, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	r := &Ring{f: f}
	hdr := make([]byte, ringHeaderSize)
	fi, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	reinit := fi.Size() != RingFileSize
	if !reinit {
		if _, err := f.ReadAt(hdr, 0); err != nil || checkHeader(hdr) != nil {
			reinit = true
		}
	}
	if reinit {
		if err := f.Truncate(0); err == nil {
			err = f.Truncate(RingFileSize)
		}
		if err == nil {
			_, err = f.WriteAt(ringHeader(), 0)
		}
		if err != nil {
			_ = f.Close()
			return nil, fmt.Errorf("initializing resource ring %s: %w", path, err)
		}
		return r, nil
	}
	// The next transition slot follows the newest one written.
	trans := make([]byte, RingTransitionSlots*RingRecordSize)
	if _, err := f.ReadAt(trans, ringHeaderSize+RingMinuteSlots*RingRecordSize); err != nil && !errors.Is(err, io.EOF) {
		_ = f.Close()
		return nil, err
	}
	var newest time.Time
	for i := 0; i < RingTransitionSlots; i++ {
		rec, ok, _ := decodeRecord(trans[i*RingRecordSize : (i+1)*RingRecordSize])
		if ok && (newest.IsZero() || rec.At.After(newest)) {
			newest, r.nextTrans = rec.At, (i+1)%RingTransitionSlots
		}
	}
	return r, nil
}

// WriteRingSample writes rec in place: a transition into the next
// transition slot, a minute sample into its minute-of-day slot.
func (r *Ring) WriteRingSample(rec RingRecord) error {
	var slot int
	if rec.Transition {
		slot = RingMinuteSlots + r.nextTrans
		r.nextTrans = (r.nextTrans + 1) % RingTransitionSlots
	} else {
		slot = int((rec.At.Unix() / 60) % RingMinuteSlots)
		if slot < 0 {
			slot += RingMinuteSlots
		}
	}
	encodeRecord(&r.buf, &rec)
	_, err := r.f.WriteAt(r.buf[:], int64(ringHeaderSize+slot*RingRecordSize))
	return err
}

// Close closes the ring file.
func (r *Ring) Close() error { return r.f.Close() }

// RingRead is a ring's readable records, in time order, and how many slots
// were torn.
type RingRead struct {
	Records []RingRecord
	Torn    int
}

// ReadRingFile reads the ring at path. A missing file returns the
// not-exist error; a file that is not a ring is a named error (I38).
func ReadRingFile(path string) (RingRead, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return RingRead{}, err
	}
	return ReadRing(raw)
}

// ReadRing decodes a ring's bytes.
func ReadRing(raw []byte) (RingRead, error) {
	if err := checkHeader(raw); err != nil {
		return RingRead{}, fmt.Errorf("resource ring unreadable: %w", err)
	}
	if len(raw) != RingFileSize {
		return RingRead{}, fmt.Errorf("resource ring unreadable: %d B, want %d B", len(raw), RingFileSize)
	}
	var out RingRead
	for off := ringHeaderSize; off+RingRecordSize <= len(raw); off += RingRecordSize {
		rec, ok, torn := decodeRecord(raw[off : off+RingRecordSize])
		if torn {
			out.Torn++
		}
		if ok {
			out.Records = append(out.Records, rec)
		}
	}
	sort.SliceStable(out.Records, func(i, j int) bool {
		a, b := out.Records[i], out.Records[j]
		if !a.At.Equal(b.At) {
			return a.At.Before(b.At)
		}
		return !a.Transition && b.Transition
	})
	return out, nil
}

// Since keeps the records at or after cutoff.
func (rd RingRead) Since(cutoff time.Time) RingRead {
	i := sort.Search(len(rd.Records), func(i int) bool { return !rd.Records[i].At.Before(cutoff) })
	return RingRead{Records: rd.Records[i:], Torn: rd.Torn}
}

// Last is the newest record.
func (rd RingRead) Last() (RingRecord, bool) {
	if len(rd.Records) == 0 {
		return RingRecord{}, false
	}
	return rd.Records[len(rd.Records)-1], true
}

// LastTransition is the newest transition record.
func (rd RingRead) LastTransition() (RingRecord, bool) {
	for i := len(rd.Records) - 1; i >= 0; i-- {
		if rd.Records[i].Transition {
			return rd.Records[i], true
		}
	}
	return RingRecord{}, false
}

// Peak is the record with the largest known footprint + children (the
// earliest on a tie).
func (rd RingRead) Peak() (RingRecord, bool) {
	var best RingRecord
	found := false
	for _, r := range rd.Records {
		c := r.Combined()
		if c < 0 {
			continue
		}
		if !found || c > best.Combined() {
			best, found = r, true
		}
	}
	return best, found
}

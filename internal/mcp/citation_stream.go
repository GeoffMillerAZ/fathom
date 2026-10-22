package mcp

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/geoffmilleraz/fathom/internal/connector"
	"github.com/geoffmilleraz/fathom/internal/fusion"
	"github.com/geoffmilleraz/fathom/internal/semantic"
	"github.com/geoffmilleraz/fathom/internal/tenancy"
)

type citationWindow struct {
	start, end, maxLines, maxBytes int
	data                           []byte
	overflow                       bool
	total                          int
}

func addCitationWindow(ctx context.Context, service, file string, start, end, maxLines, maxBytes int) *citationWindow {
	r, _ := ctx.Value(citationReadsKey{}).(*citationReads)
	if r == nil {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	key := citationKey(service, file)
	f := r.files[key]
	if f == nil {
		f = &citationFileRead{}
		r.files[key] = f
	}
	start = max(start, 1)
	for _, w := range f.windows {
		if w.start == start && w.end == end && w.maxLines == maxLines && w.maxBytes == maxBytes {
			return w
		}
	}
	// All cited windows are registered before the first read of their file.
	if f.read {
		return nil
	}
	w := &citationWindow{start: start, end: end, maxLines: maxLines, maxBytes: maxBytes}
	f.windows = append(f.windows, w)
	return w
}
func prepareObligationWindows(ctx context.Context, service string, rules []semantic.Obligation) {
	for _, r := range rules {
		if r.Status != "" {
			continue
		}
		if file, start, end, ok := parseCitation(r.Src); ok {
			addCitationWindow(ctx, service, file, start, end, end-start+1, obligationSnippetMaxBytes*2)
			if reads, _ := ctx.Value(citationReadsKey{}).(*citationReads); reads != nil {
				reads.mu.Lock()
				reads.files[citationKey(service, file)].indexed = true
				reads.mu.Unlock()
			}
		}
	}
}
func citationSourceRange(ctx context.Context, b QueryInterface, service, file string, start, end, maxLines, maxBytes int) (sourceRange, error) {
	w := addCitationWindow(ctx, service, file, start, end, maxLines, maxBytes)
	status := citationForFile(ctx, b, service, file)
	if w == nil {
		return sourceRange{Citation: status}, nil
	}
	rng, err := w.sourceRange()
	rng.Citation = status
	return rng, err
}

func (w *citationWindow) sourceRange() (sourceRange, error) {
	if w.start > w.total {
		return sourceRange{StartLine: w.start, EndLine: w.start - 1, TotalLines: w.total, ClampedEOF: true, Note: fmt.Sprintf("start_line %d is beyond EOF (file has %d lines)", w.start, w.total)}, nil
	}
	natural := w.total
	if w.end > 0 {
		natural = min(w.end, w.total)
		if natural-w.start+1 > w.maxLines {
			return sourceRange{}, fmt.Errorf("requested range %d-%d (%d lines) exceeds the %d-line cap per call — narrow start_line/end_line or make repeated calls", w.start, natural, natural-w.start+1, w.maxLines)
		}
	}
	rng, err := sliceSourceLines(w.data, 1, 0, w.maxLines, w.maxBytes, false)
	if err != nil {
		return rng, err
	}
	for i := range rng.Lines {
		rng.Lines[i].Line += w.start - 1
	}
	rng.StartLine = w.start
	rng.EndLine += w.start - 1
	rng.TotalLines = w.total
	rng.Requested = natural - w.start + 1
	var notes []string
	rng.ClampedEOF = w.end > w.total
	if rng.ClampedEOF {
		notes = append(notes, fmt.Sprintf("end_line clamped to EOF (file has %d lines)", w.total))
	}
	if w.overflow || rng.Capped {
		notes = append(notes, fmt.Sprintf("truncated at the %d-byte cap", w.maxBytes))
		rng.Capped = true
	} else if rng.Requested > w.maxLines {
		notes = append(notes, fmt.Sprintf("truncated at the %d-line cap", w.maxLines))
		rng.Capped = true
	}
	rng.Note = strings.Join(notes, "; ")
	return rng, nil
}

// citationCapture discards all but the registered, byte-bounded line windows.
// It is a streaming writer, never a copy of the file or a retained line array.
type citationCapture struct {
	windows []*citationWindow
	line    int
	last    byte
	bytes   int64
}

func (c *citationCapture) Write(p []byte) (int, error) {
	n := len(p)
	c.bytes += int64(n)
	maxStop := 0
	for _, w := range c.windows {
		stop := w.start + w.maxLines - 1
		if w.end > 0 {
			stop = min(stop, w.end)
		}
		maxStop = max(maxStop, stop)
	}
	for len(p) > 0 {
		if c.line > maxStop {
			c.line += bytes.Count(p, []byte{'\n'})
			c.last = p[len(p)-1]
			break
		}
		end := bytes.IndexByte(p, '\n')
		part := p
		if end >= 0 {
			part = p[:end+1]
		}
		for _, w := range c.windows {
			stop := w.start + w.maxLines - 1
			if w.end > 0 {
				stop = min(stop, w.end)
			}
			if c.line >= w.start && c.line <= stop {
				room := max(0, w.maxBytes+1-len(w.data))
				take := min(room, len(part))
				w.data = append(w.data, part[:take]...)
				w.overflow = w.overflow || take < len(part)
			}
		}
		c.last = part[len(part)-1]
		p = p[len(part):]
		if end >= 0 {
			c.line++
		}
	}
	return n, nil
}
func (c *citationCapture) finish() {
	total := c.line
	if c.bytes == 0 || c.last == '\n' {
		total--
	}
	for _, w := range c.windows {
		w.total = total
	}
}

type citationMeterReader struct {
	ctx     context.Context
	r       io.Reader
	read    int64
	sniffed int
}

func (r *citationMeterReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := r.r.Read(p)
	r.read += int64(n)
	if e := tenancy.Charge(r.ctx, int64(n)); e != nil {
		return 0, e
	}
	if r.read > sourceReadMaxFileBytes {
		return 0, fmt.Errorf("file grew past the source read ceiling")
	}
	sniff := min(n, 8192-r.sniffed)
	if sniff > 0 && connector.IsBinary(p[:sniff]) {
		return 0, fmt.Errorf("refusing to read binary file")
	}
	r.sniffed += sniff
	return n, err
}
func streamCitationFile(ctx context.Context, b QueryInterface, service, file string, windows []*citationWindow) (string, citationStatus, error) {
	return streamSourceFile(ctx, b, service, file, windows, true)
}
func streamSourceFile(ctx context.Context, b QueryInterface, service, file string, windows []*citationWindow, indexed bool) (string, citationStatus, error) {
	rel, full, err := sourceReadPath(ctx, b, service, file)
	if err != nil {
		return "", citationStatus{}, err
	}
	var before string
	var known bool
	if indexed {
		before, known = indexedSourceFingerprint(ctx, b, service, rel)
	}
	if indexed && !known && len(windows) == 0 {
		return "", citationStatus{State: "unknown", Reason: "indexed source identity is unavailable"}, nil
	}
	epoch, epochKnown := fusion.ExtractorEpoch, false
	if indexed {
		epoch, epochKnown = citationEpoch(ctx, b, service, rel, before)
	}
	if err := tenancy.ProducerAllowsCtx(ctx, tenancy.ClassPoint); err != nil {
		return "", citationStatus{}, err
	}
	f, err := os.Open(full)
	if err != nil {
		return "", citationStatus{}, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", citationStatus{}, err
	}
	if !info.Mode().IsRegular() || info.Size() > sourceReadMaxFileBytes {
		return "", citationStatus{}, fmt.Errorf("source file is not regular or exceeds read ceiling")
	}
	capture := &citationCapture{windows: windows, line: 1}
	reader := &citationMeterReader{ctx: ctx, r: io.LimitReader(f, sourceReadMaxFileBytes+1)}
	var hash string
	stream := io.TeeReader(reader, capture)
	if !indexed {
		_, err = io.Copy(io.Discard, stream)
	} else if epoch == fusion.ExtractorEpoch {
		hash, err = fusion.FingerprintHashReader(stream)
	} else {
		hash, err = fusion.FingerprintHashReaderAtEpoch(stream, epoch)
	}
	if err != nil {
		return "", citationStatus{}, err
	}
	capture.finish()
	if hook, _ := ctx.Value(citationReadHookKey{}).(func()); hook != nil {
		hook()
	}
	if !indexed {
		return "", citationStatus{}, nil
	}
	status := checkedCitation(ctx, b, service, rel, before, known, hash, epochKnown)
	return hash, status, nil
}

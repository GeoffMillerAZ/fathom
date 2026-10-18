package readout

import (
	"regexp"
	"strings"
)

// Stream is an output stream's class (design §6.1). The classifier sets it
// from the command and a closed table in code, never a rule.
type Stream string

// The stream classes.
const (
	// StreamContent is the stdout of a source-file read, grep, rg and git
	// grep matches, ls, and git log, show and diff bodies: error words and
	// path:line prefixes there are data.
	StreamContent Stream = "content"
	// StreamDiagnostic is everything else, stderr included: error, FAIL,
	// panic, fatal and warning lines and file:line frames are protected.
	StreamDiagnostic Stream = "diagnostic"
)

// lineClass is what a line carries, for protection and the receipt's
// dropped-by-class counts.
type lineClass int

const (
	classPlain  lineClass = iota
	classFrame            // a file:line frame or a path:line: prefix
	classError            // error, FAIL, panic, fatal, warning
	classNotice           // the harness's truncation or persisted-output notice
	classExit             // Exit code N
	classCount
)

// classOrder is the receipt's fixed order, most protected first.
var classOrder = []lineClass{classExit, classNotice, classError, classFrame, classPlain}

func (c lineClass) String() string {
	switch c {
	case classFrame:
		return "frame"
	case classError:
		return "error"
	case classNotice:
		return "notice"
	case classExit:
		return "exit"
	}
	return "plain"
}

var (
	exitLine   = regexp.MustCompile(`^Exit code -?\d+\s*$`)
	noticeLine = regexp.MustCompile(`(?i)\d+\s+(lines?|chars?|characters|bytes)\s+truncated|<persisted-output>|output too large|persisted output|full output (saved|written) to`)
	errorWord  = regexp.MustCompile(`(?i)\b(errors?|fail(s|ed|ure|ures)?|panic(s|ked)?|fatal|warn(ing|ings)?)\b`)
	frameRef   = regexp.MustCompile(`^[^\s:]+:\d+[:\s]|(^|[\s(\["'])[^\s:()\[\]"']*\.[A-Za-z0-9]+:\d+|File "[^"]+", line \d+`)
)

// classify is the strongest class a display line carries.
func classify(text string) lineClass {
	switch {
	case exitLine.MatchString(text):
		return classExit
	case noticeLine.MatchString(text):
		return classNotice
	case errorWord.MatchString(text):
		return classError
	case frameRef.MatchString(text):
		return classFrame
	}
	return classPlain
}

// protected reports whether a line of class c may not drop in stream s.
func protected(c lineClass, s Stream) bool {
	switch c {
	case classExit, classNotice:
		return true
	case classError, classFrame:
		return s != StreamContent
	}
	return false
}

var ansiSeq = regexp.MustCompile("\x1b\\[[0-?]*[ -/]*[@-~]|\x1b\\][^\x07\x1b]*(?:\x07|\x1b\\\\)|\x1b[@-Z\\\\-_]")

// renderTerminal is the text a terminal shows for one line: escape
// sequences removed and carriage-return overwrites applied.
func renderTerminal(s string) string {
	if strings.IndexByte(s, 0x1b) >= 0 {
		s = ansiSeq.ReplaceAllString(s, "")
	}
	if strings.IndexByte(s, '\r') < 0 {
		return s
	}
	segs := strings.Split(s, "\r")
	buf := []rune(segs[0])
	for _, seg := range segs[1:] {
		r := []rune(seg)
		if len(r) >= len(buf) {
			buf = r
			continue
		}
		copy(buf, r)
	}
	return string(buf)
}

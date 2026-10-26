package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"unicode/utf8"
)

// maxLabel caps every free-form label written to the ledger (model names,
// agent types, efforts). Host values are not trusted to be short.
const maxLabel = 128

// label caps a host-supplied value for the ledger.
func label(s string) string {
	if utf8.RuneCountInString(s) <= maxLabel {
		return s
	}
	return string([]rune(s)[:maxLabel])
}

// eachLine calls fn for every line of r. A line longer than maxLine is
// skipped whole, so one oversized or torn record cannot hide the lines after
// it. It returns the first read error other than io.EOF; fn has already seen
// every complete line before it.
func eachLine(r io.Reader, maxLine int, fn func([]byte)) error {
	br := bufio.NewReaderSize(r, 64<<10)
	var buf []byte
	skipping := false
	for {
		chunk, isPrefix, err := br.ReadLine()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		if !skipping {
			if len(buf)+len(chunk) > maxLine {
				skipping, buf = true, buf[:0]
			} else {
				buf = append(buf, chunk...)
			}
		}
		if !isPrefix {
			if !skipping {
				fn(buf)
			}
			buf, skipping = buf[:0], false
		}
	}
}

// shellWords splits a command line into words the way a POSIX shell would
// for labelling purposes: single quotes, double quotes and backslashes group
// characters into one word, and unquoted whitespace separates words. It stops
// after limit words (0 means no limit). It is not a shell parser: it never
// expands anything, and its only job is to never split a quoted value.
func shellWords(command string, limit int) []string {
	var words []string
	var word strings.Builder
	inWord, inSingle, inDouble, escaped := false, false, false, false
	flush := func() bool {
		if inWord {
			words = append(words, word.String())
			word.Reset()
			inWord = false
		}
		return limit > 0 && len(words) >= limit
	}
	for _, r := range command {
		switch {
		case escaped:
			word.WriteRune(r)
			escaped, inWord = false, true
		case inSingle:
			if r == '\'' {
				inSingle = false
			} else {
				word.WriteRune(r)
			}
		case r == '\\':
			escaped, inWord = true, true
		case inDouble:
			if r == '"' {
				inDouble = false
			} else {
				word.WriteRune(r)
			}
		case r == '\'':
			inSingle, inWord = true, true
		case r == '"':
			inDouble, inWord = true, true
		case r == ' ' || r == '\t' || r == '\n':
			if flush() {
				return words
			}
		default:
			word.WriteRune(r)
			inWord = true
		}
	}
	flush()
	return words
}

// rawObject decodes a JSON object into its raw members, ignoring nothing
// and typing nothing, so a host that changes the type of a field this binary
// does not use cannot make the whole payload unreadable.
func rawObject(raw []byte) (map[string]json.RawMessage, error) {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	if m == nil {
		return nil, errors.New("not a JSON object")
	}
	return m, nil
}

// rawString returns m[key] as a string, or "" when absent or not a string.
func rawString(m map[string]json.RawMessage, key string) string {
	var s string
	if json.Unmarshal(m[key], &s) != nil {
		return ""
	}
	return s
}

// marshalNoEscape encodes v without HTML escaping, so "&&" in a hook command
// keeps its bytes when a configuration file is rewritten.
func marshalNoEscape(v any) ([]byte, error) {
	var b strings.Builder
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return []byte(strings.TrimSuffix(b.String(), "\n")), nil
}

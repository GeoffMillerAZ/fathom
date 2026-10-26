package main

import (
	"fmt"
	"strings"
)

// onlyScopeNote is the shared wire format for a reading's -only disclosure.
func onlyScopeNote(name string, globs []string) string {
	return fmt.Sprintf("%s: restricted to %s (-only; the files outside are a disclosed gap, not measured)", name, strings.Join(globs, ", "))
}

func parseOnlyScopeNote(name, note string) ([]string, bool) {
	prefix, suffix, _ := strings.Cut(onlyScopeNote(name, []string{"\x00"}), "\x00")
	if !strings.HasPrefix(note, prefix) || !strings.HasSuffix(note, suffix) {
		return nil, false
	}
	scope := strings.TrimSuffix(strings.TrimPrefix(note, prefix), suffix)
	if scope == "" {
		return nil, false
	}
	return strings.Split(scope, ", "), true
}

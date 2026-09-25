package terminal_test

import (
	"regexp"
	"testing"
)

// Pinned 120/121-column Ascii fixture: full row or the renderer's exact fourth-row
// update. A partial typing match, editing caret or another row is not ready.
var finalFilterRow = regexp.MustCompile(`(?:^|\r?\n|\x1b\[4;1H)Filter: forms validation *(?:$|\r?\n|\x1b\[K)`)

func terminalFilterFrameReady(raw []byte) bool { return finalFilterRow.Match(raw) }

func TestTerminalFilterFrameReadiness(t *testing.T) {
	for _, raw := range []string{
		"Filter: forms validation \r\nSelected: interaction.forms",
		"header\r\nFilter: forms validation\r\n",
		"\x1b[4;1HFilter: forms validation ",
		"\x1b[4;1HFilter: forms validation",
		"\x1b[4;1HFilter: forms validation\x1b[K",
	} {
		if !terminalFilterFrameReady([]byte(raw)) {
			t.Errorf("final filter row refused: %q", raw)
		}
	}
	for _, raw := range []string{
		"Selected: interaction.forms",
		"> Filter: forms validation_\r\nSelected: interaction.forms",
		"\x1b[4;1H> Filter: forms validation_",
		"\x1b[4;1HFilter: forms val ",
		"\x1b[5;1HFilter: forms validation ",
		"Filter: forms validation_extra ",
		"Filter: forms validation extra",
		"\x1b[4;1HFilter: forms validation   extra\r\n",
	} {
		if terminalFilterFrameReady([]byte(raw)) {
			t.Errorf("intermediate/unrelated frame accepted: %q", raw)
		}
	}
}

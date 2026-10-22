package main

import (
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/geoffmilleraz/fathom/internal/statedir"
)

// printInitUserWritesPreview separates preserved user configuration from
// operational state, which still uses the configured state directory.
func printInitUserWritesPreview(stdout io.Writer, root, cause string, writes *initWrites, realRun bool) {
	fmt.Fprintf(stdout, "user configuration writes disabled (%s):\n", cause)
	fmt.Fprintf(stdout, "operational state directory: %s (still written during setup; relocate with FATHOM_STATE_DIR or XDG_STATE_HOME)\n", statedir.Resolve(root))
	// The dry run prints its complete write list once.
	if !realRun {
		return
	}
	if len(writes.touched) == 0 {
		fmt.Fprintln(stdout, "would write or change no user file")
	}
	home, _ := os.UserHomeDir()
	sort.Slice(writes.touched, func(i, j int) bool { return writes.touched[i].Path < writes.touched[j].Path })
	for _, touch := range writes.touched {
		fmt.Fprintf(stdout, "skipped-user-scope %s (would %s)\n", displayPath(touch.Path, root, home), touch.Action)
	}
}

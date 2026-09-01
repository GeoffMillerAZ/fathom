// Package output is the cli-kit's terminal output surface: color-capability
// detection, a status table renderer byte-compatible with
// platform-engineering's bosun doctor (the migration proof), and an NDJSON
// row writer for the machine-readable path every kit "doctor"-style command
// needs alongside its human one.
package output

import (
	"io"

	"github.com/charmbracelet/colorprofile"
)

// Mode selects a command's output shape.
type Mode string

const (
	// ModeHuman renders RenderStatusTable's aligned, human-readable table.
	ModeHuman Mode = "human"
	// ModeJSON is reserved for a future single-document JSON shape —
	// distinct from ModeNDJSON's one-object-per-line stream. Not emitted
	// by anything in this package yet; named now so a consumer's --json
	// flag has a stable value to switch on before the shape exists.
	ModeJSON Mode = "json"
	// ModeNDJSON renders WriteNDJSON's newline-delimited JSON rows.
	ModeNDJSON Mode = "ndjson"
)

// Profile is the detected color capability of an output stream. It is
// colorprofile.Profile itself (a type alias, not a wrapper) so a caller
// that already imports colorprofile for something else never juggles two
// types for the same value.
type Profile = colorprofile.Profile

// Detect reports w's color profile from its terminal state and the
// process environment. NO_COLOR / CLICOLOR_FORCE / CLICOLOR / COLORTERM /
// TERM semantics all come from colorprofile — this package never
// reimplements them (see colorprofile's env.go for the precedence rules).
func Detect(w io.Writer, environ []string) Profile {
	return colorprofile.Detect(w, environ)
}

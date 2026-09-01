package output

import (
	"fmt"
	"io"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/colorprofile"

	"github.com/geoffmilleraz/app-kit/appkit/cli/theme"
)

// StatusRow is one diagnostic row — the shape bosun doctor's roster
// produces (platform-engineering's kits/env-kit/internal/doctor.Row) and
// the shape any cli-kit "doctor"-style command should reuse rather than
// inventing a second one.
type StatusRow struct {
	ID     string
	Status string // rendered upper-cased: OK, FAIL, UNKNOWN, or any caller value
	Detail string
	Remedy string
}

// RenderStatusTable writes rows in doctor's fixed layout.
//
// Ascii and NoTTY profiles render BYTE-IDENTICAL to
// platform-engineering's kits/env-kit/internal/doctor.Render — that exact
// byte-compatibility is the bosun-onto-cli-kit migration's proof (build
// spec S3), so this geometry is not free to adjust: one status column
// (7 wide, upper-cased), one space, the id column (padded to the widest id
// in rows), two spaces, the detail — then, if any row failed, a blank line
// followed by "N check(s) failed. Remedies:" and a "  <id>\n    <remedy>"
// block per failure; if none failed but some are unknown, a
// "N check(s) could not be determined." line; otherwise "healthy".
//
// Every other profile (ANSI, ANSI256, TrueColor) renders the identical
// geometry with color added to two things only: the status word (OK =
// positive, FAIL = danger, UNKNOWN = caution) and each remedy line
// (accent). ID and detail text stay plain in both paths — coloring them
// would not add information, only inconsistency between the two render
// paths' column widths if the color codes were ever allowed near a %*s
// pad (they are not: colored substrings are built at their exact plain
// width and inserted, never passed through a width verb themselves).
//
// Styled color always resolves against theme.Default's DARK palette.
// Distinguishing an actually-light terminal needs a real background-color
// query (lipgloss.HasDarkBackground over the live TTY, or
// tea.BackgroundColorMsg inside a bubbletea program) that a one-shot
// Detect(w, environ) profile call cannot provide; dark is the common
// terminal default and the deliberate simplification here — DIVERGENCE
// from full polarity awareness, tracked for a future Option if a consumer
// needs it.
func RenderStatusTable(w io.Writer, rows []StatusRow, profile Profile) {
	idWidth := 0
	for _, r := range rows {
		if len(r.ID) > idWidth {
			idWidth = len(r.ID)
		}
	}
	styled := profile > colorprofile.Ascii

	failed, unknown := 0, 0
	for _, r := range rows {
		status := strings.ToUpper(r.Status)
		statusField := padRight(status, 7)
		if styled {
			statusField = padRight(statusStyle(status).Render(status), 7, len(status))
		}
		fmt.Fprintf(w, "%s %s  %s\n", statusField, padRight(r.ID, idWidth), r.Detail)
		switch status {
		case "FAIL":
			failed++
		case "UNKNOWN":
			unknown++
		}
	}

	switch {
	case failed > 0:
		fmt.Fprintf(w, "\n%d check%s failed. Remedies:\n", failed, plural(failed))
		for _, r := range rows {
			if strings.ToUpper(r.Status) != "FAIL" {
				continue
			}
			id, remedy := r.ID, r.Remedy
			if styled {
				remedy = theme.Styles(true).Accent.Render(remedy)
			}
			fmt.Fprintf(w, "  %s\n    %s\n", id, remedy)
		}
	case unknown > 0:
		fmt.Fprintf(w, "\n%d check%s could not be determined.\n", unknown, plural(unknown))
	default:
		fmt.Fprintln(w, "\nhealthy")
	}
}

// padRight pads s with spaces to width. An optional visibleLen overrides
// the length padRight measures against (rune count of s otherwise) — used
// when s already carries ANSI escapes whose byte length must not count
// toward the visual column width.
func padRight(s string, width int, visibleLen ...int) string {
	n := len([]rune(s))
	if len(visibleLen) > 0 {
		n = visibleLen[0]
	}
	if pad := width - n; pad > 0 {
		return s + strings.Repeat(" ", pad)
	}
	return s
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// statusStyle picks the role a status word renders in: OK = positive,
// FAIL = danger, UNKNOWN = caution, anything else falls back to Muted so
// an unrecognized status string never renders unstyled-but-silently-wrong.
func statusStyle(status string) lipgloss.Style {
	st := theme.Styles(true)
	switch status {
	case "OK":
		return st.Good
	case "FAIL":
		return st.Error
	case "UNKNOWN":
		return st.Warn
	default:
		return st.Muted
	}
}

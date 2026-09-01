// Package theme is the terminal counterpart to appkit/ui/tokens: it
// resolves the kit's 23 CUE color roles to concrete sRGB colors for cobra
// CLIs, so a terminal command gets the same roles a web app gets — just
// resolved through OKLCHToHex instead of a CSS custom property.
//
// palette_gen.go is GENERATED (appkit gen, from cue/tokens/kit.cue via
// tokens.EmitTerminalPalette) — never hand-edit it. This file is the hand
// side: Palette, the semantic Styles set, and the fang.ColorSchemeFunc
// bridge.
//
// Deliberately excluded for now: no bubbles, no huh, no glamour, no log —
// each is added only when a consumer needs it, one at a time, so it earns
// its place in go.sum rather than arriving as part of "the terminal
// stack." See ADR-0018 for the promotion bar this pays.
package theme

import (
	"image/color"

	"charm.land/fang/v2"
	"charm.land/lipgloss/v2"
)

// Palette holds every color role resolved to a concrete color, for both
// background polarities. Built once from the generated LightHex/DarkHex
// tables and never mutated.
type Palette struct {
	Light map[Role]color.Color
	Dark  map[Role]color.Color
}

// Default is the kit's palette, built from the generated hex tables.
var Default = newPalette()

func newPalette() Palette {
	p := Palette{
		Light: make(map[Role]color.Color, len(Roles)),
		Dark:  make(map[Role]color.Color, len(Roles)),
	}
	for _, r := range Roles {
		p.Light[r] = lipgloss.Color(LightHex[r])
		p.Dark[r] = lipgloss.Color(DarkHex[r])
	}
	return p
}

func resolve(r Role, isDark bool) color.Color {
	if isDark {
		return Default.Dark[r]
	}
	return Default.Light[r]
}

// StyleSet is a small semantic style vocabulary, resolved for one
// background polarity — the everyday styles a cli-kit command reaches for
// (a heading, a de-emphasized line, a failure, a warning, a success, an
// accent word, a muted footnote) without hand-picking a role each time.
type StyleSet struct {
	Title  lipgloss.Style
	Subtle lipgloss.Style
	Error  lipgloss.Style
	Warn   lipgloss.Style
	Good   lipgloss.Style
	Accent lipgloss.Style
	Muted  lipgloss.Style
}

// Styles returns the kit's semantic style set for one background polarity.
// A command that already knows its terminal's polarity (from
// tea.BackgroundColorMsg, or a flag) calls this directly; one that only
// has a lipgloss.LightDarkFunc (fang's callback shape) should build the
// two styles itself with FangColorScheme's pattern instead.
func Styles(isDark bool) StyleSet {
	return StyleSet{
		Title:  lipgloss.NewStyle().Bold(true).Foreground(resolve(RoleInk, isDark)),
		Subtle: lipgloss.NewStyle().Foreground(resolve(RoleInkMuted, isDark)),
		Error:  lipgloss.NewStyle().Bold(true).Foreground(resolve(RoleDanger, isDark)),
		Warn:   lipgloss.NewStyle().Foreground(resolve(RoleCaution, isDark)),
		Good:   lipgloss.NewStyle().Foreground(resolve(RolePositive, isDark)),
		Accent: lipgloss.NewStyle().Foreground(resolve(RoleAccent, isDark)),
		Muted:  lipgloss.NewStyle().Foreground(resolve(RoleInkFaint, isDark)),
	}
}

// FangColorScheme maps the kit's roles onto fang's ColorScheme, so
// `<any cli-kit command> --help` inherits the same palette as every other
// kit surface — accent is accent everywhere, not a second, hand-tuned CLI
// palette that quietly drifts from the web one. Pass the result to
// command.Execute (which does this by default) or fang.WithColorSchemeFunc
// directly.
//
// The mapping is fixed, not configurable — a second free-standing terminal
// palette is exactly the drift this exists to prevent:
//
//	fang field       kit role
//	Base             ink
//	Title            accent
//	Description      ink-muted
//	Program          accent
//	Command          accent
//	Flag             info
//	FlagDefault      ink-faint
//	Comment          ink-muted
//	QuotedString     positive
//	Argument         ink
//	DimmedArgument   ink-faint
//	Codeblock        surface-raised
//	Help             ink-muted
//	Dash             line-strong
//	ErrorHeader      [danger (fg), ink-inverse (bg)]
//	ErrorDetails     danger
func FangColorScheme() fang.ColorSchemeFunc {
	return func(ld lipgloss.LightDarkFunc) fang.ColorScheme {
		c := func(r Role) color.Color { return ld(Default.Light[r], Default.Dark[r]) }
		return fang.ColorScheme{
			Base:           c(RoleInk),
			Title:          c(RoleAccent),
			Description:    c(RoleInkMuted),
			Codeblock:      c(RoleSurfaceRaised),
			Program:        c(RoleAccent),
			DimmedArgument: c(RoleInkFaint),
			Comment:        c(RoleInkMuted),
			Flag:           c(RoleInfo),
			FlagDefault:    c(RoleInkFaint),
			Command:        c(RoleAccent),
			QuotedString:   c(RolePositive),
			Argument:       c(RoleInk),
			Help:           c(RoleInkMuted),
			Dash:           c(RoleLineStrong),
			ErrorHeader:    [2]color.Color{c(RoleDanger), c(RoleInkInverse)},
			ErrorDetails:   c(RoleDanger),
		}
	}
}

// Package terminal is an experimental, read-only terminal Gallery over the shared
// first-party catalog. It is an application example, not a reusable widget API.
package terminal

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/rivo/uniseg"

	"github.com/geoffmilleraz/app-kit/appkit/catalog"
)

const maxFilterBytes = 256

type Options struct {
	Mouse          bool
	Query, Profile string
}

// Model owns interaction only. Usage/source/proof strings are displayed, never run.
type Model struct {
	kit                   *catalog.Catalog
	page                  catalog.Page
	query, profile        string
	width, height         int
	selected, offset      int
	detail                *catalog.Entry
	detailOffset          int
	editing, mouse        bool
	isDark, polarityKnown bool
	notice                string
	epoch                 uint64
}

func New(c *catalog.Catalog, opts Options) (*Model, error) {
	m := &Model{kit: c, width: 80, height: 24, query: opts.Query, profile: opts.Profile, mouse: opts.Mouse}
	if len(opts.Query) > maxFilterBytes || !utf8.ValidString(opts.Query) || clean(opts.Query) != opts.Query {
		return nil, fmt.Errorf("terminal catalog: filter must be printable UTF-8, at most %d bytes", maxFilterBytes)
	}
	if err := m.search(); err != nil {
		return nil, err
	}
	return m, nil
}

func (m *Model) Init() tea.Cmd { return tea.RequestBackgroundColor }

func (m *Model) search() error {
	page, err := m.kit.Search(catalog.Query{Text: m.query, Profile: m.profile, Limit: 50})
	if err != nil {
		return err
	}
	m.page, m.selected, m.offset = page, 0, 0
	m.epoch++
	return nil
}

func (m *Model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = max(1, min(msg.Width, 300)), max(1, min(msg.Height, 120))
		m.epoch++
		m.clamp()
	case tea.BackgroundColorMsg:
		m.isDark, m.polarityKnown = msg.IsDark(), true
	case mouseAction:
		if m.mouse && msg.epoch == m.epoch {
			return m, m.action(msg.name, msg.id, msg.delta)
		}
	case tea.PasteMsg:
		if m.editing && m.detail == nil {
			m.appendFilter(msg.Content)
		}
	case tea.KeyPressMsg:
		if msg.Code == 'c' && msg.Mod&tea.ModCtrl != 0 {
			return m, tea.Quit
		}
		if msg.Code == tea.KeyF2 {
			return m, m.action("mouse", "", 0)
		}
		if m.editing {
			switch {
			case msg.Code == tea.KeyEscape || msg.Code == tea.KeyEnter || msg.Code == tea.KeyTab || msg.Code == tea.KeyDown:
				m.editing = false
			case msg.Code == 'u' && msg.Mod&tea.ModCtrl != 0:
				return m, m.action("clear", "", 0)
			case msg.Code == tea.KeyBackspace:
				start := 0
				graphemes := uniseg.NewGraphemes(m.query)
				for graphemes.Next() {
					start, _ = graphemes.Positions()
				}
				m.setFilter(m.query[:start])
			case msg.Text != "" && msg.Mod&(tea.ModCtrl|tea.ModAlt|tea.ModMeta) == 0:
				m.appendFilter(msg.Text)
			}
			return m, nil
		}
		switch msg.Code {
		case tea.KeyEscape:
			return m, m.action("back", "", 0)
		case tea.KeyEnter:
			return m, m.action("open", m.selectedID(), 0)
		case tea.KeyUp:
			return m, m.action("move", "", -1)
		case tea.KeyDown:
			return m, m.action("move", "", 1)
		case tea.KeyPgUp:
			return m, m.action("move", "", -m.bodyRows())
		case tea.KeyPgDown:
			return m, m.action("move", "", m.bodyRows())
		case tea.KeyHome:
			return m, m.action("move", "", -100000)
		case tea.KeyEnd:
			return m, m.action("move", "", 100000)
		case '/':
			return m, m.action("filter", "", 0)
		case 'u':
			if msg.Mod&tea.ModCtrl != 0 {
				return m, m.action("clear", "", 0)
			}
		case 'q':
			// Lock-state reports do not turn a plain key into a shortcut, but
			// application/window modifiers must not accidentally close the TUI.
			if msg.Mod & ^(tea.ModCapsLock|tea.ModNumLock|tea.ModScrollLock) == 0 {
				return m, tea.Quit
			}
		}
	}
	return m, nil
}

func (m *Model) selectedID() string {
	if m.selected >= 0 && m.selected < len(m.page.Items) {
		return m.page.Items[m.selected].ID
	}
	return ""
}

func (m *Model) action(name, id string, delta int) tea.Cmd {
	switch name {
	case "quit":
		return tea.Quit
	case "mouse":
		m.mouse = !m.mouse
		m.epoch++
	case "back":
		m.detail, m.editing = nil, false
		m.epoch++
	case "filter":
		m.detail, m.editing = nil, true
		m.epoch++
	case "clear":
		m.detail = nil
		m.setFilter("")
	case "select", "open":
		if m.detail != nil {
			return nil
		}
		for i, item := range m.page.Items {
			if item.ID == id {
				m.selected, m.editing = i, false
				if name == "open" {
					entry, ok := m.kit.Get(id)
					if ok {
						m.detail, m.detailOffset = &entry, 0
						m.epoch++
					}
				}
				break
			}
		}
	case "move":
		if m.detail != nil {
			m.detailOffset += delta
		} else {
			m.selected += delta
		}
	}
	m.clamp()
	return nil
}

func (m *Model) bodyRows() int { return max(1, m.height-8) }

func (m *Model) clamp() {
	m.selected = max(0, min(m.selected, len(m.page.Items)-1))
	oldOffset := m.offset
	m.offset = max(0, min(m.offset, m.selected))
	if m.selected >= m.offset+m.bodyRows() {
		m.offset = m.selected - m.bodyRows() + 1
	}
	if m.offset != oldOffset {
		m.epoch++
	}
	if m.detail != nil {
		m.detailOffset = max(0, min(m.detailOffset, len(m.detailLines())-m.bodyRows()))
	}
}

func (m *Model) setFilter(value string) {
	prior := m.query
	m.query, m.notice = value, ""
	if err := m.search(); err != nil {
		m.query = prior
		m.notice = "Filter rejected; previous results retained."
	}
}

func (m *Model) appendFilter(value string) {
	var b strings.Builder
	size, changed := len(m.query), !utf8.ValidString(value)
	for position, r := range value {
		if position >= 4096 {
			changed = true
			break
		}
		if r == '\n' || r == '\r' || r == '\t' {
			r = ' '
			changed = true
		}
		if forbidden(r) {
			changed = true
			continue
		}
		if size+utf8.RuneLen(r) > maxFilterBytes {
			changed = true
			break
		}
		b.WriteRune(r)
		size += utf8.RuneLen(r)
	}
	m.setFilter(m.query + b.String())
	if changed {
		m.notice = "Input sanitized or filter limit reached (256 bytes)."
	}
}

func forbidden(r rune) bool {
	return unicode.IsControl(r) || unicode.Is(unicode.Bidi_Control, r)
}

func clean(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' {
			return ' '
		}
		if forbidden(r) {
			return -1
		}
		return r
	}, s)
}

type mouseAction struct {
	name, id string
	delta    int
	epoch    uint64
}

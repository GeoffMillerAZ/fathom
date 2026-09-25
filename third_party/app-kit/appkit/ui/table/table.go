// Package table provides a generic token-driven data table for kit UIs.
//
// # Data model
//
// A Table takes typed Column definitions and a slice of Row values. Each Row
// contains ordered cell strings that must align with the Column slice (same
// length). Mismatched lengths are tolerated: extra cells are dropped, missing
// cells render empty.
//
// # Sortable headers
//
// Columns with Sortable=true render a clickable <th> button. The package
// emits a data-table-sort="{col.Key}" attribute on the button so the consuming
// app can wire a Datastar handler (see interaction contract in metadata.cue).
// Active sort direction is signalled by SortKey + SortDir on TableParams.
//
// # Row actions / selection
//
// An optional RowAction slot (templ.Component) is rendered in a trailing cell
// for every row. Without a slot, managed row links infer that column; RowHrefs
// can explicitly reserve it for empty or currently unlinked results.
//
// # Interaction contract (Datastar / hypermedia)
//
// This package is PRESENTATIONAL. The consuming app owns all endpoints.
//
// Emitted hooks (consuming app responsibilities in []):
//
//	data-table="{tableID}"           on the root <table> element (if TableID is set)
//	data-table-sort="{col.Key}"      on sortable column header buttons
//	data-table-row="{row.ID}"        on each <tr> element
//	data-table-select="{row.ID}"     on the checkbox input when Selectable=true
//
//	data-on-click="..."              [app]: POST to /your/sort endpoint with
//	                                       {tableID, sortKey, sortDir}; SSE stream
//	                                       delivers re-rendered table
//	data-on-change="..."             [app]: POST to /your/select endpoint with
//	                                       {tableID, rowID, checked}; SSE delivers
//	                                       updated selection state
//
// # Theming
//
// All colors reference kit token roles. No hex values appear in this package.
package table

import (
	"fmt"

	"github.com/a-h/templ"
)

// Column defines one column in the table.
type Column struct {
	// Key is the column identifier, emitted as data-table-sort when Sortable.
	Key string
	// Label is the user-visible column header text.
	Label string
	// Sortable marks the column header as a sort trigger.
	Sortable bool
	// Align controls text alignment: "left" (default), "center", "right".
	Align string
	// HeaderHTML optionally supplies rich header content (e.g. a select-all
	// checkbox, an info tooltip) rendered in place of the Label text. When set,
	// Label is ignored for display but should still describe the column for
	// callers. Sortable is ignored when HeaderHTML is set. Callers own safety.
	HeaderHTML templ.Component
}

// Row is one data row in the table.
type Row struct {
	// ID is the row identifier, emitted as data-table-row on the <tr>.
	ID string
	// Cells are the cell values in Column order. Length mismatch is tolerated.
	// Plain text; escaped on render.
	Cells []string
	// HTMLCells optionally supplies rich cell content (badges, links, viz) in
	// Column order. When HTMLCells[i] is non-nil it renders instead of the
	// escaped Cells[i] string — so a table can mix plain and rich cells. Callers
	// own the safety of any component they pass.
	HTMLCells []templ.Component
	// Href is an optional relative web reference or absolute HTTP(S) URL. A safe
	// shape renders a native Open link; unsafe nonempty input renders an inert
	// Open unavailable label. Apps still own origins, routing and authorization.
	// Any nonempty Href infers a shared trailing action column. Unlinked rows
	// reserve an empty cell in that column without inventing an action. Enabled
	// links need rebuilt scanned CSS for minimum target size and focus treatment.
	Href string
	// Activate marks the row as a keyboard-activatable target: it renders with
	// role="button", tabindex="0", and a pointer cursor + focus ring, for apps
	// that open a drawer or detail panel on row activation instead of
	// navigating. Pair it with Attrs (the client-side interaction hook a JS
	// island reads) and Label (the accessible name). It is independent of Href;
	// use Activate for in-page interactions and Href for navigation.
	Activate bool
	// Label is the row's accessible name, emitted as aria-label on the <tr>.
	// Use it with Activate so assistive tech announces what activating the row
	// does. Ignored when empty.
	Label string
	// Attrs are app-defined attributes emitted verbatim on the <tr> — the
	// interaction hooks a JS island / Datastar handler reads (e.g. a
	// data-prov-url the app fetches on activation). templ renders the keys in
	// sorted order, so output stays deterministic. Callers own their safety.
	Attrs templ.Attributes
	// Muted dims the row (reduced opacity) to mark it as inactive, disabled,
	// archived, or voided while keeping it readable and in place. The row's
	// content and semantics are unchanged; only its visual weight drops.
	Muted bool
}

// FooterCell is one cell in a footer summary row.
type FooterCell struct {
	// Text is the plain cell text; escaped on render. Ignored when HTML is set.
	Text string
	// HTML optionally supplies rich footer content (e.g. a labelled total).
	// When non-nil it renders instead of the escaped Text. Callers own safety.
	HTML templ.Component
	// Colspan spans this cell across multiple columns (default 1). Authors are
	// responsible for making each footer row's colspans sum to the table's
	// total column count, including any leading select / trailing action column.
	Colspan int
	// Align controls text alignment: "left" (default), "center", "right".
	Align string
	// Emphasis renders the cell bold — use for subtotal / total labels + values.
	Emphasis bool
}

// FooterRow is a summary row rendered inside <tfoot> (subtotals, totals).
type FooterRow struct {
	// Cells are the footer cells, left to right.
	Cells []FooterCell
}

// htmlCell returns the rich cell component for column i, or nil to fall back to
// the escaped string cell.
func htmlCell(row Row, i int) templ.Component {
	if i < len(row.HTMLCells) {
		return row.HTMLCells[i]
	}
	return nil
}

// TableParams configures a Table render.
type TableParams struct {
	// TableID is an optional identifier for the table element (data-table attr).
	TableID string
	// Columns defines the table columns.
	Columns []Column
	// Rows is the row data.
	Rows []Row
	// SortKey is the currently active sort column Key (empty = no active sort).
	SortKey string
	// SortDir is the current sort direction: "asc" or "desc".
	SortDir string
	// Selectable adds a leading checkbox column when true.
	Selectable bool
	// RowAction is an optional trailing-cell slot rendered for every row.
	// It takes precedence over Href. Nil does not suppress a column required by
	// actual row links or explicitly reserved by RowHrefs.
	RowAction templ.Component
	// Caption is an optional accessible <caption> element.
	Caption string
	// RowHrefs explicitly reserves a trailing action column, even when Rows is
	// empty or all Hrefs are empty. Nonempty Hrefs or a RowAction slot infer the
	// same column without this flag. Every body row reserves the matching cell.
	RowHrefs bool
	// Footer is an optional set of summary rows rendered in <tfoot> (subtotals,
	// totals). Empty omits the footer entirely.
	Footer []FooterRow
}

// withActionColumn derives the shared shape once, in linear time and constant
// space. TableParams is copied; caller parameters, rows and slots are unchanged.
func withActionColumn(p TableParams) TableParams {
	if p.RowHrefs || p.RowAction != nil {
		return p
	}
	for _, row := range p.Rows {
		if row.Href != "" {
			p.RowHrefs = true
			break
		}
	}
	return p
}

// thClass returns the Tailwind class string for a <th> element.
func thClass(col Column) string {
	base := "px-4 py-2"
	switch col.Align {
	case "center":
		base += " text-center"
	case "right":
		base += " text-right"
	default:
		base += " text-left"
	}
	return base
}

// tdClass returns the Tailwind class string for a <td> element.
func tdClass(col Column) string {
	base := "px-4 py-2 text-ink"
	switch col.Align {
	case "center":
		base += " text-center"
	case "right":
		base += " text-right"
	default:
		// left is the default; no extra class needed
	}
	return base
}

// trClass returns the Tailwind class string for a <tr> element.
// Alternating rows use surface-sunken for subtle striping.
func trClass(idx int) string {
	if idx%2 == 1 {
		return "bg-surface-sunken hover:bg-surface-raised transition-colors"
	}
	return "bg-surface-raised hover:bg-surface-sunken transition-colors"
}

// rowClass returns the <tr> class string, adding activation affordances
// (pointer cursor + focus ring) when the row is keyboard-activatable and a
// dimmed treatment when the row is muted (inactive/disabled).
func rowClass(idx int, r Row) string {
	c := trClass(idx)
	if r.Activate {
		c += " cursor-pointer focus-visible:outline-focus"
	}
	if r.Muted {
		c += " opacity-60"
	}
	return c
}

// footerCellClass returns the Tailwind class string for a <tfoot> cell.
func footerCellClass(c FooterCell) string {
	base := "px-4 py-2 text-ink"
	switch c.Align {
	case "center":
		base += " text-center"
	case "right":
		base += " text-right"
	}
	if c.Emphasis {
		base += " font-semibold"
	}
	return base
}

// footerColspan returns the colspan attribute value (minimum 1).
func footerColspan(c FooterCell) int {
	if c.Colspan < 1 {
		return 1
	}
	return c.Colspan
}

// cellValue safely returns the cell value at index i, returning an empty
// string when the row has fewer cells than columns.
func cellValue(cells []string, i int) string {
	if i < len(cells) {
		return cells[i]
	}
	return ""
}

// emptyColspan computes the colspan for the empty-state row.
func emptyColspan(p TableParams) string {
	n := len(p.Columns)
	if p.Selectable {
		n++
	}
	if p.RowAction != nil || p.RowHrefs {
		n++
	}
	return fmt.Sprintf("%d", n)
}

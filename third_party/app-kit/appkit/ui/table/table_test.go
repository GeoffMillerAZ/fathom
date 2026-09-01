package table

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
)

// render executes a templ.Component to a string for assertion.
func render(t *testing.T, c templ.Component) string {
	t.Helper()
	var buf bytes.Buffer
	if err := c.Render(context.Background(), &buf); err != nil {
		t.Fatalf("render error: %v", err)
	}
	return buf.String()
}

var testCols = []Column{
	{Key: "name", Label: "Name", Sortable: true},
	{Key: "status", Label: "Status", Sortable: false},
	{Key: "count", Label: "Count", Sortable: true, Align: "right"},
}

var testRows = []Row{
	{ID: "r1", Cells: []string{"Alpha", "active", "42"}},
	{ID: "r2", Cells: []string{"Beta", "inactive", "7"}},
	{ID: "r3", Cells: []string{"Gamma", "active", "0"}},
}

func TestTable_HTMLCells(t *testing.T) {
	// A rich cell (raw HTML component) renders instead of the escaped string;
	// columns without an HTMLCell fall back to the plain string cell.
	rows := []Row{{
		ID:        "r1",
		Cells:     []string{"Alpha", "ignored", "42"},
		HTMLCells: []templ.Component{nil, templ.Raw(`<span class="badge">live</span>`)},
	}}
	got := render(t, Table(TableParams{Columns: testCols, Rows: rows}))
	if !strings.Contains(got, `<span class="badge">live</span>`) {
		t.Errorf("rich HTMLCell not rendered raw; got:\n%s", got)
	}
	if strings.Contains(got, "ignored") {
		t.Errorf("string cell should be superseded by the non-nil HTMLCell")
	}
	if !strings.Contains(got, "Alpha") || !strings.Contains(got, "42") {
		t.Errorf("nil HTMLCells should fall back to string cells (Alpha/42)")
	}
}

func TestTable_HrefLinkUsesTokens(t *testing.T) {
	got := render(t, Table(TableParams{Columns: testCols, Rows: []Row{{ID: "r", Cells: []string{"A", "b", "1"}, Href: "/x"}}, RowHrefs: true}))
	if strings.Contains(got, "neon") {
		t.Errorf("href link must not use non-token neon colors; got:\n%s", got)
	}
	if !strings.Contains(got, "text-accent") {
		t.Errorf("href link should use the accent token")
	}
}

func TestTable_Structure(t *testing.T) {
	got := render(t, Table(TableParams{Columns: testCols, Rows: testRows}))
	for _, tag := range []string{"<table", "<thead", "<tbody", "<th", "<td", "<tr"} {
		if !strings.Contains(got, tag) {
			t.Errorf("table missing %s element; got:\n%s", tag, got)
		}
	}
}

func TestTable_TokenClasses(t *testing.T) {
	got := render(t, Table(TableParams{Columns: testCols, Rows: testRows}))
	for _, token := range []string{"border-line", "bg-surface-raised", "bg-surface-sunken", "text-ink", "text-ink-muted"} {
		if !strings.Contains(got, token) {
			t.Errorf("table missing token class %q; got:\n%s", token, got)
		}
	}
}

func TestTable_ColumnHeaders(t *testing.T) {
	got := render(t, Table(TableParams{Columns: testCols, Rows: testRows}))
	for _, col := range testCols {
		if !strings.Contains(got, col.Label) {
			t.Errorf("table missing column header %q; got:\n%s", col.Label, got)
		}
	}
}

func TestTable_SortableHeaders(t *testing.T) {
	got := render(t, Table(TableParams{Columns: testCols, Rows: testRows}))
	// Sortable columns emit data-table-sort attribute.
	if !strings.Contains(got, `data-table-sort="name"`) {
		t.Errorf("sortable column 'name' missing data-table-sort attr; got:\n%s", got)
	}
	if !strings.Contains(got, `data-table-sort="count"`) {
		t.Errorf("sortable column 'count' missing data-table-sort attr; got:\n%s", got)
	}
	// Non-sortable column should NOT have data-table-sort.
	if strings.Contains(got, `data-table-sort="status"`) {
		t.Errorf("non-sortable column 'status' should not have data-table-sort; got:\n%s", got)
	}
}

func TestTable_RowAttributes(t *testing.T) {
	got := render(t, Table(TableParams{Columns: testCols, Rows: testRows}))
	for _, row := range testRows {
		if !strings.Contains(got, `data-table-row="`+row.ID+`"`) {
			t.Errorf("row %q missing data-table-row attr; got:\n%s", row.ID, got)
		}
	}
}

func TestTable_ActivatableRow(t *testing.T) {
	// An activatable row renders button semantics + a focus ring, carries its
	// app-defined data-* hooks verbatim, and announces its accessible name.
	rows := []Row{{
		ID:       "r1",
		Cells:    []string{"Alpha", "active", "42"},
		Activate: true,
		Label:    "Open provenance for Alpha",
		Attrs:    templ.Attributes{"data-provenance-row": true, "data-prov-url": "/evidence/r1/provenance"},
	}}
	got := render(t, Table(TableParams{Columns: testCols, Rows: rows}))
	for _, want := range []string{
		`role="button"`,
		`tabindex="0"`,
		`aria-label="Open provenance for Alpha"`,
		`data-prov-url="/evidence/r1/provenance"`,
		`data-provenance-row`,
		"cursor-pointer",
		"focus-visible:outline-focus",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("activatable row missing %q; got:\n%s", want, got)
		}
	}
}

func TestTable_NonActivatableRow_NoButtonSemantics(t *testing.T) {
	// A plain row must not borrow activation semantics.
	got := render(t, Table(TableParams{Columns: testCols, Rows: testRows}))
	if strings.Contains(got, `role="button"`) {
		t.Errorf("non-activatable rows should not emit role=button; got:\n%s", got)
	}
	if strings.Contains(got, "cursor-pointer") {
		t.Errorf("non-activatable rows should not emit cursor-pointer; got:\n%s", got)
	}
}

func TestTable_MutedRow(t *testing.T) {
	rows := []Row{
		{ID: "r1", Cells: []string{"Alpha", "active", "42"}},
		{ID: "r2", Cells: []string{"Beta", "inactive", "7"}, Muted: true},
	}
	got := render(t, Table(TableParams{Columns: testCols, Rows: rows}))
	if !strings.Contains(got, "opacity-60") {
		t.Errorf("muted row should carry a dimmed class; got:\n%s", got)
	}
}

func TestTable_HeaderHTML(t *testing.T) {
	cols := []Column{
		{Key: "sel", Label: "Select", HeaderHTML: templ.Raw(`<input type="checkbox" data-select-all>`)},
		{Key: "name", Label: "Name", Sortable: true},
	}
	got := render(t, Table(TableParams{Columns: cols, Rows: []Row{{ID: "r", Cells: []string{"x", "Alpha"}}}}))
	if !strings.Contains(got, `data-select-all`) {
		t.Errorf("custom HeaderHTML not rendered; got:\n%s", got)
	}
	// HeaderHTML suppresses the sort button for that column but not others.
	if strings.Contains(got, `data-table-sort="sel"`) {
		t.Errorf("HeaderHTML column should not also render a sort button; got:\n%s", got)
	}
	if !strings.Contains(got, `data-table-sort="name"`) {
		t.Errorf("other sortable columns still render normally; got:\n%s", got)
	}
}

func TestTable_Footer(t *testing.T) {
	got := render(t, Table(TableParams{
		Columns: testCols,
		Rows:    testRows,
		Footer: []FooterRow{
			{Cells: []FooterCell{
				{Text: "Subtotal", Colspan: 2, Align: "right", Emphasis: true},
				{Text: "49", Align: "right", Emphasis: true},
			}},
		},
	}))
	if !strings.Contains(got, "<tfoot") {
		t.Errorf("footer should render a <tfoot>; got:\n%s", got)
	}
	if !strings.Contains(got, "Subtotal") || !strings.Contains(got, "49") {
		t.Errorf("footer cell content missing; got:\n%s", got)
	}
	if !strings.Contains(got, `colspan="2"`) {
		t.Errorf("footer colspan not emitted; got:\n%s", got)
	}
	if !strings.Contains(got, "font-semibold") {
		t.Errorf("emphasised footer cell should be bold; got:\n%s", got)
	}
}

func TestTable_NoFooter(t *testing.T) {
	got := render(t, Table(TableParams{Columns: testCols, Rows: testRows}))
	if strings.Contains(got, "<tfoot") {
		t.Errorf("table without Footer should not emit <tfoot>; got:\n%s", got)
	}
}

func TestFooterColspan_Default(t *testing.T) {
	if got := footerColspan(FooterCell{}); got != 1 {
		t.Errorf("footerColspan default = %d, want 1", got)
	}
	if got := footerColspan(FooterCell{Colspan: 3}); got != 3 {
		t.Errorf("footerColspan(3) = %d, want 3", got)
	}
}

func TestTable_CellValues(t *testing.T) {
	got := render(t, Table(TableParams{Columns: testCols, Rows: testRows}))
	for _, row := range testRows {
		for _, cell := range row.Cells {
			if !strings.Contains(got, cell) {
				t.Errorf("cell value %q missing from table; got:\n%s", cell, got)
			}
		}
	}
}

func TestTable_WithTableID(t *testing.T) {
	got := render(t, Table(TableParams{
		TableID: "my-table",
		Columns: testCols,
		Rows:    testRows,
	}))
	if !strings.Contains(got, `data-table="my-table"`) {
		t.Errorf("table missing data-table attr; got:\n%s", got)
	}
}

func TestTable_WithoutTableID(t *testing.T) {
	got := render(t, Table(TableParams{Columns: testCols, Rows: testRows}))
	if strings.Contains(got, `data-table=`) {
		t.Errorf("table without TableID should not emit data-table attr; got:\n%s", got)
	}
}

func TestTable_Selectable(t *testing.T) {
	got := render(t, Table(TableParams{
		Columns:    testCols,
		Rows:       testRows,
		Selectable: true,
	}))
	if !strings.Contains(got, `type="checkbox"`) {
		t.Errorf("selectable table missing checkbox inputs; got:\n%s", got)
	}
	for _, row := range testRows {
		if !strings.Contains(got, `data-table-select="`+row.ID+`"`) {
			t.Errorf("selectable table missing data-table-select for %q; got:\n%s", row.ID, got)
		}
	}
}

func TestTable_RowAction(t *testing.T) {
	got := render(t, Table(TableParams{
		Columns:   testCols,
		Rows:      testRows,
		RowAction: actionButton(),
	}))
	if !strings.Contains(got, "Edit") {
		t.Errorf("row action slot content missing; got:\n%s", got)
	}
	if !strings.Contains(got, "sr-only") {
		t.Errorf("row action column missing sr-only Actions header; got:\n%s", got)
	}
}

func TestTable_NoRowAction(t *testing.T) {
	got := render(t, Table(TableParams{Columns: testCols, Rows: testRows}))
	if strings.Contains(got, "sr-only") {
		t.Errorf("table without RowAction should not emit action column header; got:\n%s", got)
	}
}

func TestTable_SortActive(t *testing.T) {
	got := render(t, Table(TableParams{
		Columns: testCols,
		Rows:    testRows,
		SortKey: "name",
		SortDir: "asc",
	}))
	// Active sort column shows up-arrow indicator.
	if !strings.Contains(got, "↑") {
		t.Errorf("active sort asc column should show ↑ indicator; got:\n%s", got)
	}
	// Inactive sort columns show the neutral indicator.
	if !strings.Contains(got, "↕") {
		t.Errorf("inactive sort columns should show ↕ indicator; got:\n%s", got)
	}
}

func TestTable_SortDesc(t *testing.T) {
	got := render(t, Table(TableParams{
		Columns: testCols,
		Rows:    testRows,
		SortKey: "count",
		SortDir: "desc",
	}))
	if !strings.Contains(got, "↓") {
		t.Errorf("active sort desc column should show ↓ indicator; got:\n%s", got)
	}
}

func TestTable_EmptyRows(t *testing.T) {
	got := render(t, Table(TableParams{Columns: testCols, Rows: nil}))
	if !strings.Contains(got, "No data") {
		t.Errorf("empty table should render 'No data' empty state; got:\n%s", got)
	}
}

func TestTable_Caption(t *testing.T) {
	got := render(t, Table(TableParams{
		Columns: testCols,
		Rows:    testRows,
		Caption: "Active deployments",
	}))
	if !strings.Contains(got, "Active deployments") {
		t.Errorf("table caption missing; got:\n%s", got)
	}
	if !strings.Contains(got, "<caption") {
		t.Errorf("table missing <caption> element; got:\n%s", got)
	}
}

func TestTable_NoHexColors(t *testing.T) {
	got := render(t, Table(TableParams{
		Columns:    testCols,
		Rows:       testRows,
		Selectable: true,
		RowAction:  actionButton(),
	}))
	if strings.Contains(got, "#") {
		t.Errorf("table output contains a literal hex color; got:\n%s", got)
	}
}

func TestTable_MissingCells_Tolerated(t *testing.T) {
	// Row with fewer cells than columns; should not panic.
	short := []Row{{ID: "s1", Cells: []string{"Only One"}}}
	got := render(t, Table(TableParams{Columns: testCols, Rows: short}))
	if !strings.Contains(got, "Only One") {
		t.Errorf("table missing available cell value; got:\n%s", got)
	}
}

// ---- unit: helper functions ------------------------------------------------

func TestThClass_Alignment(t *testing.T) {
	cases := []struct {
		align string
		want  string
	}{
		{"left", "text-left"},
		{"center", "text-center"},
		{"right", "text-right"},
		{"", "text-left"},
	}
	for _, tc := range cases {
		cls := thClass(Column{Align: tc.align})
		if !strings.Contains(cls, tc.want) {
			t.Errorf("thClass(align=%q) missing %q; got %q", tc.align, tc.want, cls)
		}
	}
}

func TestTdClass_Alignment(t *testing.T) {
	cls := tdClass(Column{Align: "right"})
	if !strings.Contains(cls, "text-right") {
		t.Errorf("tdClass right-aligned missing text-right; got %q", cls)
	}
	if !strings.Contains(cls, "text-ink") {
		t.Errorf("tdClass missing text-ink token class; got %q", cls)
	}
}

func TestTrClass_Striping(t *testing.T) {
	even := trClass(0)
	odd := trClass(1)
	if !strings.Contains(even, "bg-surface-raised") {
		t.Errorf("even row missing bg-surface-raised; got %q", even)
	}
	if !strings.Contains(odd, "bg-surface-sunken") {
		t.Errorf("odd row missing bg-surface-sunken; got %q", odd)
	}
}

func TestCellValue_InBounds(t *testing.T) {
	cells := []string{"a", "b", "c"}
	if got := cellValue(cells, 1); got != "b" {
		t.Errorf("cellValue(1) = %q, want b", got)
	}
}

func TestCellValue_OutOfBounds(t *testing.T) {
	cells := []string{"a"}
	if got := cellValue(cells, 5); got != "" {
		t.Errorf("cellValue out-of-bounds = %q, want empty", got)
	}
}

func TestEmptyColspan_Math(t *testing.T) {
	cases := []struct {
		p    TableParams
		want string
	}{
		{TableParams{Columns: testCols}, "3"},
		{TableParams{Columns: testCols, Selectable: true}, "4"},
		{TableParams{Columns: testCols, RowAction: actionButton()}, "4"},
		{TableParams{Columns: testCols, Selectable: true, RowAction: actionButton()}, "5"},
		{TableParams{Columns: testCols, RowHrefs: true}, "4"},
		{TableParams{Columns: testCols, Selectable: true, RowHrefs: true}, "5"},
	}
	for _, tc := range cases {
		got := emptyColspan(tc.p)
		if got != tc.want {
			t.Errorf("emptyColspan(%+v) = %q, want %q", tc.p, got, tc.want)
		}
	}
}

package table

import (
	"testing"

	"github.com/a-h/templ"
	"golang.org/x/net/html"
)

// headerShapes covers every header cell the template can emit: the leading
// select column, a sortable button, rich HeaderHTML, a plain label and the
// trailing action column (inferred from Href, reserved by RowHrefs, or a slot).
func headerShapes() map[string]TableParams {
	cols := []Column{
		{Key: "name", Label: "Name", Sortable: true},
		{Key: "all", Label: "All", HeaderHTML: templ.Raw(`<input type="checkbox" aria-label="Select all">`)},
		{Key: "count", Label: "Count", Align: "right"},
	}
	rows := []Row{{ID: "r1", Cells: []string{"Alpha", "", "42"}, Href: "/alpha"}, {ID: "r2", Cells: []string{"Beta", "", "7"}}}
	return map[string]TableParams{
		"plain":         {Columns: cols, Rows: rows[1:]},
		"selectable":    {Columns: cols, Rows: rows, Selectable: true},
		"inferred-href": {Columns: cols, Rows: rows},
		"reserved":      {Columns: cols, RowHrefs: true},
		"slot":          {Columns: cols, Rows: rows, RowAction: actionButton()},
		"footer":        {Columns: cols, Rows: rows, Footer: []FooterRow{{Cells: []FooterCell{{Text: "Total", Colspan: 3}, {Text: "49"}}}}},
	}
}

func headerCells(n *html.Node, out []*html.Node) []*html.Node {
	if n.Type == html.ElementNode && n.Data == "th" {
		out = append(out, n)
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		out = headerCells(child, out)
	}
	return out
}

func insideTag(n *html.Node, tag string) bool {
	for p := n.Parent; p != nil; p = p.Parent {
		if p.Type == html.ElementNode && p.Data == tag {
			return true
		}
	}
	return false
}

// FATHOM-F0-KIT-01: every header cell names the column it heads. The table has
// no row-header cells (body and footer cells are all <td>), so each <th> sits
// in <thead> and carries scope="col".
func TestTable_EveryHeaderCellHasColumnScope(t *testing.T) {
	for name, p := range headerShapes() {
		doc := linkTestDocument(t, render(t, Table(p)))
		ths := headerCells(doc, nil)
		want := len(p.Columns) + boolInt(p.Selectable) + boolInt(withActionColumn(p).RowHrefs || p.RowAction != nil)
		if len(ths) != want {
			t.Errorf("%s: %d header cells, want %d", name, len(ths), want)
		}
		for i, th := range ths {
			if got := linkTestAttr(th, "scope"); got != "col" {
				t.Errorf("%s: header cell %d (%q) scope = %q, want col", name, i, actionColumnText(th), got)
			}
			if !insideTag(th, "thead") {
				t.Errorf("%s: header cell %d is outside <thead>; a row header would need scope=row", name, i)
			}
		}
	}
}

// FATHOM-F0-KIT-02: the visually hidden "Select"/"Actions" header labels are
// absolutely positioned (sr-only). Their containing block must be the table's
// own scroll wrapper, or a table wider than a 390px screen lets the hidden
// label escape the wrapper and widen the whole document. The wrapper is the
// first element, is positioned (relative) and scrolls horizontally, and every
// sr-only label is its descendant with no other positioned ancestor between.
func TestTable_ScrollWrapperContainsHiddenHeaderLabels(t *testing.T) {
	for name, p := range headerShapes() {
		doc := linkTestDocument(t, render(t, Table(p)))
		wrapper := actionColumnFind(doc, "div", "class")
		if wrapper == nil {
			t.Fatalf("%s: no wrapper div", name)
		}
		for _, class := range []string{"relative", "overflow-x-auto"} {
			if !linkTestClassHas(wrapper, class) {
				t.Errorf("%s: scroll wrapper class %q lacks %q", name, linkTestAttr(wrapper, "class"), class)
			}
		}
		if table := actionColumnChildren(wrapper, "table"); len(table) != 1 {
			t.Errorf("%s: wrapper must directly contain the one <table>, got %d", name, len(table))
		}
		var hidden []*html.Node
		var visit func(*html.Node)
		visit = func(n *html.Node) {
			if n.Type == html.ElementNode && linkTestClassHas(n, "sr-only") {
				hidden = append(hidden, n)
			}
			for child := n.FirstChild; child != nil; child = child.NextSibling {
				visit(child)
			}
		}
		visit(wrapper)
		wantHidden := boolInt(p.Selectable) + boolInt(withActionColumn(p).RowHrefs || p.RowAction != nil)
		if len(hidden) != wantHidden {
			t.Errorf("%s: %d sr-only header labels inside the wrapper, want %d", name, len(hidden), wantHidden)
		}
	}
}

package table

import (
	"context"
	"fmt"
	"io"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/a-h/templ"
	"golang.org/x/net/html"
)

func TestTable_ActionColumnConcurrentRender(t *testing.T) {
	component := Table(TableParams{
		Columns: []Column{{Key: "name", Label: "Name"}},
		Rows: []Row{{ID: "linked", Cells: []string{"Linked"}, Href: "/details"},
			{ID: "plain", Cells: []string{"Plain"}}},
	})
	start := make(chan struct{})
	var workers sync.WaitGroup
	for range 8 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			<-start
			for range 8 {
				if err := component.Render(context.Background(), io.Discard); err != nil {
					t.Errorf("render: %v", err)
				}
			}
		}()
	}
	close(start)
	workers.Wait()
}

func TestTable_ActionColumnConsistency(t *testing.T) {
	columns := []Column{{Key: "name", Label: "Name"}, {Key: "state", Label: "State"}}
	rowSets := []struct {
		name string
		rows []Row
	}{
		{name: "none"},
		{name: "no-href", rows: []Row{{ID: "none", Cells: []string{"None", "Idle"}}}},
		{name: "mixed-valid-unsafe-empty", rows: []Row{
			{ID: "valid", Cells: []string{"Valid", "Ready"}, Href: "/details"},
			{ID: "unsafe", Cells: []string{"Unsafe", "Blocked"}, Href: "javascript:alert(1)"},
			{ID: "empty", Cells: []string{"Empty", "Idle"}},
		}},
	}
	for _, selectable := range []bool{false, true} {
		for _, rowHrefs := range []bool{false, true} {
			for _, custom := range []bool{false, true} {
				for _, rows := range rowSets {
					name := fmt.Sprintf("selectable=%t/row-hrefs=%t/custom=%t/%s", selectable, rowHrefs, custom, rows.name)
					t.Run(name, func(t *testing.T) {
						var rowAction templ.Component
						if custom {
							rowAction = templ.Raw(`<button type="button" data-custom-action>Edit</button>`)
						}
						params := TableParams{
							Columns: append([]Column(nil), columns...),
							Rows:    append([]Row(nil), rows.rows...), Selectable: selectable,
							RowAction: rowAction, RowHrefs: rowHrefs,
						}
						beforeColumns := append([]Column(nil), params.Columns...)
						beforeRows := cloneActionColumnRows(params.Rows)
						body := render(t, Table(params))
						if !reflect.DeepEqual(params.Columns, beforeColumns) || !reflect.DeepEqual(params.Rows, beforeRows) || params.Selectable != selectable || params.RowHrefs != rowHrefs {
							t.Fatal("render mutated TableParams")
						}

						doc := linkTestDocument(t, body)
						table := actionColumnFind(doc, "table", "")
						if table == nil {
							t.Fatalf("rendered document has no table: %s", body)
						}
						hasRowHref := false
						for _, row := range params.Rows {
							hasRowHref = hasRowHref || row.Href != ""
						}
						hasAction := custom || rowHrefs || hasRowHref
						wantColumns := len(columns) + boolInt(selectable) + boolInt(hasAction)
						head := actionColumnFind(table, "thead", "")
						if got := len(actionColumnChildren(actionColumnFind(head, "tr", ""), "th")); got != wantColumns {
							t.Errorf("header columns = %d, want %d", got, wantColumns)
						}

						tbody := actionColumnFind(table, "tbody", "")
						bodyRows := actionColumnChildren(tbody, "tr")
						if len(params.Rows) == 0 {
							if len(bodyRows) != 1 {
								t.Fatalf("empty body rows = %d, want 1", len(bodyRows))
							}
							cells := actionColumnChildren(bodyRows[0], "td")
							if len(cells) != 1 || linkTestAttr(cells[0], "colspan") != fmt.Sprint(wantColumns) {
								t.Errorf("empty row colspan = %q, want %d", linkTestAttr(cells[0], "colspan"), wantColumns)
							}
							return
						}
						if len(bodyRows) != len(params.Rows) {
							t.Fatalf("body rows = %d, want %d", len(bodyRows), len(params.Rows))
						}
						for i, rowNode := range bodyRows {
							cells := actionColumnChildren(rowNode, "td")
							if len(cells) != wantColumns {
								t.Errorf("row %q columns = %d, want %d", params.Rows[i].ID, len(cells), wantColumns)
								continue
							}
							if !hasAction {
								continue
							}
							action := cells[len(cells)-1]
							markup := actionColumnHTML(action)
							if custom {
								if !strings.Contains(markup, "data-custom-action") || strings.Contains(markup, "/details") || strings.Contains(markup, "Open unavailable") {
									t.Errorf("row %q lost custom action precedence: %s", params.Rows[i].ID, markup)
								}
							} else if params.Rows[i].Href == "" {
								if strings.TrimSpace(actionColumnText(action)) != "" || strings.Contains(markup, "href=") || strings.Contains(markup, "Open unavailable") {
									t.Errorf("row %q invented an action: %s", params.Rows[i].ID, markup)
								}
							} else if params.Rows[i].ID == "valid" {
								if link := actionColumnFind(action, "a", ""); link == nil || linkTestAttr(link, "href") != "/details" {
									t.Errorf("valid row action = %s", markup)
								}
							} else if !strings.Contains(markup, "Open unavailable") || strings.Contains(markup, "href=") {
								t.Errorf("unsafe row action = %s", markup)
							}
						}
					})
				}
			}
		}
	}
}

func cloneActionColumnRows(rows []Row) []Row {
	out := append([]Row(nil), rows...)
	for i := range out {
		out[i].Cells = append([]string(nil), rows[i].Cells...)
		out[i].HTMLCells = append([]templ.Component(nil), rows[i].HTMLCells...)
	}
	return out
}

func actionColumnChildren(n *html.Node, tag string) []*html.Node {
	var out []*html.Node
	if n == nil {
		return out
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode && child.Data == tag {
			out = append(out, child)
		}
	}
	return out
}

func actionColumnFind(n *html.Node, tag, attr string) *html.Node {
	if n == nil {
		return nil
	}
	if n.Type == html.ElementNode && n.Data == tag && (attr == "" || linkTestAttr(n, attr) != "") {
		return n
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if found := actionColumnFind(child, tag, attr); found != nil {
			return found
		}
	}
	return nil
}

func actionColumnText(n *html.Node) string {
	if n == nil {
		return ""
	}
	if n.Type == html.TextNode {
		return n.Data
	}
	var out strings.Builder
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		out.WriteString(actionColumnText(child))
	}
	return out.String()
}

func actionColumnHTML(n *html.Node) string {
	var out strings.Builder
	if n != nil {
		_ = html.Render(&out, n)
	}
	return out.String()
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

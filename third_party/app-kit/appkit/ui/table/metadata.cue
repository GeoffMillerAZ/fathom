// Component registry metadata for appkit/ui/table.
// Validated by the appkit CUE registry schema (D7/D12).
package table

component: {
	name:        "Table"
	package:     "github.com/geoffmilleraz/app-kit/appkit/ui/table"
	description: "Generic token-driven data table with typed columns, rows, sortable headers, row selection, and an optional row-action slot."
	since:       "0.1.0"

	params: [
		{name: "TableID", type: "string", required: false, description: "Optional identifier emitted as data-table attr for Datastar targeting."},
		{name: "Columns", type: "[]Column", required: true, description: "Column definitions (key, label, sortable, alignment)."},
		{name: "Rows", type: "[]Row", required: false, description: "Row data. Cell count may be shorter than Columns; extra are rendered empty."},
		{name: "SortKey", type: "string", required: false, description: "Currently active sort column Key. Empty = no active sort."},
		{name: "SortDir", type: "string", required: false, description: "Current sort direction: 'asc' or 'desc'."},
		{name: "Selectable", type: "bool", required: false, description: "Adds a leading checkbox column. App wires data-on-change to a selection endpoint."},
		{name: "RowAction", type: "templ.Component", required: false, description: "Optional trailing-cell slot rendered for every row; takes precedence over managed Href links. Nil alone does not suppress an otherwise required action column."},
		{name: "RowHrefs", type: "bool", required: false, description: "Reserve an action column even for empty/unlinked results. Actual nonempty row Hrefs or a custom action slot infer the column automatically; unlinked rows get an empty action cell."},
		{name: "Caption", type: "string", required: false, description: "Optional accessible <caption> element rendered above the header row."},
	]

	types: {
		Column: {
			fields: [
				{name: "Key", type: "string", description: "Column identifier. Emitted as data-table-sort when Sortable=true."},
				{name: "Label", type: "string", description: "User-visible column header text."},
				{name: "Sortable", type: "bool", description: "Renders header as a clickable sort trigger."},
				{name: "Align", type: "string", description: "Text alignment: 'left' (default), 'center', 'right'."},
			]
		}
		Row: {
			fields: [
				{name: "ID", type: "string", description: "Row identifier. Emitted as data-table-row on the <tr> and as data-table-select on checkboxes."},
				{name: "Cells", type: "[]string", description: "Cell values in Column order. Short rows are tolerated; missing cells render empty."},
				{name: "HTMLCells", type: "[]templ.Component", description: "Rich cell content in Column order; a non-nil entry renders instead of the escaped Cells string."},
				{name: "Href", type: "string", description: "Relative web reference or absolute HTTP(S) URL for a native Open link. Unsafe nonempty input becomes an inert Open unavailable label; empty input adds no action. Rebuild scanned CSS for 44px targets/focus. Apps own route/origin authorization."},
				{name: "Activate", type: "bool", description: "Render the row as a keyboard-activatable target (role=button, tabindex=0, pointer cursor + focus ring) for in-page interactions like opening a drawer. Independent of Href."},
				{name: "Label", type: "string", description: "Accessible name emitted as aria-label on the <tr>; pair with Activate."},
				{name: "Attrs", type: "templ.Attributes", description: "App-defined attributes emitted verbatim on the <tr> (e.g. a data-* URL a JS island reads on activation). Keys render in sorted order."},
				{name: "Muted", type: "bool", description: "Dim the row (reduced opacity) to mark it inactive/disabled/archived/voided while keeping it readable and in place."},
			]
		}
		Column_HeaderHTML: {
			fields: [
				{name: "HeaderHTML", type: "templ.Component", description: "Rich header content (e.g. a select-all checkbox, info tooltip) rendered in place of Label. Suppresses the sort button for that column."},
			]
		}
		FooterCell: {
			fields: [
				{name: "Text", type: "string", description: "Plain footer cell text; escaped. Ignored when HTML is set."},
				{name: "HTML", type: "templ.Component", description: "Rich footer content rendered instead of Text."},
				{name: "Colspan", type: "int", description: "Columns this cell spans (default 1). Each footer row's colspans must sum to the table's total column count."},
				{name: "Align", type: "string", description: "Text alignment: 'left' (default), 'center', 'right'."},
				{name: "Emphasis", type: "bool", description: "Render bold — for subtotal/total labels and values."},
			]
		}
		FooterRow: {
			fields: [
				{name: "Cells", type: "[]FooterCell", description: "Footer cells left to right, rendered in <tfoot>."},
			]
		}
	}

	// interactionContract documents the data-* hooks this component emits and
	// what the consuming app must wire to Datastar endpoints.
	interactionContract: {
		emitted: [
			{attr: "data-table={tableID}", element: "table element", description: "Emitted only when TableID is set. Selector hook for app targeting."},
			{attr: "data-table-sort={col.Key}", element: "sortable <th> button", description: "Identifies which column header was clicked for sort."},
			{attr: "data-table-row={row.ID}", element: "each <tr>", description: "Identifies the row for app-level event delegation."},
			{attr: "data-table-select={row.ID}", element: "checkbox input", description: "Emitted when Selectable=true. Identifies which row's selection changed."},
			{attr: "role=button + tabindex=0", element: "each <tr> with Row.Activate", description: "Marks the row as a keyboard-activatable target for in-page interactions (drawer/detail panel)."},
			{attr: "{Row.Attrs...}", element: "each <tr>", description: "App-defined attributes emitted verbatim (e.g. a data-* URL the app fetches on activation). The kit is presentational; the app owns the endpoint and the handler."},
		]
		consumerMustWire: [
			{event: "click / data-on-click", element: "data-table-sort button", description: "POST {tableID, sortKey, nextSortDir} to an app-owned sort endpoint. SSE stream delivers re-rendered table with new SortKey/SortDir."},
			{event: "change / data-on-change", element: "data-table-select checkbox", description: "POST {tableID, rowID, checked} to an app-owned selection endpoint. SSE delivers updated state."},
			{event: "click + keydown(Enter/Space) / data-on-click", element: "<tr> with Row.Activate", description: "Read the row's Attrs hook (e.g. data-prov-url) and open the app's drawer/panel; the app owns that fetch + focus management."},
		]
	}

	variants: [
		{name: "default", description: "Three columns, three rows, no sort, no selection, no action."},
		{name: "sortable-active", description: "SortKey and SortDir set; shows active direction indicator."},
		{name: "selectable", description: "Selectable=true with checkbox column."},
		{name: "with-action", description: "RowAction slot populated with an edit button."},
		{name: "activatable-rows", description: "Rows with Activate=true + Attrs hooks; render as role=button targets that open an app drawer on activation."},
		{name: "muted-rows", description: "Rows with Muted=true render dimmed to mark inactive/disabled state."},
		{name: "header-html", description: "A Column with HeaderHTML (e.g. a select-all checkbox) renders rich header content in place of the label."},
		{name: "footer-totals", description: "Footer rows render a <tfoot> of subtotal/total summary cells with colspans."},
		{name: "empty", description: "No rows; renders 'No data' empty state."},
	]

	// tokenRoles lists every kit token role referenced by this component.
	tokenRoles: [
		"surface-raised",
		"surface-sunken",
		"line",
		"ink",
		"ink-muted",
		"ink-faint",
		"accent",
		"focus",
	]
}

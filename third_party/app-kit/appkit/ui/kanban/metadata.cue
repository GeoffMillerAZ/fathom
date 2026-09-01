// Component registry metadata for appkit/ui/kanban.
// Validated by the appkit CUE registry schema (D7/D12).
package kanban

component: {
	name:        "Board"
	package:     "github.com/geoffmilleraz/app-kit/appkit/ui/kanban"
	description: "Data-driven kanban board rendering columns of card primitives. Presentational: emits data-* hooks for Datastar wiring."
	since:       "0.1.0"

	params: [
		{name: "columns", type: "[]Column", required: true, description: "Stage lane descriptors (ID, Label, Count). Order is preserved."},
		{name: "cards", type: "[]CardItem", required: false, description: "Card data rows distributed into columns by StageID match."},
	]

	types: {
		Column: {
			fields: [
				{name: "ID", type: "string", description: "Stage identifier. Emitted as data-kanban-col. Must match CardItem.StageID."},
				{name: "Label", type: "string", description: "User-visible column header."},
				{name: "Count", type: "int", description: "Card count shown in header badge. Pass -1 to suppress."},
			]
		}
		CardItem: {
			fields: [
				{name: "ID", type: "string", description: "Card identifier. Emitted as data-kanban-card."},
				{name: "StageID", type: "string", description: "Must match a Column.ID. Cards with no match are silently dropped."},
				{name: "Title", type: "string", description: "Primary card text."},
				{name: "Subtitle", type: "string", description: "Optional secondary text (assignee, due date, etc.)."},
				{name: "Badges", type: "[]Badge", description: "Optional label chips (priority, type, tag)."},
				{name: "Meta", type: "string", description: "Optional trailing metadata text."},
				{name: "Href", type: "string", description: "Optional URL. When non-empty the card title renders as an anchor linking to this URL. The card wrapper remains draggable. Empty string = no link (backward-compatible)."},
			]
		}
		Badge: {
			fields: [
				{name: "Label", type: "string", description: "Badge text."},
				{name: "Role", type: "string", description: "Token color role for badge surface. Valid: positive-soft, caution-soft, danger-soft, info-soft, accent-soft. Falls back to surface-sunken."},
			]
		}
	}

	// interactionContract documents the data-* hooks this component emits and
	// what the consuming app must wire to Datastar endpoints.
	interactionContract: {
		emitted: [
			{attr: "data-kanban-board", element: "root div", description: "Selector hook for app-level board initialization."},
			{attr: "data-kanban-col={col.ID}", element: "column wrapper", description: "Identifies the drop-target stage lane."},
			{attr: "data-kanban-card={card.ID}", element: "card wrapper", description: "Identifies the draggable card."},
			{attr: "draggable=true", element: "card wrapper", description: "Enables HTML5 drag-and-drop on each card."},
		]
		consumerMustWire: [
			{event: "dragstart", element: "data-kanban-card", description: "Record dragged card ID + source stage in a Datastar signal (ephemeral client state)."},
			{event: "dragover", element: "data-kanban-col", description: "preventDefault() to enable drop; apply a visual drop-zone highlight (e.g. ring-2 ring-accent)."},
			{event: "dragleave", element: "data-kanban-col", description: "Remove drop-zone highlight."},
			{event: "drop", element: "data-kanban-col", description: "POST {cardID, fromStageID, toStageID} to an app-owned move endpoint. The SSE stream delivers the re-rendered board patch. No optimistic update."},
		]
	}

	variants: [
		{name: "default", description: "Three columns (todo/doing/done) with sample cards."},
		{name: "empty-columns", description: "Columns with no cards — renders empty drop zones."},
		{name: "many-badges", description: "Cards with multiple badge chips."},
	]

	// tokenRoles lists every kit token role referenced by this component.
	tokenRoles: [
		"surface-sunken",
		"surface-raised",  // via card.Card
		"line",            // via card.Card border + badge border
		"ink",
		"ink-muted",
		"ink-faint",
		"positive-soft",
		"caution-soft",
		"danger-soft",
		"info-soft",
		"accent-soft",
	]

	dependencies: [
		"github.com/geoffmilleraz/app-kit/appkit/ui/card",
	]
}

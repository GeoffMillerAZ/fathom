// Package kanban provides a server-rendered, token-driven Kanban board
// composed from appkit/ui/card primitives.
//
// # Data model
//
// A Board takes a slice of Column descriptors and a slice of CardItem data
// rows. Column.ID must match CardItem.StageID for the card to appear in that
// column. Cards with a StageID not present in any column are silently dropped.
//
// # Interaction contract (Datastar / hypermedia)
//
// This package is PRESENTATIONAL. It emits data-* hooks so the consuming app
// can wire Datastar handlers to backend endpoints without this package knowing
// any routes.
//
// Emitted hooks (consuming app responsibilities in []):
//
//	data-kanban-board             on the root board element
//	data-kanban-col="{col.ID}"    on each column wrapper
//	data-kanban-card="{card.ID}"  on each card wrapper
//	draggable="true"              on each card wrapper
//
//	data-on-dragstart="..."       [app]: record drag source card ID + stage
//	data-on-dragover="..."        [app]: prevent default, highlight drop zone
//	data-on-dragleave="..."       [app]: remove drop-zone highlight
//	data-on-drop="..."            [app]: POST to /your/move endpoint with
//	                                    {cardID, fromStageID, toStageID}; the
//	                                    SSE stream delivers re-rendered board
//
// Stage transitions are short POSTs; the server owns state; the DOM is a
// projection (Tao-of-Datastar CQRS shape, ADR-0003). No JS state or
// optimistic update is implied by this package.
//
// Column counts (Column.Count) are rendered in the header badge. Pass -1 to
// hide the badge (e.g. when count is unknown or irrelevant).
//
// # Theming
//
// All colors are kit token roles. No hex values appear in this package.
package kanban

// Column describes one stage lane in the board.
type Column struct {
	// ID uniquely identifies this stage. Must match CardItem.StageID for cards
	// to appear here. Emitted as data-kanban-col attribute.
	ID string
	// Label is the column header text (user-visible).
	Label string
	// Count is the number of cards in this stage, rendered in the header badge.
	// Pass -1 to suppress the badge.
	Count int
}

// Badge is a small labeled chip rendered on a card (e.g. priority, type, tag).
type Badge struct {
	// Label is the badge text.
	Label string
	// Role is the token color role for the badge surface (e.g. "positive-soft",
	// "caution-soft", "danger-soft", "info-soft", "accent-soft"). Falls back to
	// "surface-sunken" for unknown/empty roles.
	Role string
}

// CardItem is one data row rendered as a card in a column.
type CardItem struct {
	// ID uniquely identifies this card. Emitted as data-kanban-card attribute.
	ID string
	// StageID must match a Column.ID for the card to appear in that column.
	StageID string
	// Title is the primary card text.
	Title string
	// Subtitle is optional secondary text (e.g. assignee, due date).
	Subtitle string
	// Badges are optional chips rendered below the title row.
	Badges []Badge
	// Meta is optional trailing metadata text (e.g. "3 subtasks").
	Meta string
	// Href is an optional URL. When non-empty the card title is rendered as an
	// anchor element linking to this URL. The card wrapper remains draggable.
	Href string
}

// cardsForStage filters cards to those belonging to the given stage ID.
// Returns a nil slice (not an empty slice) when none match, to avoid
// allocating when columns are empty.
func cardsForStage(cards []CardItem, stageID string) []CardItem {
	var out []CardItem
	for _, c := range cards {
		if c.StageID == stageID {
			out = append(out, c)
		}
	}
	return out
}

// softRoles is the set of valid "soft" token roles for badge surfaces.
// Any role not in this set falls back to surface-sunken.
var softRoles = map[string]bool{
	"positive-soft":  true,
	"caution-soft":   true,
	"danger-soft":    true,
	"info-soft":      true,
	"accent-soft":    true,
	"surface-sunken": true,
}

// badgeClass returns the Tailwind class string for a badge chip.
// Only known token roles are emitted; unknown/empty roles fall back to
// surface-sunken — never inject an arbitrary caller string into a class attr.
func badgeClass(role string) string {
	if !softRoles[role] {
		role = "surface-sunken"
	}
	return "inline-flex items-center rounded-full px-2 py-0.5 text-xs font-sans font-medium text-ink border border-line bg-" + role
}

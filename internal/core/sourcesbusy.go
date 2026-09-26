package core

import "errors"

// ErrSourcesBusy is RT-146's named skip (docs/design/responsible-tenant.md
// §4.6): a hook-origin whole-tree read found both process-wide slots held
// and did not wait. It lives in core so every layer that renders a
// response (pkg/client, internal/mcp, the hooks) can name it with
// errors.Is — a skipped read is disclosed, never shown as an empty corpus
// (I38).
var ErrSourcesBusy = errors.New("coverage skipped: whole-tree read in progress")

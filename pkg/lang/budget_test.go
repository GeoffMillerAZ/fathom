package lang_test

import (
	"time"

	"github.com/geoffmilleraz/fathom/internal/extractor/tsengine"
)

// shrinkParseTimeout makes the engine's deadline one nanosecond and returns
// the function that restores it.
func shrinkParseTimeout() func() {
	old := tsengine.ParseTimeout
	tsengine.ParseTimeout = time.Nanosecond
	return func() { tsengine.ParseTimeout = old }
}

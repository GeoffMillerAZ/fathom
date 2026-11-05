package lang_test

import (
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/extractor"
	"github.com/geoffmilleraz/fathom/internal/langroute"
	"github.com/geoffmilleraz/fathom/pkg/lang"
)

// The public constants are compatibility surface and the API snapshot cannot
// see an alias's value, so each is written as a literal in the package and its
// value is asserted here, against the literal and against the internal
// constant it must agree with.
func TestPublicConstantValues(t *testing.T) {
	for _, c := range []struct {
		name, got, want, inner string
	}{
		{"PackEmpty", lang.PackEmpty, "empty", extractor.PackEmpty},
		{"PackManifest", lang.PackManifest, "manifest", extractor.PackManifest},
		{"PackConflict", lang.PackConflict, "conflict", extractor.PackConflict},
		{"PackEngine", lang.PackEngine, "engine", extractor.PackEngine},
		{"PackInjections", lang.PackInjections, "injections", extractor.PackInjections},
		{"PackGrammar", lang.PackGrammar, "grammar", extractor.PackGrammar},
		{"PackQuery", lang.PackQuery, "query", extractor.PackQuery},
		{"PackContainer", lang.PackContainer, "container", extractor.PackContainer},
		{"PackClaims", lang.PackClaims, "claims", extractor.PackClaims},
		{"PackRoute", lang.PackRoute, "route", extractor.PackRoute},
		{"SkipBinary", lang.SkipBinary, "binary", "binary"},
		{"SkipOversize", lang.SkipOversize, "oversize", "oversize"},
		{"SkipGenerated", lang.SkipGenerated, "generated", "generated"},
		{"SkipAmbiguous", lang.SkipAmbiguous, "ambiguous", "ambiguous"},
		{"SkipUnclassified", lang.SkipUnclassified, "unclassified", "unclassified"},
		{"SkipNotLinked", lang.SkipNotLinked, "not-linked", "not-linked"},
		{"SkipBusy", lang.SkipBusy, "busy", "busy"},
		{"LevelUnmeasured", lang.LevelUnmeasured, "unmeasured", "unmeasured"},
		{"LevelNone", lang.LevelNone, "none", "none"},
	} {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q: a change to a public constant is a breaking change", c.name, c.got, c.want)
		}
		if c.got != c.inner {
			t.Errorf("%s = %q but the internal constant it must agree with is %q", c.name, c.got, c.inner)
		}
	}
	if lang.DefaultReadDeadline != 30*time.Second {
		t.Errorf("DefaultReadDeadline = %v, want 30s: a change to a public constant is a breaking change", lang.DefaultReadDeadline)
	}
	limit := lang.DefaultMaxFileBytes
	if limit != 2*1024*1024 {
		t.Errorf("DefaultMaxFileBytes = %d, want 2 MiB", limit)
	}
	if internal := langroute.MaxExtractSize; limit != internal {
		t.Errorf("DefaultMaxFileBytes = %d, but the product's limit is %d", limit, internal)
	}
}

package stackframe

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

// fixture is testdata/frames.json, TL-1's one shared frame fixture: the
// localize tests (internal/mcp) and the prompt-hook test (cmd/fathom) read
// the same file.
type fixture struct {
	Sentinel string `json:"sentinel"`
	Cases    []struct {
		Name   string  `json:"name"`
		Shape  string  `json:"shape"`
		Text   string  `json:"text"`
		Frames []Frame `json:"frames"`
	} `json:"cases"`
	NotIssueShaped []string `json:"not_issue_shaped"`
}

func loadFixture(t *testing.T) fixture {
	t.Helper()
	b, err := os.ReadFile("testdata/frames.json")
	if err != nil {
		t.Fatal(err)
	}
	var f fixture
	if err := json.Unmarshal(b, &f); err != nil {
		t.Fatal(err)
	}
	if len(f.Cases) == 0 || f.Sentinel == "" {
		t.Fatal("fixture has no cases or no sentinel")
	}
	return f
}

// TestExtractReadsEveryFixtureShape is TL-1's parser contract: every raw
// grammar (PHP, V8, Java with its package path, Python, Go) and every
// payload shape (Bugsnag, OTLP, Lambda, New Relic) yields the fixture's
// frame tuples, depth 0 the throw site.
func TestExtractReadsEveryFixtureShape(t *testing.T) {
	f := loadFixture(t)
	for _, c := range f.Cases {
		t.Run(c.Name, func(t *testing.T) {
			got := Extract(c.Text)
			if !reflect.DeepEqual(got.Frames, c.Frames) {
				gb, _ := json.MarshalIndent(got.Frames, "", " ")
				wb, _ := json.MarshalIndent(c.Frames, "", " ")
				t.Errorf("frames:\n got %s\nwant %s", gb, wb)
			}
			wantPayload := c.Shape != "raw"
			if got.Payload != wantPayload {
				t.Errorf("Payload = %v, want %v", got.Payload, wantPayload)
			}
			if wantPayload && !containsString(got.Shapes, c.Shape) {
				t.Errorf("Shapes = %v, want it to name %q", got.Shapes, c.Shape)
			}
		})
	}
}

// TestExtractNeverCarriesMessageText: frame tuples only (ADR-058 d3). The
// sentinel sits in every message, breadcrumb, user and metaData field of
// the fixture; no extracted frame may carry it.
func TestExtractNeverCarriesMessageText(t *testing.T) {
	f := loadFixture(t)
	for _, c := range f.Cases {
		got := Extract(c.Text)
		b, _ := json.Marshal(got)
		if strings.Contains(string(b), f.Sentinel) {
			t.Errorf("%s: extraction carries message text: %s", c.Name, b)
		}
	}
}

// TestIssueShaped: the fixture's traces and payloads are issue-shaped;
// prose, a config object and a single prose file:line are not.
func TestIssueShaped(t *testing.T) {
	f := loadFixture(t)
	for _, c := range f.Cases {
		if c.Shape == "raw" {
			if n := CountFrameLines(c.Text, 2); n < 2 && !strings.Contains(c.Text, "Traceback") {
				t.Errorf("%s: CountFrameLines = %d, want >= 2", c.Name, n)
			}
			continue
		}
		if !IsPayload(c.Text) {
			t.Errorf("%s: IsPayload = false, want true", c.Name)
		}
	}
	for _, s := range f.NotIssueShaped {
		if IsPayload(s) || CountFrameLines(s, 2) >= 2 {
			t.Errorf("%q: read as issue-shaped", s)
		}
	}
}

func TestShortName(t *testing.T) {
	for in, want := range map[string]string{
		"com.acme.billing.Util.round":  "round",
		"App\\Billing\\Invoice->total": "total",
		"Cart::applyCoupon":            "applyCoupon",
		"Object.handle":                "handle",
		"new Widget":                   "Widget",
		"async loadAll":                "loadAll",
		"Object.<anonymous>":           "",
		"<module>":                     "",
		"main.lookup":                  "lookup",
		"pkg.(*T).Method":              "Method",
		"parseOrder":                   "parseOrder",
	} {
		if got := ShortName(in); got != want {
			t.Errorf("ShortName(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestJavaPathHintPrefersPackageDirectory: the package names the directory
// (ADR-062's Java fix); a nested class and a lambda keep the outer file.
func TestJavaPathHintPrefersPackageDirectory(t *testing.T) {
	got := Extract("\tat com.acme.a.Util$Inner.lambda$run$0(Util.java:7)\n\tat com.acme.b.Svc.go(Svc.java:3)").Frames
	if len(got) != 2 || got[0].PathHint != "com/acme/a/Util.java" || got[1].PathHint != "com/acme/b/Svc.java" {
		t.Errorf("hints = %+v", got)
	}
}

func TestExtractDeterministic(t *testing.T) {
	f := loadFixture(t)
	for _, c := range f.Cases {
		a, b := Extract(c.Text), Extract(c.Text)
		if !reflect.DeepEqual(a, b) {
			t.Errorf("%s: not deterministic", c.Name)
		}
	}
}

func containsString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// TestRegistryDecodes pins the embedded payload-shape registry.
func TestRegistryDecodes(t *testing.T) {
	r := loadRegistry()
	if len(r.FrameArrays) == 0 || len(r.TraceTexts) == 0 || len(r.CodeAttributes) == 0 || len(r.NeverRead) == 0 {
		t.Errorf("registry incomplete: %+v", r)
	}
}

// TestUnregisteredJSONIsAPayloadWithNoFrames: a JSON value no registered
// shape matches is never read as a trace (its fields may be message text).
func TestUnregisteredJSONIsAPayloadWithNoFrames(t *testing.T) {
	r, ok := ExtractPayload(`{"level":"error","msg":"boom at handler.go:12","caller":"x.go:3"}`)
	if !ok || !r.Payload || len(r.Frames) != 0 || len(r.Shapes) != 0 {
		t.Errorf("got %+v ok=%v, want a payload with no frames and no shape", r, ok)
	}
	if IsPayload(`{"level":"error"}`) {
		t.Error("IsPayload true for a frameless JSON value")
	}
}

// TestNeverReadSubtreesAreSkipped: a registered key planted inside
// metaData is not read.
func TestNeverReadSubtreesAreSkipped(t *testing.T) {
	r, _ := ExtractPayload(`{"events":[{"metaData":{"x":{"stacktrace":[{"file":"evil.js","lineNumber":1,"method":"m"}]}},"exceptions":[{"stacktrace":[{"file":"a.js","lineNumber":2,"method":"f"}]}]}]}`)
	if len(r.Frames) != 1 || r.Frames[0].File != "a.js" {
		t.Errorf("frames = %+v, want only a.js", r.Frames)
	}
}

// TestCausesReadRootCauseFirst: Bugsnag lists the error first and its
// causes after; the Lambda Java runtime nests the cause. Either way the
// root cause's frames come first.
func TestCausesReadRootCauseFirst(t *testing.T) {
	bugsnag := `{"events":[{"exceptions":[{"errorClass":"Outer","stacktrace":[{"file":"Cart.java","lineNumber":24,"method":"com.acme.Cart.checkout"}]},{"errorClass":"Inner","stacktrace":[{"file":"Order.java","lineNumber":9,"method":"com.acme.Order.reserve"}]}]}]}`
	lambda := `{"errorType":"Outer","stackTrace":["com.acme.Cart.checkout(Cart.java:24)"],"cause":{"errorType":"Inner","stackTrace":["com.acme.Order.reserve(Order.java:9)"]}}`
	for name, text := range map[string]string{"bugsnag": bugsnag, "lambda": lambda} {
		fr := Extract(text).Frames
		if len(fr) != 2 || fr[0].File != "Order.java" || fr[1].File != "Cart.java" {
			t.Errorf("%s: frames = %+v, want Order.java (the root cause) first", name, fr)
		}
	}
}

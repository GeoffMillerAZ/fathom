package mcp_test

// localize_logtemplate_test.go — TL-4 (F67, ADR-064): a pasted log line,
// given as localize's `error`, seeds the logger call whose template wrote
// it (a typed log_template seed), the call's enclosing symbol ranks first
// above the literal-grep hop, the longest coverage wins deterministically,
// a trace's throw site still outranks it, and nothing of the line beyond
// the matched template's text reaches a hop, a why-line, the answer or the
// call log (a sentinel in a placeholder position, ADR-058 decision 3).

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/core"
	"github.com/geoffmilleraz/fathom/internal/logtemplate"
	"github.com/geoffmilleraz/fathom/internal/mcp"
)

const ltSentinel = "SENTINEL_tl4_9f3a"

// ltSources is the fixture service: code symbols plus the templates the
// reader finds in these files, as ingest stores them.
var ltSources = map[string]string{
	"billing/invoice.py": `import logging
logger = logging.getLogger(__name__)


class Invoice:
    def total(self, n):
        logger.error("invoice %s total %d over limit", self.id, n)
        return n

    def refund(self, rid):
        logger.warning("order %s failed", rid)

    def retry(self, rid, why):
        logger.warning("order %s failed: %s", rid, why)
`,
	"orders/cart.py": `import structlog
log = structlog.get_logger()


def checkout(order_id, reason):
    log.error("payment_failed", order_id=order_id, reason=reason)
    lookup("A-17")
    lookup("A-17")


def lookup(code):
    return code
`,
	"src/main/java/com/acme/Charge.java": `package com.acme;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
public class Charge {
  private static final Logger log = LoggerFactory.getLogger(Charge.class);
  public void decline(String id, String user) {
    log.warn("charge {} declined for {}", id, user);
  }
}
`,
	"src/ship.js": `const logger = require('./log');
function ship(id) {
  logger.info({ id }, 'parcel %s left the depot', id);
}
`,
	"src/Auth.php": `<?php
class Auth {
    public function verify($token) {
        $this->logger->notice('token {token} expired', ['token' => $token]);
    }
}
`,
	"orders/lookup_users.py": `def users():
    a = "A-17"
    b = "A-17"
    return a + b
`,
}

// ltNodes are the code symbols of ltSources, as an extractor names them.
var ltNodes = []core.SymbolNode{
	{Service: "svc", Package: "invoice", Name: "Invoice", Kind: core.KindPyClass, FilePath: "billing/invoice.py", Line: 5, EndLine: 14},
	{Service: "svc", Package: "invoice", Name: "Invoice.total", Kind: core.KindPyFunc, FilePath: "billing/invoice.py", Line: 6, EndLine: 8},
	{Service: "svc", Package: "invoice", Name: "Invoice.refund", Kind: core.KindPyFunc, FilePath: "billing/invoice.py", Line: 10, EndLine: 11},
	{Service: "svc", Package: "invoice", Name: "Invoice.retry", Kind: core.KindPyFunc, FilePath: "billing/invoice.py", Line: 13, EndLine: 14},
	{Service: "svc", Package: "cart", Name: "checkout", Kind: core.KindPyFunc, FilePath: "orders/cart.py", Line: 5, EndLine: 8, Calls: []string{"lookup"}},
	{Service: "svc", Package: "cart", Name: "lookup", Kind: core.KindPyFunc, FilePath: "orders/cart.py", Line: 11, EndLine: 12},
	{Service: "svc", Package: "com.acme", Name: "Charge", Kind: core.KindJavaClass, FilePath: "src/main/java/com/acme/Charge.java", Line: 4, EndLine: 9},
	{Service: "svc", Package: "com.acme", Name: "Charge.decline", Kind: core.KindJavaMethod, FilePath: "src/main/java/com/acme/Charge.java", Line: 6, EndLine: 8},
	{Service: "svc", Package: "ship", Name: "ship", Kind: core.KindJSFunc, FilePath: "src/ship.js", Line: 2, EndLine: 4},
	{Service: "svc", Package: "", Name: "Auth", Kind: "PhpClass", FilePath: "src/Auth.php", Line: 2, EndLine: 6},
	{Service: "svc", Package: "", Name: "Auth.verify", Kind: "PhpMethod", FilePath: "src/Auth.php", Line: 3, EndLine: 5},
	{Service: "svc", Package: "lookup_users", Name: "users", Kind: core.KindPyFunc, FilePath: "orders/lookup_users.py", Line: 1, EndLine: 4},
}

// logTemplateBackend writes ltSources to disk (the literal hop greps them)
// and indexes their symbols plus their templates, as ingest stores them.
func logTemplateBackend(t *testing.T) *localizeFakeBackend {
	t.Helper()
	root := t.TempDir()
	nodes := append([]core.SymbolNode(nil), ltNodes...)
	var files []string
	for f := range ltSources {
		files = append(files, f)
	}
	for _, f := range files {
		p := filepath.Join(root, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(ltSources[f]), 0o644); err != nil {
			t.Fatal(err)
		}
		facts := logtemplate.Read(f, []byte(ltSources[f]))
		logtemplate.Enclose(facts, ltNodes)
		for _, fact := range facts {
			nodes = append(nodes, logtemplate.ToObservation("svc", fact, "h", time.Unix(0, 0), false).ToSymbolNode())
		}
	}
	return &localizeFakeBackend{nodes: nodes, svcPath: root, indexCommit: "c1"}
}

func topCandidate(t *testing.T, got map[string]any) (string, string) {
	t.Helper()
	cands, _ := got["candidates"].([]any)
	if len(cands) == 0 {
		t.Fatalf("no candidates: %v", got)
	}
	top := cands[0].(map[string]any)
	return top["file"].(string), top["symbol"].(string)
}

// TestLocalize_LogLineSeedsItsTemplate: each logger API's rendered line,
// pasted as `error`, seeds a log_template hop naming the template and its
// call site, and the call's enclosing symbol is the top candidate.
func TestLocalize_LogLineSeedsItsTemplate(t *testing.T) {
	cases := []struct{ name, line, file, symbol, template string }{
		{"python logging", "2026-10-09 12:00:00,123 ERROR [billing.invoice] invoice INV-7 total 300 over limit", "billing/invoice.py", "Invoice.total", "invoice %s total %d over limit"},
		{"structlog console", "2026-10-09 12:00:00 [error    ] payment_failed                 order_id=17 reason='card'", "orders/cart.py", "checkout", "payment_failed"},
		{"structlog json", `{"order_id": 17, "reason": "card", "event": "payment_failed", "level": "error"}`, "orders/cart.py", "checkout", "payment_failed"},
		{"slf4j", "2026-10-09 12:00:00.123 WARN  [main] com.acme.Charge - charge C-9 declined for bob", "src/main/java/com/acme/Charge.java", "Charge.decline", "charge {} declined for {}"},
		{"pino", `{"level":30,"time":1760000000000,"pid":7,"hostname":"h","id":"P-1","msg":"parcel P-1 left the depot"}`, "src/ship.js", "ship", "parcel %s left the depot"},
		{"monolog", `[2026-10-09T12:00:00+00:00] app.NOTICE: token t-77 expired {"token":"t-77"} []`, "src/Auth.php", "Auth.verify", "token {token} expired"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := mcp.NewServer(logTemplateBackend(t))
			got := localizeCall(t, srv, map[string]any{"service": "svc", "error": c.line})
			if f, s := topCandidate(t, got); f != c.file || s != c.symbol {
				t.Errorf("top = %s %s, want %s %s; %v", f, s, c.file, c.symbol, got["candidates"])
			}
			hops, _ := got["hops"].([]any)
			if len(hops) == 0 || hops[0].(map[string]any)["tool"] != "log_template" || !strings.Contains(hops[0].(map[string]any)["arg"].(string), c.template) {
				t.Errorf("first hop = %v, want a log_template hop citing %q", hops, c.template)
			}
			lt, _ := got["log_template"].(map[string]any)
			if lt == nil || lt["lines"] != float64(1) {
				t.Errorf("log_template disclosure = %v, want lines 1", got["log_template"])
			}
		})
	}
}

// TestLocalize_LogTemplateOutranksTheLiteralHop: an unmatched line beside
// the log line quotes a literal that greps to other symbols; the template's
// call still ranks first.
func TestLocalize_LogTemplateOutranksTheLiteralHop(t *testing.T) {
	srv := mcp.NewServer(logTemplateBackend(t))
	text := "WARNING order A-17 failed\ncaused by: lookup \"A-17\" returned nothing"
	got := localizeCall(t, srv, map[string]any{"service": "svc", "error": text})
	if f, s := topCandidate(t, got); f != "billing/invoice.py" || s != "Invoice.refund" {
		t.Errorf("top = %s %s, want billing/invoice.py Invoice.refund; %v", f, s, got["candidates"])
	}
	var tools []string
	for _, h := range got["hops"].([]any) {
		tools = append(tools, h.(map[string]any)["tool"].(string))
	}
	if !containsStr(tools, "search") {
		t.Errorf("the unmatched line's literal was not seeded: hops %v", tools)
	}
}

// TestLocalize_LogTemplateLongestCoverageWins: of two templates a line
// matches, the one covering more of it wins; the shorter line matches only
// the shorter template; the answer is the same every time.
func TestLocalize_LogTemplateLongestCoverageWins(t *testing.T) {
	be := logTemplateBackend(t)
	for line, want := range map[string]string{
		"WARNING:billing.invoice:order O-1 failed: card expired": "Invoice.retry",
		"WARNING:billing.invoice:order O-1 failed":               "Invoice.refund",
	} {
		var first []byte
		for i := 0; i < 3; i++ {
			got := localizeCall(t, mcp.NewServer(be), map[string]any{"service": "svc", "error": line})
			if _, s := topCandidate(t, got); s != want {
				t.Errorf("%q: top = %s, want %s", line, s, want)
			}
			b, _ := json.Marshal(got)
			if first == nil {
				first = b
			} else if !bytes.Equal(first, b) {
				t.Errorf("%q: answer differs between calls", line)
			}
		}
	}
}

// TestLocalize_TraceThrowSiteStillOutranksALogLine: a log line pasted with
// the trace that followed it: the throw site stays first.
func TestLocalize_TraceThrowSiteStillOutranksALogLine(t *testing.T) {
	srv := mcp.NewServer(logTemplateBackend(t))
	text := "2026-10-09 12:00:00 ERROR invoice INV-7 total 300 over limit\n" +
		"Traceback (most recent call last):\n" +
		"  File \"/app/orders/cart.py\", line 6, in checkout\n" +
		"    lookup(\"A-17\")\n" +
		"  File \"/app/orders/cart.py\", line 12, in lookup\n" +
		"    return code\n" +
		"KeyError: 'A-17'"
	got := localizeCall(t, srv, map[string]any{"service": "svc", "error": text})
	if f, s := topCandidate(t, got); f != "orders/cart.py" || s != "lookup" {
		t.Errorf("top = %s %s, want the throw site orders/cart.py lookup; %v", f, s, got["candidates"])
	}
}

// TestLocalize_LogLineSentinelNeverEchoed: a sentinel in a placeholder
// position (bare, quoted, inside a JSON line) reaches no hop, why-line,
// summary, answer or call-log row.
func TestLocalize_LogLineSentinelNeverEchoed(t *testing.T) {
	lines := []string{
		"ERROR invoice " + ltSentinel + " total 300 over limit",
		`WARNING order "` + ltSentinel + `" failed: "` + ltSentinel + `(x)"`,
		"2026-10-09 [error    ] payment_failed  order_id=" + ltSentinel + " reason='" + ltSentinel + "'",
		"WARN com.acme.Charge - charge " + ltSentinel + " declined for " + ltSentinel + ".py:12",
		`{"level":30,"id":"` + ltSentinel + `","msg":"parcel ` + ltSentinel + ` left the depot"}`,
		`[2026-10-09T12:00:00+00:00] app.NOTICE: token ` + ltSentinel + ` expired {"token":"` + ltSentinel + `"} []`,
	}
	for _, line := range lines {
		var calls bytes.Buffer
		srv := mcp.NewServerWithLogger(logTemplateBackend(t), mcp.NewCallLogger(&calls))
		got := localizeCall(t, srv, map[string]any{"service": "svc", "error": line})
		if got["log_template"] == nil {
			t.Errorf("%q: no template matched: %v", line, got)
		}
		b, _ := json.Marshal(got)
		if strings.Contains(string(b), ltSentinel) {
			t.Errorf("answer echoes the pasted line: %s", b)
		}
		if strings.Contains(calls.String(), ltSentinel) {
			t.Errorf("call log echoes the pasted line: %s", calls.String())
		}
	}
}

// TestLocalize_ShapedPayloadIsNeverReadAsALogLine: a registered payload's
// message field carrying a template's rendering seeds no template (TL-1:
// frame tuples only).
func TestLocalize_ShapedPayloadIsNeverReadAsALogLine(t *testing.T) {
	srv := mcp.NewServer(logTemplateBackend(t))
	payload := `{"events":[{"exceptions":[{"errorClass":"E","message":"order X-1 failed: boom","stacktrace":[{"file":"orders/cart.py","lineNumber":12,"method":"lookup","inProject":true}]}]}]}`
	got := localizeCall(t, srv, map[string]any{"service": "svc", "error": payload})
	if got["log_template"] != nil {
		t.Errorf("a registered payload was read as a log line: %v", got["log_template"])
	}
	if f, s := topCandidate(t, got); f != "orders/cart.py" || s != "lookup" {
		t.Errorf("top = %s %s, want the payload's throw site", f, s)
	}
}

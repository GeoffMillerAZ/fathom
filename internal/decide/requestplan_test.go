package decide_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/geoffmilleraz/fathom/internal/decide"
	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

// refuseTransport fails every request without opening a socket: a test
// whose provider is a scripted fake must never reach the adapter's wire.
type refuseTransport struct{}

func (refuseTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("refuseTransport: no request may leave this test")
}

// planAnswer is a scripted answer to one predicate question.
func planAnswer(id string) decision.MockEntry {
	return decision.MockEntry{Response: decision.Response{
		Provider: "typesafe", Model: "jev-test",
		Answers: map[string]decision.Answer{id: {ID: id, Kind: decision.KindPredicate, Noul: 0.9, Confidence: 0.9}},
	}}
}

// planPort opens a configured port whose provider is fake, at site; every
// ask the plan lets through reaches fake and nothing else.
func planPort(t *testing.T, user, project string, disclose, log io.Writer, fake decision.Decider, site string) decision.Decider {
	t.Helper()
	cfg := loadDecide(t, user, project)
	rec := decide.NewRecorder(log, func() time.Time { return time.Unix(0, 0).UTC() }, "test")
	ports, err := decide.Open(cfg, nil, "", rec,
		decide.WithKeyIO(newKeyCounter(map[string]string{"FATHOM_TEST_JF21_KEY": testKey}).io()),
		decide.WithHTTPClient(&http.Client{Transport: refuseTransport{}}), decide.WithDisclose(disclose), decide.WithUserConfig(""),
		decide.CalibratedFor(t, "typesafe", "jev-test", append([]string{site}, decide.AllSites...)...))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	d := ports.Gated(func(decision.Decider) decision.Decider { return fake })
	if site != "" {
		d = decide.AtSite(d, site)
	}
	return d
}

// TestRequestPlanCarriesOnlyDeclaredSpansRedacted is JF-21 (I61): every
// ask's request leaves through the request plan in the port's wrapper
// chain. It keeps only the fields the asking site declares, redacts
// secret-shaped strings with llmsec's extended catalog and counts them on
// the decision row, sends nothing from a path a [decide] exclude glob
// names (the ask degrades with one `excluded` line), prints exactly the
// bytes that would leave under FATHOM_DECIDE_PLAN_ONLY=1 and sends
// nothing, and leaves provider none as it was.
func TestRequestPlanCarriesOnlyDeclaredSpansRedacted(t *testing.T) {
	unsetProviderKeys(t)
	t.Setenv("HOME", t.TempDir())
	isolateKill(t)
	t.Setenv(decide.PlanOnlyEnv, "")
	if err := os.Unsetenv(decide.PlanOnlyEnv); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FATHOM_TEST_JF21_KEY", testKey)
	user := "[decide]\nprovider = \"typesafe\"\napi_key_env = \"FATHOM_TEST_JF21_KEY\"\nexclude = [\"secrets/**\"]\n"
	project := "[decide]\nexclude = [\"internal/vault/**\", \"*.pem\"]\n"

	// Secret-shaped, not secrets: the AWS shape is AKIA and 16 uppercase
	// alphanumerics, the PEM shape a private-key block.
	aws := "AKIA" + "QX7TR2MZ9WLC4JDB"
	pem := "-----BEGIN RSA " + "PRIVATE KEY-----\nMIIEowIBAAKCAQEAnotakeyatall\n-----END RSA " + "PRIVATE KEY-----"
	spanRequest := func() decision.Request {
		return decision.Request{
			State: "file: internal/store/store.go\n\nhunk:\n+const key = \"" + aws + "\"\n+" + pem + "\n",
			Questions: []decision.Question{{
				ID: "q1", Kind: decision.KindPredicate,
				Instructions: "Does this rule govern the edit described in the state?",
				TrueCriteria: "Rule: never commit " + aws,
				// Fields obligations_rerank does not declare.
				Options: []decision.Option{{Name: "extra", Criteria: "an option a predicate never carries"}},
				Rubric:  "a rubric the site never sends",
			}},
			Images: []decision.Image{{MediaType: "image/png", Data: []byte{0x89, 'P', 'N', 'G'}}},
		}
	}

	t.Run("layers union their exclude globs", func(t *testing.T) {
		cfg := loadDecide(t, user, project)
		if want := []string{"*.pem", "internal/vault/**", "secrets/**"}; !slices.Equal(cfg.Exclude, want) {
			t.Fatalf("Exclude = %q, want %q: a repository adds globs and never removes the user's", cfg.Exclude, want)
		}
	})

	var sent []byte
	t.Run("only declared fields leave, secrets redacted and counted", func(t *testing.T) {
		var disclose, log bytes.Buffer
		fake := decision.NewMockDecider("typesafe", planAnswer("q1"))
		d := planPort(t, user, project, &disclose, &log, fake, decide.SiteObligationsRerank)
		resp, err := d.Decide(context.Background(), spanRequest())
		if err != nil {
			t.Fatalf("Decide: %v", err)
		}
		calls := fake.Calls()
		if len(calls) != 1 {
			t.Fatalf("the provider got %d requests, want 1", len(calls))
		}
		got := calls[0]
		for _, s := range []string{aws, "PRIVATE KEY", "MIIEow"} {
			if strings.Contains(got.State, s) || strings.Contains(got.Questions[0].TrueCriteria, s) {
				t.Errorf("the request left with %q unredacted:\n%s", s, got.State)
			}
		}
		if !strings.Contains(got.State, "[REDACTED:") || !strings.Contains(got.State, "file: internal/store/store.go") {
			t.Errorf("the state lost its declared span or its redaction labels:\n%s", got.State)
		}
		q := got.Questions[0]
		if len(q.Options) != 0 || q.Rubric != "" || len(got.Images) != 0 {
			t.Errorf("undeclared fields left: options %v, rubric %q, %d images", q.Options, q.Rubric, len(got.Images))
		}
		if q.Instructions == "" || q.ID != "q1" || q.Kind != decision.KindPredicate {
			t.Errorf("a declared field was dropped: %+v", q)
		}
		if _, ok := resp.Answers["q1"]; !ok {
			t.Errorf("the answer did not come back: %+v", resp)
		}
		sent, err = json.Marshal(got)
		if err != nil {
			t.Fatal(err)
		}
		if disclose.Len() != 0 {
			t.Errorf("an ask the plan let through wrote a line: %q", disclose.String())
		}
	})

	t.Run("the decision row counts the redactions", func(t *testing.T) {
		var disclose, log bytes.Buffer
		cfg := loadDecide(t, user, project)
		rec := decide.NewRecorder(&log, func() time.Time { return time.Unix(0, 0).UTC() }, "test")
		ports, err := decide.Open(cfg, nil, "", rec,
			decide.WithKeyIO(newKeyCounter(map[string]string{"FATHOM_TEST_JF21_KEY": testKey}).io()),
			decide.WithHTTPClient(&http.Client{Transport: refuseTransport{}}), decide.WithDisclose(&disclose), decide.WithUserConfig(""),
			decide.CalibratedFor(t, "typesafe", "jev-test", decide.AllSites...))
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		fake := decision.NewMockDecider("typesafe", planAnswer("q1"), planAnswer("q1"))
		d := decide.AtSite(ports.Gated(func(decision.Decider) decision.Decider { return fake }), decide.SiteObligationsRerank)
		resp, err := d.Decide(context.Background(), spanRequest())
		if err != nil {
			t.Fatalf("Decide: %v", err)
		}
		rec.Record(decide.SiteObligationsRerank, resp, nil)
		var row struct {
			QuestionID string `json:"question_id"`
			Redactions int    `json:"redactions"`
		}
		if err := json.Unmarshal(bytes.TrimSpace(log.Bytes()), &row); err != nil {
			t.Fatalf("decision row: %v\n%s", err, log.String())
		}
		if row.QuestionID != "q1" || row.Redactions != 3 {
			t.Fatalf("row = %+v, want q1 with 3 redactions (the AWS shape twice, the PEM block once)", row)
		}
		// A clean ask's row carries no count: rows stay byte-identical.
		log.Reset()
		clean := spanRequest()
		clean.State, clean.Questions[0].TrueCriteria = "file: internal/store/store.go", "Rule: keep it small"
		resp, err = d.Decide(context.Background(), clean)
		if err != nil {
			t.Fatalf("Decide: %v", err)
		}
		rec.Record(decide.SiteObligationsRerank, resp, nil)
		if strings.Contains(log.String(), "redactions") {
			t.Fatalf("a clean ask's row names redactions: %s", log.String())
		}
	})

	t.Run("an excluded path's span never reaches the provider", func(t *testing.T) {
		for _, tc := range []struct{ name, site, state string }{
			{"the repository's glob, an edit hunk", decide.SiteObligationsRerank, "file: internal/vault/keys.go\n\nhunk:\n+x := 1"},
			{"the user's glob, a test's source with its line", "", "=== TestToken (secrets/token_test.go:12) ===\nfunc TestToken(t *testing.T) {}"},
			{"a git diff header", "", "diff --git a/internal/vault/k.go b/internal/vault/k.go\n+y"},
			{"a slashless glob at any depth", decide.SiteCompilePrefilter, "id: s1\ndoc: certs/dev/server.pem\ntext:\nnot a key"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				var disclose, log bytes.Buffer
				fake := decision.NewMockDecider("typesafe", planAnswer("q1"))
				d := planPort(t, user, project, &disclose, &log, fake, tc.site)
				req := decision.Request{State: tc.state, Questions: []decision.Question{{ID: "q1", Kind: decision.KindPredicate, Instructions: "Does the rule govern the edit?"}}}
				for range 2 {
					_, err := d.Decide(context.Background(), req)
					var de *decide.DegradedError
					if !errors.As(err, &de) || !errors.Is(err, decision.ErrNoProvider) || decide.MissClass(err) != "excluded" {
						t.Fatalf("Decide err = %v (class %q), want a *DegradedError of class excluded", err, decide.MissClass(err))
					}
				}
				if n := len(fake.Calls()); n != 0 {
					t.Fatalf("the provider got %d requests from an excluded path, want 0", n)
				}
				if n := strings.Count(disclose.String(), "decision port degraded (excluded)"); n != 1 {
					t.Fatalf("two excluded asks wrote %d degrade lines, want 1:\n%s", n, disclose.String())
				}
				if !strings.Contains(disclose.String(), "[decide] exclude") {
					t.Fatalf("the degrade line does not name [decide] exclude: %s", disclose.String())
				}
			})
		}
	})

	t.Run("plan only prints exactly the bytes that would leave and sends nothing", func(t *testing.T) {
		if sent == nil {
			t.Skip("the normal ask failed")
		}
		t.Setenv(decide.PlanOnlyEnv, "1")
		var disclose, log bytes.Buffer
		fake := decision.NewMockDecider("typesafe", planAnswer("q1"))
		d := planPort(t, user, project, &disclose, &log, fake, decide.SiteObligationsRerank)
		_, err := d.Decide(context.Background(), spanRequest())
		if !errors.Is(err, decision.ErrNoProvider) || decide.MissClass(err) != "plan only" {
			t.Fatalf("Decide err = %v (class %q), want the deterministic path, class plan only", err, decide.MissClass(err))
		}
		if n := len(fake.Calls()); n != 0 {
			t.Fatalf("plan only sent %d requests, want 0", n)
		}
		out := disclose.String()
		plan, rest, _ := strings.Cut(out, "\n")
		if plan != string(sent) {
			t.Fatalf("plan-only output differs from the bytes a normal ask sends:\nplan: %s\nsent: %s", plan, sent)
		}
		if strings.Count(rest, "\n") != 1 || !strings.Contains(rest, "decision port degraded (plan only)") {
			t.Fatalf("plan only wrote %q after the plan, want one degrade line", rest)
		}
	})

	t.Run("provider none is unchanged", func(t *testing.T) {
		t.Setenv(decide.PlanOnlyEnv, "1")
		cfg := loadDecide(t, "[decide]\nprovider = \"none\"\nexclude = [\"secrets/**\"]\n", "")
		var disclose bytes.Buffer
		ports, err := decide.Open(cfg, nil, "", nil, decide.WithDisclose(&disclose), decide.WithUserConfig(""))
		if err != nil {
			t.Fatalf("Open: %v", err)
		}
		if _, isNone := ports.Decider.(decision.None); !isNone {
			t.Fatalf("provider none opened %T, want decision.None", ports.Decider)
		}
		if _, err := ports.Decider.Decide(context.Background(), spanRequest()); !errors.Is(err, decision.ErrNoProvider) {
			t.Fatalf("none answered %v, want ErrNoProvider", err)
		}
		if disclose.Len() != 0 {
			t.Fatalf("provider none wrote %q, want nothing", disclose.String())
		}
	})
}

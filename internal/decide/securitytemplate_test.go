package decide_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/decide"
	decision "github.com/geoffmilleraz/fathom/third_party/agentic-kit/foundations/decision"
)

// TestSecurityPropertiesNeverJudgedForServing is JF-20 (I62, R7): a site
// or a question template that names a security property is refused at the
// ask, before any provider sees it, with one disclosed `security property`
// line per session and site, and the site answers on its deterministic
// path. A security word that is only data (a path, the state, a rule's
// text in the criteria) is not a template naming the property.
func TestSecurityPropertiesNeverJudgedForServing(t *testing.T) {
	unsetProviderKeys(t)
	t.Setenv("HOME", t.TempDir())
	isolateKill(t)
	t.Setenv(decide.PlanOnlyEnv, "")
	t.Setenv("FATHOM_TEST_JF21_KEY", testKey)
	user := "[decide]\nprovider = \"typesafe\"\napi_key_env = \"FATHOM_TEST_JF21_KEY\"\n"
	predicate := func(instructions string) decision.Question {
		return decision.Question{ID: "q1", Kind: decision.KindPredicate, Instructions: instructions}
	}

	for _, tc := range []struct {
		name, site string
		q          decision.Question
	}{
		{"an authorization question", decide.SiteObligationsRerank, predicate("Does this handler enforce authorization before it writes?")},
		{"a secure-or-not question", "", predicate("The change keeps the connection secure.")},
		{"an injection question, any case", decide.SiteCompilePrefilter, predicate("Is this query open to SQL Injection?")},
		{"a rubric naming the property", "", decision.Question{ID: "q1", Kind: decision.KindScore, Instructions: "Rate the change.", Levels: []string{"low", "high"}, Rubric: "How vulnerable is the code to an exploit?"}},
		{"an answer named for the property", decide.SiteRoute, decision.Question{ID: "q1", Kind: decision.KindChoice, Instructions: "Which fits?", Options: []decision.Option{{Name: "sanitized"}, {Name: "none"}}}},
		{"a site named for the property", "judge_access_control", predicate("Does the rule govern the edit?")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var disclose, log bytes.Buffer
			fake := decision.NewMockDecider("typesafe", planAnswer("q1"))
			d := planPort(t, user, "", &disclose, &log, fake, tc.site)
			req := decision.Request{State: "file: internal/store/store.go", Questions: []decision.Question{tc.q}}
			for range 2 {
				_, err := d.Decide(context.Background(), req)
				var de *decide.DegradedError
				if !errors.As(err, &de) || !errors.Is(err, decision.ErrNoProvider) || decide.MissClass(err) != "security property" {
					t.Fatalf("Decide err = %v (class %q), want a *DegradedError of class security property", err, decide.MissClass(err))
				}
			}
			if n := len(fake.Calls()); n != 0 {
				t.Fatalf("a security property reached the provider %d times, want 0", n)
			}
			if n := strings.Count(disclose.String(), "decision port degraded (security property)"); n != 1 {
				t.Fatalf("two refused asks wrote %d lines, want 1:\n%s", n, disclose.String())
			}
		})
	}

	t.Run("a security word that is only data is asked", func(t *testing.T) {
		var disclose, log bytes.Buffer
		fake := decision.NewMockDecider("typesafe", planAnswer("q1"))
		d := planPort(t, user, "", &disclose, &log, fake, decide.SiteObligationsRerank)
		req := decision.Request{
			State: "file: internal/auth/authorization.go\n\nhunk:\n+checkAuthorization(ctx)",
			Questions: []decision.Question{{
				ID: "q1", Kind: decision.KindPredicate,
				Instructions: "Did the change to internal/auth/authorization.go follow the guidance?",
				TrueCriteria: "Rule: every handler checks authorization first\nScope: internal/auth/**",
			}},
		}
		if _, err := d.Decide(context.Background(), req); err != nil {
			t.Fatalf("Decide: %v (a path or a rule's text is data, not the template)", err)
		}
		if n := len(fake.Calls()); n != 1 || disclose.Len() != 0 {
			t.Fatalf("%d requests and lines %q, want 1 request and no line", n, disclose.String())
		}
	})
}

package decisiontest_test

import (
	"context"
	"errors"
	"math"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"

	decision "github.com/geoffmilleraz/agentic-kit/foundations/decision"
	"github.com/geoffmilleraz/agentic-kit/foundations/decision/decisiontest"
)

// plantedKey is the fake credential the planted Decider holds.
const plantedKey = "decisiontest-planted-credential"

// defectEnv names the defect a re-executed child test plants.
const defectEnv = "DECISIONTEST_PLANTED_DEFECT"

// planted is a Decider over a Script that conforms unless defect names one
// way to break the contract.
type planted struct {
	mu     sync.Mutex
	s      decisiontest.Script
	calls  int
	defect string
}

func (p *planted) Name() string { return "planted" }

func (p *planted) Capabilities() decision.Capabilities {
	return decision.Capabilities{
		Declared:            true,
		Kinds:               []decision.Kind{decision.KindChoice, decision.KindScore, decision.KindPredicate},
		SingleRoundTrip:     true,
		ConfidenceSemantics: decision.ConfidenceNativeProbability,
	}
}

func (p *planted) Decide(_ context.Context, req decision.Request) (decision.Response, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.defect == "calls-before-validate" {
		p.calls++
	}
	if err := decision.Validate(req); err != nil {
		return decision.Response{}, err
	}
	if p.defect != "calls-before-validate" {
		p.calls++
	}
	if p.s.Fail != "" {
		msg := "planted: provider failed"
		if p.defect == "leaks-key" {
			msg += " for key " + plantedKey
		}
		return decision.Response{}, errors.New(msg)
	}
	resp := decision.Response{Provider: p.Name(), Answers: map[string]decision.Answer{}, Invalid: map[string]string{}}
	byID := map[string]decisiontest.ScriptedAnswer{}
	for _, a := range p.s.Answers {
		byID[a.ID] = a
	}
	for _, q := range req.Questions {
		a, ok := byID[q.ID]
		if !ok {
			if p.defect == "fails-batch" {
				return decision.Response{}, errors.New("planted: a question went unanswered")
			}
			resp.Invalid[q.ID] = "no answer"
			continue
		}
		ans := decision.Answer{ID: q.ID, Kind: q.Kind}
		switch q.Kind {
		case decision.KindChoice:
			known := false
			for _, o := range q.Options {
				known = known || o.Name == a.Choice
			}
			if !known && p.defect != "accepts-bad-choice" {
				resp.Invalid[q.ID] = "choice is not an option"
				continue
			}
			ans.Choice, ans.Probabilities, ans.Confidence = a.Choice, a.Probabilities, decision.Confidence(a.Probabilities)
		case decision.KindScore:
			ans.Score, ans.Confidence = a.Score, decision.Confidence(nil)
		default:
			ans.Noul, ans.Confidence = a.Probability, math.Max(a.Probability, 1-a.Probability)
		}
		if p.defect == "trusts-self-reported-confidence" {
			ans.Confidence = a.SelfConfidence
		}
		resp.Answers[q.ID] = ans
	}
	return resp, nil
}

// plantedHarness builds a planted Decider with defect.
func plantedHarness(defect string) decisiontest.Harness {
	return decisiontest.Harness{
		New: func(_ *testing.T, s decisiontest.Script) decisiontest.Instance {
			p := &planted{s: s, defect: defect}
			return decisiontest.Instance{Decider: p, ProviderCalls: func() int {
				p.mu.Lock()
				defer p.mu.Unlock()
				return p.calls
			}}
		},
		APIKey:           plantedKey,
		RedactsEchoedKey: true,
	}
}

// A Decider that keeps the contract passes the suite, echo check included.
func TestRunConformancePassesAConformingDecider(t *testing.T) {
	decisiontest.RunConformance(t, plantedHarness(""))
}

// TestPlantedDefectChild runs the suite over one planted defect. It only
// runs as the child TestRunConformanceCatchesPlantedDefects re-executes,
// where it is expected to fail.
func TestPlantedDefectChild(t *testing.T) {
	defect := os.Getenv(defectEnv)
	if defect == "" {
		t.Skip("runs only as a re-executed child of TestRunConformanceCatchesPlantedDefects")
	}
	decisiontest.RunConformance(t, plantedHarness(defect))
}

// Each planted defect must fail the suite with the check that names it: a
// conformance suite that cannot catch a planted violation proves nothing.
func TestRunConformanceCatchesPlantedDefects(t *testing.T) {
	defects := []struct {
		defect, want string
	}{
		{"calls-before-validate", "an invalid Request never reaches the provider"},
		{"fails-batch", "one bad answer never fails the batch"},
		{"accepts-bad-choice", `bad answer "bad-choice-q" landed in Answers`},
		{"trusts-self-reported-confidence", "is the provider's self-reported value"},
		{"leaks-key", "key leak: an error contains the API key"},
	}
	for _, d := range defects {
		t.Run(d.defect, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=^TestPlantedDefectChild$", "-test.count=1")
			cmd.Env = append(os.Environ(), defectEnv+"="+d.defect)
			out, err := cmd.CombinedOutput()
			if err == nil {
				t.Fatalf("the suite passed a Decider planted with %q:\n%s", d.defect, out)
			}
			if !strings.Contains(string(out), d.want) {
				t.Errorf("the suite failed %q without the expected finding %q:\n%s", d.defect, d.want, out)
			}
		})
	}
}

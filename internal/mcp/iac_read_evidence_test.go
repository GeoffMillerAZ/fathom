package mcp_test

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/geoffmilleraz/fathom/internal/iacfacts"
)

func TestReviewDiffCarriesReadEvidence(t *testing.T) {
	t.Run("module", func(t *testing.T) {
		laneIaCSimpleHome(t)
		root := iacRepo(t, map[string]string{
			"main.tf":     iacProviders + "resource \"widget_thing\" \"a\" {\n name = upper(var.name)\n}\n",
			"defaults.tf": "variable \"name\" { default = \"product-default-canary\" }\n",
		})
		p, _ := iacCall(t, iacServer(root), "review_diff", map[string]any{"service": "svc", "changed_files": []string{"main.tf"}})
		laneIaCDimension(t, p, false)
		assertReviewEvidence(t, p, iacBlock(t, p), []string{"defaults.tf", "main.tf"}, "main.tf", iacfacts.DecidePlan)
	})
	t.Run("template", func(t *testing.T) {
		cfnHome(t, true)
		root := iacRepo(t, map[string]string{"stack.yaml": "AWSTemplateFormatVersion: \"2010-09-09\"\nResources:\n  Widget:\n    Type: Example::Widget::Thing\n    Properties:\n      Name: !Join [\"-\", [a, b]]\n"})
		p, _ := iacCall(t, cfnServer(root), "review_diff", map[string]any{"service": "svc", "changed_files": []string{"stack.yaml"}})
		laneIaCDimension(t, p, true)
		assertReviewEvidence(t, p, iacBlock(t, p)["cloudformation"].(map[string]any), []string{"stack.yaml"}, "stack.yaml", iacfacts.DecideRender)
	})
}

func assertReviewEvidence(t *testing.T, p, section map[string]any, paths []string, file string, action iacfacts.DecidableBy) {
	t.Helper()
	raw, err := json.Marshal(section["evidence"])
	if err != nil {
		t.Fatal(err)
	}
	var e iacfacts.ReadEvidence
	if err := json.Unmarshal(raw, &e); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(e.Read, paths) || !reflect.DeepEqual(e.Unread, []string{}) || len(e.Unknowns) != 1 {
		t.Fatalf("registered evidence: %s", raw)
	}
	u := e.Unknowns[0]
	if u.Code != iacfacts.UnknownExpression || u.Span.File != file || !reflect.DeepEqual(u.DecidableBy, []iacfacts.DecidableBy{action}) {
		t.Fatalf("reason: %+v", u)
	}
	all, _ := json.Marshal(p)
	if strings.Contains(string(all), "product-default-canary") || strings.Contains(string(all), `"value":`) {
		t.Fatalf("product value leaked: %s", all)
	}
}

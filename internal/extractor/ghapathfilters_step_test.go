package extractor

import (
	"strings"
	"testing"
)

func TestGitHubPathFilterStepConditions(t *testing.T) {
	filters, err := ReadGitHubPathFilters([]byte(`on: push
jobs:
  controlled:
    steps:
      - run: example
      - if: true
        run: example
  expression:
    steps:
      - if: '${{ github.event_name == "pull_request" }}'
        run: example
  implicit:
    steps:
      - if: github.ref == 'refs/heads/main'
        run: example
  disabled:
    steps:
      - if: false
        run: example
`))
	if err != nil {
		t.Fatal(err)
	}
	if filters.Jobs["controlled"] != "" {
		t.Fatalf("literal true: %q", filters.Jobs["controlled"])
	}
	for _, job := range []string{"expression", "implicit", "disabled"} {
		if condition := filters.Jobs[job]; !strings.Contains(condition, "step if") {
			t.Errorf("%s condition = %q, want step if uncertainty", job, condition)
		}
	}
}

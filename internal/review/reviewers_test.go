package review

import (
	"reflect"
	"testing"
)

// TestSuggestReviewersFromOwnership proves suggestReviewers' own contract
// (CX-4, design doc §2, I58): CODEOWNERS and frequent-committer matches are
// merged, each entry carries the files that put it there, and the ONLY
// ordering applied is alphabetical — never a rank, score or commit count.
func TestSuggestReviewersFromOwnership(t *testing.T) {
	codeowners := []OwnerFiles{
		{Owner: "@team-widget", Files: []string{"internal/widget/widget.go", "internal/widget/widget.go", "internal/widget/api.go"}},
	}
	frequent := []OwnerFiles{
		{Owner: "@alice", Files: []string{"internal/widget/api.go"}},
		{Owner: "@team-widget", Files: []string{"internal/widget/other.go"}},
	}

	got := suggestReviewers(codeowners, frequent)

	want := []OwnerFiles{
		{Owner: "@alice", Files: []string{"internal/widget/api.go"}, Source: OwnerSourceFrequentCommitter},
		{Owner: "@team-widget", Files: []string{"internal/widget/api.go", "internal/widget/widget.go"}, Source: OwnerSourceCodeowners},
		{Owner: "@team-widget", Files: []string{"internal/widget/other.go"}, Source: OwnerSourceFrequentCommitter},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("suggestReviewers() =\n%+v\nwant\n%+v", got, want)
	}

	// I58, structural: OwnerFiles must never carry a rank, score or commit
	// count field — a human's standing next to their name, not just the
	// absence of a sort key.
	typ := reflect.TypeOf(OwnerFiles{})
	deniedFields := []string{"Rank", "Score", "Count", "Commits", "Frequency", "Weight"}
	for i := 0; i < typ.NumField(); i++ {
		name := typ.Field(i).Name
		for _, denied := range deniedFields {
			if name == denied {
				t.Errorf("OwnerFiles has a %q field — I58 forbids ranking or scoring a person", name)
			}
		}
	}
}

func TestSuggestReviewersEmptyInputsAreEmpty(t *testing.T) {
	got := suggestReviewers(nil, nil)
	if len(got) != 0 {
		t.Errorf("suggestReviewers(nil, nil) = %+v, want empty", got)
	}
}

// TestBuildRiskReceiptCarriesSuggestedReviewers proves the receipt actually
// surfaces CX-4's output, not just that suggestReviewers works standalone.
func TestBuildRiskReceiptCarriesSuggestedReviewers(t *testing.T) {
	in := Input{
		ChangedFiles: []string{"internal/widget/widget.go"},
		CodeownersMatches: []OwnerFiles{
			{Owner: "@team-widget", Files: []string{"internal/widget/widget.go"}},
		},
	}
	r := BuildRiskReceipt(in, DefaultRiskTable())
	if len(r.SuggestedReviewers) != 1 || r.SuggestedReviewers[0].Owner != "@team-widget" {
		t.Fatalf("SuggestedReviewers = %+v, want the codeowners match carried through", r.SuggestedReviewers)
	}
}

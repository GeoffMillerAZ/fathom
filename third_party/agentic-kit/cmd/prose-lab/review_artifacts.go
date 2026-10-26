package main

import (
	"errors"
	"os"
	"reflect"

	prose "github.com/geoffmilleraz/fathom/third_party/agentic-kit/kits/governed-prose-kit"
)

// existingReviewArtifacts never creates or repairs private capture artifacts.
func existingReviewArtifacts(path string) (*os.Root, error) {
	if path == "" {
		return nil, errors.New("missing private artifact root")
	}
	st, err := os.Lstat(path)
	if err != nil || !st.IsDir() || st.Mode().Perm()&0077 != 0 {
		return nil, errors.New("private artifact root unavailable")
	}
	return os.OpenRoot(path)
}
func reviewArtifactValidate(root *os.Root, in reviewInputs, c reviewCase, r reservation, d prose.SupportDecision, caseCharge, cumulative int64) error {
	if root == nil {
		return errors.New("artifact root unavailable")
	}
	prepared := reviewPreparedDigest(in, c.ID, []byte(r.RequestJSON))
	var marker struct {
		CaseID         string `json:"case_id"`
		PreparedDigest string `json:"prepared_digest"`
		StudyDigest    string `json:"study_digest"`
	}
	if err := readJournalJSON(root, c.ID+".started.json", &marker); err != nil || marker.CaseID != c.ID || marker.PreparedDigest != prepared || marker.StudyDigest != in.studyDigest {
		return errors.New("missing or mismatched case marker")
	}
	var got reviewReceipt
	if err := readJournalJSON(root, c.ID+".receipt.json", &got); err != nil {
		return errors.New("missing or invalid review receipt")
	}
	expected := reviewReceipt{Version: "prose-lab.review-capture/1", CaseID: c.ID, StudyDigest: in.studyDigest, PreparedDigest: prepared, CorpusDigest: in.corpus.Digest, GoalScopeDigest: in.goalDigest, ReviewScopeDigest: in.reviewDigest, ReviewLane: in.laneID, RequestedReviewModel: in.selected.Model, NativeRequestDigest: r.RequestHash, Status: d.Status, Expected: c.Expected, Matched: d.Status == c.Expected, ChargeBasis: "journal-usage-priced-or-reserved-local-accounting", CumulativeChargedMicroUSD: &cumulative, CaseChargedMicroUSD: &caseCharge}
	if !reflect.DeepEqual(got, expected) {
		return errors.New("receipt differs from native replay or accounting")
	}
	var reason struct {
		Reason string `json:"reason"`
	}
	if err := readJournalJSON(root, c.ID+".reason.json", &reason); err != nil || reason.Reason == "" || reason.Reason != d.Reason {
		return errors.New("missing or mismatched private reason")
	}
	return nil
}

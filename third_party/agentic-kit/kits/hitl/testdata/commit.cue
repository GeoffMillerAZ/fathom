package hitl

commit: #CommitSnapshot & {
	FormatVersion: 1
	Target: {
		Resource:       "draft"
		Version:        1
		EvidenceDigest: "0000000000000000000000000000000000000000000000000000000000000000"
		ConfigDigest:   "1111111111111111111111111111111111111111111111111111111111111111"
		Value: {Count: 0}
	}
	Decisions: [{
		ID:    "00000000000000000000000000000001"
		Actor: "reviewer"
		Binding: {
			Action:          "publish"
			Resource:        "draft"
			OperationDigest: "2222222222222222222222222222222222222222222222222222222222222222"
			ExpectedVersion: 1
			EvidenceDigest:  commit.Target.EvidenceDigest
			ConfigDigest:    commit.Target.ConfigDigest
		}
		IssuedAt:  "2026-09-13T00:00:00Z"
		ExpiresAt: "2026-09-13T00:01:00Z"
		Revoked:   false
		Consumed:  true
	}]
	Receipts: []
	Attempts: [{
		ID:         commit.Decisions[0].ID
		Actor:      "reviewer"
		Binding:    commit.Decisions[0].Binding
		Status:     "prepared"
		PreparedAt: "2026-09-13T00:00:00Z"
		UpdatedAt:  "2026-09-13T00:00:00Z"
	}]
}

// valid.cue — concrete values that satisfy the constraints defined in
// schema.cue. Running `cue vet ./...` from the package root validates these
// against the schema.
package authenticity

// ── Method values ────────────────────────────────────────────────────────────

_methodHashMatch:         #Method & "hash_match"
_methodC2PA:              #Method & "c2pa"
_methodEXIF:              #Method & "exif_forensics"
_methodAIText:            #Method & "ai_text_detector"
_methodDeepfake:          #Method & "deepfake_detector"
_methodMetadata:          #Method & "metadata_forensics"
_methodManual:            #Method & "manual_review"
_methodDKIM:              #Method & "dkim"
_methodAuthResults:       #Method & "auth_results_header"

// ── Verdict values ───────────────────────────────────────────────────────────

_verdictAuthentic:        #Verdict & "authentic"
_verdictLikelyAuthentic:  #Verdict & "likely_authentic"
_verdictInconclusive:     #Verdict & "inconclusive"
_verdictLikelySynthetic:  #Verdict & "likely_synthetic"
_verdictSynthetic:        #Verdict & "synthetic"
_verdictTampered:         #Verdict & "tampered"

// ── Signal values ────────────────────────────────────────────────────────────

// A hash-match signal from a human analyst confirming the record is unmodified.
_signalHashMatchAuthentic: #Signal & {
	ID:       "sig-001"
	RecordID: "record-ABC"
	Method:   "hash_match"
	Verdict:  "authentic"
	Score:    0.99
	Detail:   "SHA-256 matches known-good reference"
	Author: {
		Kind: "human"
		Human: {
			ID: "analyst@example.com"
		}
	}
	At: "2026-06-08T10:00:00Z"
}

// An AI-text-detector signal indicating likely synthetic content.
_signalAITextDetector: #Signal & {
	ID:       "sig-002"
	RecordID: "record-ABC"
	Method:   "ai_text_detector"
	Verdict:  "likely_synthetic"
	Score:    0.82
	Detail:   "GPT-4o-style token distribution detected"
	Author: {
		Kind: "ai"
		AI: {
			ModelID:               "ai-text-classifier-v3"
			ModelVersion:          "20260601"
			PromptHash:            "sha256-promptDDD"
			PromptTemplateVersion: "2.1.0"
			ParamsHash:            "sha256-paramsEEE"
			InputDigest:           "sha256-inputFFF"
			At:                    "2026-06-08T10:05:00Z"
		}
	}
	At: "2026-06-08T10:05:00Z"
}

// A manual review signal with minimum required fields (Author optional in schema).
_signalManualMinimal: #Signal & {
	ID:       "sig-003"
	RecordID: "record-DEF"
	Method:   "manual_review"
	Verdict:  "inconclusive"
	Score:    0.5
	Detail:   ""
	At:       "2026-06-08T11:00:00Z"
}

// A C2PA signal at Score boundary 0.
_signalC2PAZeroScore: #Signal & {
	ID:       "sig-004"
	RecordID: "record-GHI"
	Method:   "c2pa"
	Verdict:  "inconclusive"
	Score:    0.0
	Detail:   "C2PA credentials absent"
	At:       "2026-06-08T11:30:00Z"
}

// A deepfake-detector signal at Score boundary 1.
_signalDeepfakeMaxScore: #Signal & {
	ID:       "sig-005"
	RecordID: "record-JKL"
	Method:   "deepfake_detector"
	Verdict:  "synthetic"
	Score:    1.0
	Detail:   "Face swap artifacts detected with maximum confidence"
	Author: {
		Kind: "ai"
		AI: {
			ModelID:      "deepfake-detector-v2"
			ModelVersion: "20260601"
			PromptHash:   "sha256-promptGGG"
			ParamsHash:   ""
			InputDigest:  "sha256-inputHHH"
			At:           "2026-06-08T12:00:00Z"
		}
	}
	At: "2026-06-08T12:00:00Z"
}

// A DKIM-verified signal: cryptographic signature verified against DNS-published key.
_signalDKIMPass: #Signal & {
	ID:       "sig-006"
	RecordID: "record-EML"
	Method:   "dkim"
	Verdict:  "authentic"
	Score:    0.95
	Detail:   "DKIM-Signature verified for domain example.com selector mail"
	Author: {
		Kind: "human"
		Human: {
			ID: "detector:dkim-v1"
		}
	}
	At: "2026-06-10T09:00:00Z"
}

// A receiver-asserted auth_results_header signal (SPF pass, not independently verified).
_signalAuthResultsHeader: #Signal & {
	ID:       "sig-007"
	RecordID: "record-EML"
	Method:   "auth_results_header"
	Verdict:  "likely_authentic"
	Score:    0.55
	Detail:   "Authentication-Results: spf=pass dkim=pass dmarc=pass (receiver-asserted)"
	At:       "2026-06-10T09:01:00Z"
}

// ── Assessment values ────────────────────────────────────────────────────────

// An assessment that concluded authentic — no adverse signals, high confidence.
_assessmentAuthentic: #Assessment & {
	RecordID:     "record-ABC"
	Overall:      "authentic"
	Confidence:   0.997
	Signals:      [_signalHashMatchAuthentic]
	Conservative: false
}

// An assessment where a tampered signal dominated authentic signals
// (conservative rule applied).
_assessmentTamperedDominates: #Assessment & {
	RecordID:     "record-MNO"
	Overall:      "tampered"
	Confidence:   0.80
	Signals: [{
		ID:       "sig-auth-1"
		RecordID: "record-MNO"
		Method:   "hash_match"
		Verdict:  "authentic"
		Score:    0.95
		Detail:   "hash ok"
		At:       "2026-06-08T10:00:00Z"
	}, {
		ID:       "sig-tamper-1"
		RecordID: "record-MNO"
		Method:   "exif_forensics"
		Verdict:  "tampered"
		Score:    0.80
		Detail:   "EXIF GPS coordinates inconsistent with claimed origin"
		At:       "2026-06-08T10:10:00Z"
	}]
	Conservative: true
}

// An inconclusive assessment — no strong signals either way.
_assessmentInconclusive: #Assessment & {
	RecordID:     "record-PQR"
	Overall:      "inconclusive"
	Confidence:   0.55
	Signals: [{
		ID:       "sig-inc-1"
		RecordID: "record-PQR"
		Method:   "metadata_forensics"
		Verdict:  "inconclusive"
		Score:    0.55
		Detail:   "metadata present but unverifiable"
		At:       "2026-06-08T13:00:00Z"
	}]
	Conservative: false
}

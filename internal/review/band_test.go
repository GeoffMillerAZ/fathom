package review

import "testing"

// TestRiskBandStrongestReason proves riskBand's own contract (CX-2, design
// doc §2, ADR-051 decision 4, refused CX-X1): the band is the strongest
// cited reason, never a summed score — many medium factors never outrank
// one high factor, and among several factors at the same strongest weight
// the FIRST one in the caller's own order is cited, never a merge of all of
// them.
func TestRiskBandStrongestReason(t *testing.T) {
	table := DefaultRiskTable()

	t.Run("all none is low", func(t *testing.T) {
		factors := []RiskFactor{
			{Name: "a", Weight: WeightNone, Reason: "a clean"},
			{Name: "b", Weight: WeightNone, Reason: "b clean"},
		}
		band, reason := riskBand(factors, table)
		if band != BandLow {
			t.Errorf("band = %q, want low", band)
		}
		if reason == "a clean" || reason == "b clean" {
			t.Errorf("reason %q must not cite a factor that never fired", reason)
		}
	})

	t.Run("many mediums never outrank one high — no summed score", func(t *testing.T) {
		factors := []RiskFactor{
			{Name: "a", Weight: WeightMedium, Reason: "a medium"},
			{Name: "b", Weight: WeightMedium, Reason: "b medium"},
			{Name: "c", Weight: WeightMedium, Reason: "c medium"},
			{Name: "d", Weight: WeightHigh, Reason: "d high"},
		}
		band, reason := riskBand(factors, table)
		if band != BandHigh {
			t.Fatalf("band = %q, want high (three mediums must never sum past one high)", band)
		}
		if reason != "d high" {
			t.Errorf("reason = %q, want the single high factor's own reason", reason)
		}
	})

	t.Run("first factor at the strongest weight wins the citation", func(t *testing.T) {
		factors := []RiskFactor{
			{Name: "a", Weight: WeightHigh, Reason: "a high"},
			{Name: "b", Weight: WeightHigh, Reason: "b high"},
		}
		_, reason := riskBand(factors, table)
		if reason != "a high" {
			t.Errorf("reason = %q, want the first high factor (%q), not a merge of both", reason, "a high")
		}
	})

	t.Run("medium alone bands medium", func(t *testing.T) {
		factors := []RiskFactor{
			{Name: "a", Weight: WeightNone, Reason: "a clean"},
			{Name: "b", Weight: WeightMedium, Reason: "b medium"},
		}
		band, reason := riskBand(factors, table)
		if band != BandMedium {
			t.Errorf("band = %q, want medium", band)
		}
		if reason != "b medium" {
			t.Errorf("reason = %q, want %q", reason, "b medium")
		}
	})
}

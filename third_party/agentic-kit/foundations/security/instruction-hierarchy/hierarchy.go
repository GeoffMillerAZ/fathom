package instructionhierarchy

import "fmt"

// Resolve enforces instruction precedence rules across the supplied slice.
// The algorithm:
//  1. Establish the highest authority tier present.
//  2. Any instruction whose content appears to override a higher tier is
//     rejected. Specifically, an untrusted instruction is always rejected if a
//     higher-tier instruction is present. A user instruction is rejected if it
//     attempts to claim system or developer authority.
//  3. All non-rejected instructions are returned in Accepted, ordered by
//     descending tier rank (system first, untrusted last).
//
// Resolve is deterministic — identical inputs always produce identical outputs.
func Resolve(instructions []Instruction) Effective {
	if len(instructions) == 0 {
		return Effective{}
	}

	// Find the highest tier present.
	highest := TierUntrusted
	for _, inst := range instructions {
		if tierRank[inst.Tier] > tierRank[highest] {
			highest = inst.Tier
		}
	}

	var accepted []Instruction
	var rejected []RejectedInstruction

	for _, inst := range instructions {
		reason := checkViolation(inst, highest)
		if reason != "" {
			rejected = append(rejected, RejectedInstruction{
				Instruction: inst,
				Reason:      reason,
			})
		} else {
			accepted = append(accepted, inst)
		}
	}

	// Sort accepted by descending tier rank (stable: preserve original order
	// within same tier).
	sortByTierDesc(accepted)

	return Effective{
		Accepted: accepted,
		Rejected: rejected,
	}
}

// checkViolation returns a non-empty rejection reason if inst violates the
// hierarchy rules given the highestTier present in the set, or "" if it is
// accepted.
func checkViolation(inst Instruction, highestTier Tier) string {
	switch inst.Tier {
	case TierUntrusted:
		// Untrusted is rejected whenever any higher authority is present.
		if highestTier != TierUntrusted {
			return fmt.Sprintf(
				"untrusted instruction rejected: higher-authority tier %q is present",
				highestTier,
			)
		}
	case TierUser:
		// User instructions are rejected if system or developer authority is present
		// AND the user content contains override indicators.
		if (highestTier == TierSystem || highestTier == TierDeveloper) && containsOverrideAttempt(inst.Content) {
			return fmt.Sprintf(
				"user instruction rejected: contains override attempt against tier %q",
				highestTier,
			)
		}
	}
	return ""
}

// overrideKeywords are generic indicators that a lower-tier instruction is
// trying to usurp higher-tier authority. No org-specific literals.
var overrideKeywords = []string{
	"ignore system",
	"override system",
	"disregard system",
	"ignore developer",
	"override developer",
	"disregard developer",
	"ignore previous",
	"override previous",
	"disregard previous",
	"you are now",
	"new instructions:",
	"updated instructions:",
}

// containsOverrideAttempt checks whether the content contains a generic
// override indicator suggesting an attempt to supersede higher-tier authority.
func containsOverrideAttempt(content string) bool {
	lower := toLower(content)
	for _, kw := range overrideKeywords {
		if contains(lower, kw) {
			return true
		}
	}
	return false
}

// sortByTierDesc performs a stable sort by descending tier rank.
// Uses insertion sort (small slices in practice).
func sortByTierDesc(instructions []Instruction) {
	for i := 1; i < len(instructions); i++ {
		for j := i; j > 0 && tierRank[instructions[j].Tier] > tierRank[instructions[j-1].Tier]; j-- {
			instructions[j], instructions[j-1] = instructions[j-1], instructions[j]
		}
	}
}

// toLower converts a string to lower-case using only ASCII folding to avoid
// importing unicode or strings packages beyond what stdlib provides.
func toLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

// contains reports whether s contains substr (both already lower-cased).
func contains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(substr) > len(s) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

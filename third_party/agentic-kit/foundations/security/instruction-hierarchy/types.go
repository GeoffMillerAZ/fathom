// Package instructionhierarchy enforces instruction precedence for agentic
// systems: system > developer > user > untrusted. Lower-tier instructions
// cannot override higher-tier ones.
//
// Concurrency: values are immutable and safe for concurrent use.
package instructionhierarchy

// Tier represents the authority level of an instruction source.
// Values are strictly ordered: System > Developer > User > Untrusted.
type Tier string

const (
	// TierSystem is the highest authority — set by the platform/operator.
	TierSystem Tier = "system"

	// TierDeveloper is the application-level authority — set by the builder.
	TierDeveloper Tier = "developer"

	// TierUser is the end-user authority — set by an authenticated human.
	TierUser Tier = "user"

	// TierUntrusted is the lowest authority — sourced from untrusted content
	// (e.g. external documents, tool outputs, injected text).
	TierUntrusted Tier = "untrusted"
)

// tierRank maps each Tier to a numeric rank for ordered comparison.
// Higher numbers indicate higher authority.
var tierRank = map[Tier]int{
	TierUntrusted: 0,
	TierUser:      1,
	TierDeveloper: 2,
	TierSystem:    3,
}

// TierGTE returns true if tier a is greater than or equal to tier b.
func TierGTE(a, b Tier) bool {
	return tierRank[a] >= tierRank[b]
}

// Instruction is a single directive carrying a Tier and textual content.
type Instruction struct {
	// Tier is the authority level of the instruction source.
	Tier Tier

	// Content is the directive text.
	Content string
}

// Effective is the resolved set of instructions after hierarchy enforcement.
type Effective struct {
	// Accepted is the list of instructions that passed the hierarchy check,
	// ordered by descending Tier (highest authority first).
	Accepted []Instruction

	// Rejected is the list of instructions that were rejected because they
	// attempted to override a higher-tier instruction.
	Rejected []RejectedInstruction
}

// RejectedInstruction records an instruction that was rejected and why.
type RejectedInstruction struct {
	// Instruction is the original instruction that was rejected.
	Instruction Instruction

	// Reason is a human-readable explanation of why it was rejected.
	Reason string
}

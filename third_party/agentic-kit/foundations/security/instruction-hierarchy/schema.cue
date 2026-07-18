// Package instructionhierarchy defines CUE contracts for instruction-tier
// enforcement. Tiers form a strictly ordered authority chain:
// system > developer > user > untrusted.
package instructionhierarchy

// #Tier enumerates the four recognised instruction authority levels.
// Values are strictly ordered: untrusted < user < developer < system.
#Tier: "system" | "developer" | "user" | "untrusted"

// #Instruction is a single directive carrying a tier and textual content.
#Instruction: {
	// Tier is the authority level of the instruction source.
	Tier: #Tier

	// Content is the directive text. Must not be empty.
	Content: string & !=""
}

// #RejectedInstruction records an instruction that was rejected and why.
#RejectedInstruction: {
	// Instruction is the original instruction that was rejected.
	Instruction: #Instruction

	// Reason is a human-readable explanation of the rejection.
	Reason: string & !=""
}

// #Effective is the resolved set of instructions after hierarchy enforcement.
#Effective: {
	// Accepted is the list of instructions that passed the hierarchy check.
	Accepted: [...#Instruction]

	// Rejected is the list of instructions that were rejected.
	Rejected: [...#RejectedInstruction]
}

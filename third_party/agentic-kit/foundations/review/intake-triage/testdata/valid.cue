package intaketriage

// valid_proposed is the minimal valid proposed item.
valid_proposed: #Proposal & {
	ID:    "prop-001"
	State: "proposed"
}

// valid_imported is a human-decided import.
valid_imported: #Proposal & {
	ID:    "prop-002"
	State: "imported"
	DecidedBy: {
		Kind: "human"
		ID:   "human:web"
	}
}

// valid_excluded requires a reason.
valid_excluded: #Proposal & {
	ID:              "prop-003"
	State:           "excluded"
	ExclusionReason: "not relevant to the investigation"
	DecidedBy: {
		Kind: "human"
		ID:   "human:reviewer"
	}
}

// valid_deferred is a deferred proposal.
valid_deferred: #Proposal & {
	ID:    "prop-004"
	State: "deferred"
	DecidedBy: {
		Kind: "human"
		ID:   "human:web"
	}
}

// valid_transition records a human-initiated transition.
valid_transition: #TransitionEvent & {
	ProposalID: "prop-001"
	From:       "proposed"
	To:         "imported"
	By: {
		Kind: "human"
		ID:   "human:web"
	}
	Reason: ""
}

// valid_exclusion_transition records an exclusion with a reason.
valid_exclusion_transition: #TransitionEvent & {
	ProposalID: "prop-003"
	From:       "proposed"
	To:         "excluded"
	By: {
		Kind: "human"
		ID:   "human:reviewer"
	}
	Reason: "privileged communication"
}

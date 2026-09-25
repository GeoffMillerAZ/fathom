package qualityplan

import (
	"list"
	"strings"
)

#ID:         string & =~"^[a-z][a-z0-9._-]{0,95}$"
#Hash:       string & =~"^[a-f0-9]{64}$"
#MaybeText:  string & strings.MaxRunes(2048)
#Text:       #MaybeText & =~"[^\\s]"
#RepoPath:   string & =~"^[a-zA-Z0-9][a-zA-Z0-9._/-]*$" & !~"(^|/)\\.\\.(/|$)" & !~"^/"
#Profile:    "static-web" | "go-web" | "react-web" | "portable" | "cli"
#GateName:   "reproducibility" | "structure" | "visual" | "interaction" | "lifecycle" | "security" | "data" | "performance" | "integration"
#StandardID: "typed-config" | "semantic-theme" | "native-interaction" | "honest-states" | "bounded-performance" | "safe-composition" | "reproducible-delivery"

#Ref: {
	path!: #RepoPath
	note!: #Text
}

#PinnedRef: {
	path!:   #RepoPath
	sha256!: #Hash
}

#Review: {
	by!:  #Text
	ref!: #RepoPath
}

#StandardDefinition: {
	name!:        #Text
	requirement!: #Text
	gate_refs!: [...#GateName] & list.MinItems(1) & list.MaxItems(4)
}

#StandardDefinitions: {
	"typed-config"!:          #StandardDefinition
	"semantic-theme"!:        #StandardDefinition
	"native-interaction"!:    #StandardDefinition
	"honest-states"!:         #StandardDefinition
	"bounded-performance"!:   #StandardDefinition
	"safe-composition"!:      #StandardDefinition
	"reproducible-delivery"!: #StandardDefinition
}

#StandardStatus: {
	status!:    *"not-started" | "not-started" | "partial" | "implemented" | "not-applicable"
	reason?:    #Text
	next_step?: #Text
	evidence!: *[] | [...#Ref] & list.MaxItems(16)
	review?: #Review
	if status == "not-started" {
		next_step!: #Text
		evidence: []
	}
	if status == "partial" {
		reason!:    #Text
		next_step!: #Text
		evidence:   list.MinItems(1)
	}
	if status == "implemented" {
		evidence: list.MinItems(1)
		review!:  #Review
	}
	if status == "not-applicable" {
		reason!: #Text
		evidence: []
		review!: #Review
	}
}

#StandardStatuses: {
	"typed-config"!:          #StandardStatus
	"semantic-theme"!:        #StandardStatus
	"native-interaction"!:    #StandardStatus
	"honest-states"!:         #StandardStatus
	"bounded-performance"!:   #StandardStatus
	"safe-composition"!:      #StandardStatus
	"reproducible-delivery"!: #StandardStatus
}

#GateStatus: {
	status!:    *"not-run" | "not-run" | "partial" | "failed" | "passed" | "not-applicable"
	reason?:    #Text
	next_step?: #Text
	evidence!: *[] | [...#Ref] & list.MaxItems(32)
	receipt?:           #PinnedRef
	source_revision?:   #Hash
	artifact_revision?: #Hash
	review?:            #Review
	if status == "not-run" {
		next_step!: #Text
		evidence: []
	}
	if status == "partial" {
		reason!:    #Text
		next_step!: #Text
		evidence:   list.MinItems(1)
	}
	if status == "failed" {
		reason!:    #Text
		next_step!: #Text
		evidence:   list.MinItems(1)
	}
	if status == "passed" {
		evidence:           list.MinItems(1)
		receipt!:           #PinnedRef
		source_revision!:   #Hash
		artifact_revision!: #Hash
		review!:            #Review
	}
	if status == "not-applicable" {
		reason!: #Text
		evidence: []
		review!: #Review
	}
}

#GateStatuses: {
	reproducibility!: #GateStatus
	structure!:       #GateStatus
	visual!:          #GateStatus
	interaction!:     #GateStatus
	lifecycle!:       #GateStatus
	security!:        #GateStatus
	data!:            #GateStatus
	performance!:     #GateStatus
	integration!:     #GateStatus
}

#Consumer: {
	app!: #ID
	revision!: {
		kind!:   "source" | "selection" | "artifact"
		sha256!: #Hash
	}
	integration_status!: "selected" | "integrated" | "verified-scoped" | "adopted"
	evidence!: [...#Ref] & list.MinItems(1) & list.MaxItems(16)
	receipt?: #PinnedRef
	review?:  #Review
	if integration_status == "adopted" {
		receipt!: #PinnedRef
		review!:  #Review
	}
}

#Budget: {
	name!:  #ID
	limit!: #Text
}

#ConversionSelection: {
	original!: {
		kind!:            "catalog-capability" | "design-asset" | "private-source"
		id!:              #Text
		revision_sha256!: #Hash
	}
	target!: {
		id!:              #Text
		revision_sha256!: #Hash
	}
	acceptance_contract!: #PinnedRef
	workload!:            #Text
	budgets!: [...#Budget] & list.MinItems(1) & list.MaxItems(16)
}

#Item: {
	id!:                            #ID
	kind!:                          "starter-area" | "candidate" | "conversion"
	owner!:                         #Text
	task!:                          #Text
	profile!:                       #Profile
	priority!:                      "core" | "high" | "normal" | "on-demand"
	core!:                          bool
	state!:                         "queued" | "in-progress" | "blocked" | "deferred" | "accepted"
	enhancement_standard_revision!: "converted-core-v1"
	capability_ids!: [...#ID] & list.MinItems(1) & list.MaxItems(16)
	source_refs!: [...#Ref] & list.MinItems(1) & list.MaxItems(16)
	standards!: #StandardStatuses
	gates!:     #GateStatuses
	selection?: #ConversionSelection
	// Empty means no verified adoption is recorded yet, never that no consumer exists.
	consumers!: [...#Consumer] & list.MaxItems(16)
	if core {
		priority: "core"
		kind:     "starter-area"
	}
	if !core {
		priority: "high" | "normal" | "on-demand"
		kind:     "candidate" | "conversion"
	}
	if kind != "conversion" {
		selection?: _|_
	}
	if kind == "candidate" {
		state: "queued" | "deferred"
	}
	if kind == "conversion" && state == "in-progress" {
		selection!: #ConversionSelection
	}
	if kind == "conversion" && state == "blocked" {
		selection!: #ConversionSelection
	}
	if kind == "conversion" && state == "accepted" {
		selection!: #ConversionSelection
	}
	if state == "accepted" {
		standards: [string]: status: "implemented" | "not-applicable"
		gates: [string]: status:     "passed" | "not-applicable"
	}
}

#BacklogPolicy: {
	mode!:          "just-in-time"
	default_state!: "unselected-not-run"
	selection_requires!: [...#Text] & list.MinItems(3) & list.MaxItems(8)
	owner_batch_requires!: [...#Text] & list.MinItems(2) & list.MaxItems(8)
	unselected_claim!: #Text
}

#InventoryShape: {
	schema_version!:                1
	enhancement_standard_revision!: "converted-core-v1"
	standards!:                     #StandardDefinitions
	backlog_policy!:                #BacklogPolicy
	items!: [...#Item] & list.MinItems(6) & list.MaxItems(32)
}

// cue vet/export apply this definition to inventory.cue's concrete top-level data.
#Inventory: #InventoryShape

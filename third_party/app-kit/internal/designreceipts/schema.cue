package designreceipts

import "list"
import "strings"

#ID: string & =~"^[a-z][a-z0-9._-]{0,95}$"
#Hash: string & =~"^[a-f0-9]{64}$"
#MaybeText: string & strings.MaxRunes(2048)
#Text: #MaybeText & =~"[^\\s]"
#MaxGateEvidence: 64
#Asset: {id!: #ID, sha256!: #Hash, bytes!: int & >=0 & <=33554432}
#Assets: [...#Asset] & list.MaxItems(64)
#Inputs: {
	source!: #Assets & list.MinItems(1)
	transformer!: #Assets & list.MinItems(1)
	config!: #Assets
	patch!: #Assets
	runtime!: #Assets
	output!: #Assets & list.MinItems(1)
	fixture!: #Assets & list.MinItems(1)
	toolchain!: #Assets & list.MinItems(1)
	baseline!: #Assets & list.MinItems(1)
}
#Subject: {
	id!: #ID
	profile!: "static-web" | "go-web" | "react-web" | "portable" | "cli"
	variant!: #ID
	source_archive!: string & =~"^[a-z0-9-]+@[a-f0-9]{64}$"
	source_revision!: #Hash
	artifact_sha256!: #Hash
}
#Slot: {id!: #ID, role!: "report" | "image" | "review" | "measurement"}
#Gate: {
	applicable!: bool
	scope!: #Text
	reason!: #MaybeText
	review_assertion!: #MaybeText
	slots!: [...#Slot] & list.MaxItems(#MaxGateEvidence)
	if applicable { slots: list.MinItems(1) }
	if !applicable { reason: #Text, review_assertion: #Text, slots: [] }
}
#Gates: {
	reproducibility!: #Gate
	structure!: #Gate
	visual!: #Gate
	interaction!: #Gate
	lifecycle!: #Gate
	security!: #Gate
	data!: #Gate
	performance!: #Gate
	integration!: #Gate
}
#Contract: {
	schema_version!: 1
	subject!: #Subject
	// Prospective policy and imported historical evidence must remain distinct.
	mode!: "prospective" | "retrospective"
	inputs!: #Inputs
	gates!: #Gates
}
#Evidence: {#Asset, role!: "report" | "image" | "review" | "measurement"}
#Result: {
	status!: "passed" | "failed" | "not-run" | "not-applicable"
	reason!: #MaybeText
	evidence!: [...#Evidence] & list.MaxItems(#MaxGateEvidence)
	if status != "passed" { reason: #Text }
	if status == "not-applicable" { evidence: [] }
}
#Results: {
	reproducibility!: #Result
	structure!: #Result
	visual!: #Result
	interaction!: #Result
	lifecycle!: #Result
	security!: #Result
	data!: #Result
	performance!: #Result
	integration!: #Result
}
#Receipt: {
	schema_version!: 1
	contract_sha256!: #Hash
	source_revision!: #Hash
	artifact_sha256!: #Hash
	results!: #Results
}

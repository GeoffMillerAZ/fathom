package designevidence

import "list"

#Hash: string & =~"^[a-f0-9]{64}$"
#Text: string & =~"[^\\s]"
#Subject: {
	id!: #Text
	profile!: "static-web" | "go-web" | "react-web" | "portable" | "cli"
	variant!: #Text
	source_archive!: string & =~"^[a-z0-9-]+@[a-f0-9]{64}$"
	source_revision!: #Hash
	artifact_sha256!: #Hash
}
#Report: {
	integrity!: "matched"
	verification!: "not-verified"
	contract_sha256!: #Hash
	receipt_sha256!: #Hash
	subject!: #Subject
	mode!: "prospective" | "retrospective"
	claimed_statuses!: [string]: "passed" | "failed" | "not-run" | "not-applicable"
	incomplete_gates!: [...string] & list.MaxItems(9)
	matched_artifacts!: int & >=0 & <=864
}
#Entry: {
	id!: string & =~"^evidence@[a-f0-9]{64}$"
	source_member!: #Text
	recorded!: #Report
}

// The index is a historical integrity snapshot. Its report is explicitly not a
// current transformer, target semantic, runtime, or compatibility certification.
#Catalog: {
	schema_version!: 1
	revision!: #Hash
	entries!: [...#Entry] & list.MaxItems(4096)
}

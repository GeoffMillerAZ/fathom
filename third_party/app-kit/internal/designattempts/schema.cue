package designattempts

import "list"

#Hash: string & =~"^[a-f0-9]{64}$"
#Text: string & !=""
#Entry: {
	id!: string & =~"^syntax@[a-f0-9]{64}$"
	source!: {
		archive_id!: string & =~"^[a-z0-9-]+@[a-f0-9]{64}$"
		path!: #Text
		sha256!: #Hash
		size!: int & >=0 & <=262144
	}
	target!: "go-templ"
	transformer!: {
		id!: "html-to-templ-v1"
		sha256!: #Hash
		go_version!: #Text
		html_parser!: string & =~"^golang\\.org/x/net@v[0-9]+\\.[0-9]+\\.[0-9]+([+.-][0-9A-Za-z.+-]+)?$"
		required_templ!: string & =~"^github\\.com/a-h/templ@v[0-9]+\\.[0-9]+\\.[0-9]+([+.-][0-9A-Za-z.+-]+)?$"
	}
	status!: "translated" | "unsupported"
	verification!: "not-verified"
	diagnostic_codes!: [...#Text] & list.MaxItems(64)
	omitted_diagnostics!: int & >=0
	if status == "translated" {
		artifact_sha256!: #Hash
		diagnostic_codes: []
		omitted_diagnostics: 0
	}
	if status == "unsupported" {
		artifact_sha256?: _|_
		diagnostic_codes: list.MinItems(1)
	}
}

// This index records historical attempts; its hash validates metadata, not fresh
// artifact replay, original membership or target-runtime compatibility.
#Catalog: {
	schema_version!: 1
	revision!: #Hash
	entries!: [...#Entry] & list.MaxItems(4096)
}

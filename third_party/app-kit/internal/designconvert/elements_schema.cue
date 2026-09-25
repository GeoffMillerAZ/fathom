package designconvert

import "list"

#ElementsHash: string & =~"^[a-f0-9]{64}$"

#ElementsDiagnostic: {
	code!: string & !=""
	message!: string & !=""
	offset!: int & >=0 & <=262144
}

#ElementsMapping: {
	source_start!: int & >=0 & <=262144
	source_end!: int & >=source_start & <=262144
	output_start!: int & >=0
	output_end!: int & >=output_start
}

#ElementsTranslation: {
	schema_version!: 1
	kind!: "elements-syntax"
	source!: {
		archive_id!: string & =~"^[a-z0-9-]+@[a-f0-9]{64}$"
		path!: string & !=""
		sha256!: #ElementsHash
		size!: int & >=0 & <=262144
	}
	target!: "go-templ"
	transformer!: {
		id!: "html-elements-to-templ-v1"
		sha256!: #ElementsHash
		go_version!: string & !=""
		html_parser!: "golang.org/x/net@v0.54.0"
		required_templ!: "github.com/a-h/templ@v0.3.1020"
	}
	runtime!: {
		package!: "@tailwindplus/elements"
		version!: "1.0.22"
		esm_sha256!: "120ff5b81c8b3bb7e6de13d8b83a92da984c4312aa017b7392c9cb876cd65013"
		tgz_sha256!: "fc684f17d960fb60dadfdf9a0b49ba8fd295af4b8fd1715a26cd20b832ea245b"
		delivery!: "host-provided-module"
	}
	verification!: "not-verified"
	status!: "translated" | "unsupported"
	diagnostics!: [...#ElementsDiagnostic] & list.MaxItems(64)
	omitted_diagnostics!: int & >=0
	if status == "translated" {
		diagnostics: []
		omitted_diagnostics: 0
		artifact!: {
			filename!: "snippet.templ"
			sha256!: #ElementsHash
			content!: string & !=""
			mappings!: [...#ElementsMapping]
		}
	}
	if status == "unsupported" {
		diagnostics: list.MinItems(1)
		artifact?: _|_
	}
}

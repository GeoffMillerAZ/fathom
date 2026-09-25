package designconvert

import "list"

#Hash: string & =~"^[a-f0-9]{64}$"
#Diagnostic: {
	code!: string & !=""
	message!: string & !=""
	offset!: int & >=0 & <=262144
}
#Mapping: {
	source_start!: int & >=0 & <=262144
	source_end!: int & >=source_start & <=262144
	output_start!: int & >=0
	output_end!: int & >=output_start
}
// Historical identities describe the attempt that was saved, not a supported
// runtime. Current emission/replay additionally require the exact pins below.
#StoredTranslation: {
	schema_version!: 1
	source!: {
		archive_id!: string & =~"^[a-z0-9-]+@[a-f0-9]{64}$"
		path!: string & !=""
		sha256!: #Hash
		size!: int & >=0 & <=262144
	}
	target!: "go-templ"
	transformer!: {
		id!: "html-to-templ-v1"
		sha256!: #Hash
		go_version!: string & !=""
		html_parser!: string & =~"^golang\\.org/x/net@v[0-9]+\\.[0-9]+\\.[0-9]+([+.-][0-9A-Za-z.+-]+)?$"
		required_templ!: string & =~"^github\\.com/a-h/templ@v[0-9]+\\.[0-9]+\\.[0-9]+([+.-][0-9A-Za-z.+-]+)?$"
	}
	verification!: "not-verified"
	status!: "translated" | "unsupported"
	diagnostics!: [...#Diagnostic] & list.MaxItems(64)
	omitted_diagnostics!: int & >=0
	if status == "translated" {
		diagnostics: []
		omitted_diagnostics: 0
		artifact!: {
			filename!: "snippet.templ"
			sha256!: #Hash
			content!: string & !=""
			mappings!: [...#Mapping]
		}
	}
	if status == "unsupported" {
		diagnostics: list.MinItems(1)
		artifact?: _|_
	}
}

#Translation: #StoredTranslation & {
	transformer: {
		html_parser: "golang.org/x/net@v0.54.0"
		required_templ: "github.com/a-h/templ@v0.3.1020"
	}
}

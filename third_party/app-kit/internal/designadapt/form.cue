package designadapt

#FormHash: string & =~"^[a-f0-9]{64}$"
// A generation envelope, never a target certificate.
#FormArtifact: {
	schema_version!: 1
	adapter!: "p05-email-field-v1"
	source!: {
		archive_id!: string & =~"^[a-z0-9-]+@[a-f0-9]{64}$"
		path!: string & !=""
		sha256!: #FormHash
		size!: int & >=1 & <=262144
	}
	syntax!: {
		id!: "html-to-templ-v1"
		sha256!: #FormHash
		go_version!: string & !=""
		html_parser!: "golang.org/x/net@v0.54.0"
		required_templ!: "github.com/a-h/templ@v0.3.1020"
	}
	rule_sha256!: #FormHash
	syntax_sha256!: syntax.sha256
	templ_sha256!: #FormHash
	model_sha256!: #FormHash
	defaults_sha256!: #FormHash
	templ!: string & !=""
	model!: string & !=""
	defaults!: {
		ID!: string & !=""
		Name!: string & !=""
		Label!: string & !=""
		Value!: string
		Placeholder!: string
		Error!: string & !=""
		Required!: false
		Autofocus!: false
	}
	verification!: "not-verified"
}

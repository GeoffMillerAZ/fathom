package designadapt

import "list"

#DirectoryHash: string & =~"^[a-f0-9]{64}$"
// Source generation evidence only; never a certified-target receipt.
#DirectoryArtifact: {
	schema_version!: 1
	adapter!: "p04-directory-navigation-v1"
	source!: {
		archive_id!: string & =~"^[a-z0-9-]+@[a-f0-9]{64}$"
		path!: string & !=""
		sha256!: #DirectoryHash
		size!: int & >=1 & <=262144
	}
	syntax!: {
		id!: "html-to-templ-v1"
		sha256!: #DirectoryHash
		go_version!: string & !=""
		html_parser!: "golang.org/x/net@v0.54.0"
		required_templ!: "github.com/a-h/templ@v0.3.1020"
	}
	rule_sha256!: #DirectoryHash
	syntax_sha256!: syntax.sha256
	templ_sha256!: #DirectoryHash
	model_sha256!: #DirectoryHash
	defaults_sha256!: #DirectoryHash
	templ!: string & !=""
	model!: string & !=""
	defaults!: {
		Heading!: string & !=""
		Introduction!: string & !=""
		AddLabel!: string & !=""
		EditLabel!: string & !=""
		NameLabel!: string & !=""
		TitleLabel!: string & !=""
		EmailLabel!: string & !=""
		RoleLabel!: string & !=""
		AddURL!: ""
		Rows!: [...{
			Name!: string & !=""
			Title!: string & !=""
			Email!: string & !=""
			Role!: string & !=""
			EditURL!: ""
		}] & list.MinItems(6) & list.MaxItems(6)
	}
	verification!: "not-verified"
}

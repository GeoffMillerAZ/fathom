package designadapt

import "list"

#NavbarHash: string & =~"^[a-f0-9]{64}$"
#NavbarArtifact: {
	schema_version!: 1
	adapter!: "p09-responsive-navbar-v1"
	source!: {
		archive_id!: string & =~"^[a-z0-9-]+@[a-f0-9]{64}$"
		path!: string & !=""
		sha256!: #NavbarHash
		size!: int & >=1 & <=262144
	}
	syntax!: {
		id!: "html-elements-to-templ-v1"
		sha256!: #NavbarHash
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
	rule_sha256!: #NavbarHash
	syntax_sha256!: syntax.sha256
	templ_sha256!: #NavbarHash
	model_sha256!: #NavbarHash
	defaults_sha256!: #NavbarHash
	templ!: string & !=""
	model!: string & !=""
	defaults!: {
		IDPrefix!: ""
		Label!: ""
		MobileLabel!: string & !=""
		Brand!: {LightURL!: "", DarkURL!: "", Alt!: string & !=""}
		Links!: [...{Key!: "", Label!: string & !="", URL!: ""}] & list.MinItems(4) & list.MaxItems(4)
		CurrentKey!: ""
		Notifications!: null
		Account!: null
	}
	verification!: "not-verified"
}

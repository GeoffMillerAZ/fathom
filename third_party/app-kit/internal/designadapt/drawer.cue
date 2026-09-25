package designadapt

import "list"

#DrawerHash: string & =~"^[a-f0-9]{64}$"
#DrawerArtifact: {
	schema_version!: 1
	adapter!: "p08-profile-drawer-v1"
	source!: {
		archive_id!: string & =~"^[a-z0-9-]+@[a-f0-9]{64}$"
		path!: string & !=""
		sha256!: #DrawerHash
		size!: int & >=1 & <=262144
	}
	syntax!: {
		id!: "html-elements-to-templ-v1"
		sha256!: #DrawerHash
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
	rule_sha256!: #DrawerHash
	syntax_sha256!: syntax.sha256
	templ_sha256!: #DrawerHash
	model_sha256!: #DrawerHash
	defaults_sha256!: #DrawerHash
	templ!: string & !=""
	model!: string & !=""
	defaults!: {
		IDPrefix!: ""
		OpenLabel!: string & !=""
		Heading!: string & !=""
		CloseLabel!: string & !=""
		Name!: string & !=""
		Handle!: string & !=""
		OnlineLabel!: ""
		MenuLabel!: string & !=""
		ImageURL!: ""
		ImageAlt!: ""
		ImageDecorative!: true
		Primary!: null
		Secondary!: null
		MenuLinks!: []
		Details!: [...{
			Label!: string & !=""
			Text!: string & !=""
			Kind!: "text" | "paragraph" | "date"
			Date!: string
		}] & list.MinItems(4) & list.MaxItems(4)
	}
	verification!: "not-verified"
}

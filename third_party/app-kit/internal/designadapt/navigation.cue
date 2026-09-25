package designadapt

#NavigationHash: string & =~"^[a-f0-9]{64}$"
#NavigationArtifact: {
	schema_version!: 1
	adapter!: "p06-page-navigation-v1"
	source!: {
		archive_id!: string & =~"^[a-z0-9-]+@[a-f0-9]{64}$"
		path!: string & !=""
		sha256!: #NavigationHash
		size!: int & >=1 & <=262144
	}
	syntax!: {
		id!: "html-to-templ-v1"
		sha256!: #NavigationHash
		go_version!: string & !=""
		html_parser!: "golang.org/x/net@v0.54.0"
		required_templ!: "github.com/a-h/templ@v0.3.1020"
	}
	rule_sha256!: #NavigationHash
	syntax_sha256!: syntax.sha256
	templ_sha256!: #NavigationHash
	model_sha256!: #NavigationHash
	defaults_sha256!: #NavigationHash
	templ!: string & !=""
	model!: string & !=""
	defaults!: {
		Items!: [{Key!: "page-1", Label!: string & !="", URL!: "#"},
			{Key!: "page-2", Label!: string & !="", URL!: "#"},
			{Key!: "page-3", Label!: string & !="", URL!: "#"},
			{Key!: "page-4", Label!: string & !="", URL!: "#"}]
		CurrentKey!: "page-1" | "page-2" | "page-3" | "page-4"
		NavLabel!: string & !=""
		SelectLabel!: string & !=""
	}
	verification!: "not-verified"
}

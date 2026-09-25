package designadapt

#HeroHash: string & =~"^[a-f0-9]{64}$"
// Generation envelope only. No passed/certified compatibility state exists.
#HeroArtifact: {
	schema_version!: 1
	adapter!: "p01-hero-v1"
	source!: {
		archive_id!: string & =~"^[a-z0-9-]+@[a-f0-9]{64}$"
		path!: string & !=""
		sha256!: #HeroHash
		size!: int & >=1 & <=262144
	}
	syntax!: {
		id!: "html-to-templ-v1"
		sha256!: #HeroHash
		go_version!: string & !=""
		html_parser!: "golang.org/x/net@v0.54.0"
		required_templ!: "github.com/a-h/templ@v0.3.1020"
	}
	source_assets!: {
		logo_light_url!: string & !=""
		logo_dark_url!: string & !=""
		image_url!: string & !=""
	}
	rule_sha256!: #HeroHash
	syntax_sha256!: syntax.sha256
	templ_sha256!: #HeroHash
	model_sha256!: #HeroHash
	defaults_sha256!: #HeroHash
	css_sha256!: #HeroHash
	templ!: string & !=""
	model!: string & !=""
	css!: string & !=""
	defaults!: {
		LogoLightURL!: ""
		LogoDarkURL!: ""
		LogoAlt!: string & !=""
		Announcement!: string & !=""
		AnnouncementLabel!: string & !=""
		AnnouncementURL!: ""
		Heading!: string & !=""
		Introduction!: string & !=""
		PrimaryLabel!: string & !=""
		PrimaryURL!: ""
		SecondaryLabel!: string & !=""
		SecondaryURL!: ""
		ImageURL!: ""
		ImageAlt!: ""
		ImageDecorative!: true
	}
	verification!: "not-verified"
}

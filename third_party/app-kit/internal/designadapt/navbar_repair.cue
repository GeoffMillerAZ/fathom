#NavbarRepairArtifact: {
	schema_version!: 1
	adapter!: "p09-navbar-repair-v1"
	base!: #NavbarArtifact
	rule_sha256!: #NavbarHash
	css_sha256!: #NavbarHash
	css!: string & !=""
	routes_go_sha256!: #NavbarHash
	routes_go!: string & !=""
	routes_templ_sha256!: #NavbarHash
	routes_templ!: string & !=""
	wrapper_attribute!: "data-appkit-navbar-repair"
	verification!: "not-verified"
}

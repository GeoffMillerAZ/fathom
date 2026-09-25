package designadapt

// Independent opt-in companion. The base artifact/CLI envelope is unchanged.
#DirectoryEnhancedArtifact: {
	schema_version!: 1
	adapter!: "p04-directory-enhanced-v1"
	base!: #DirectoryArtifact
	rule_sha256!: #DirectoryHash
	templ_sha256!: #DirectoryHash
	model_sha256!: #DirectoryHash
	css_sha256!: #DirectoryHash
	templ!: string & !=""
	model!: string & !=""
	css!: string & !=""
	verification!: "not-verified"
}

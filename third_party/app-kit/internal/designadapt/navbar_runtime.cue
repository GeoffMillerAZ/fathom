package designadapt

#NavbarRuntimeHash: string & =~"^[a-f0-9]{64}$"
#NavbarRuntimeArtifact: {
	schema_version!: 1
	adapter!: "p09-navbar-runtime-focus-v1"
	repair_sha256!: #NavbarRuntimeHash
	original_runtime_sha256!: "120ff5b81c8b3bb7e6de13d8b83a92da984c4312aa017b7392c9cb876cd65013"
	runtime_sha256!: #NavbarRuntimeHash
	rule_sha256!: #NavbarRuntimeHash
	runtime!: string & !=""
	wrapper_attribute!: "data-appkit-navbar-repair"
	verification!: "not-verified"
}

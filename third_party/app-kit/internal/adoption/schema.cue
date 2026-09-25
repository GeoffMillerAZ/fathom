package adoption

#Text: string & =~"[^\\s]"
#Hash: string & =~"^[0-9a-f]{64}$"
#Selection: {
	schema_version!: 1 | 2
	consumer!: string & =~"^[a-z0-9]+(-[a-z0-9]+)*$"
	// Catalog validates profile/layer vocabulary and exact capability fit.
	profile!: #Text
	layers!: [...#Text]
	capabilities!: [...#Text]
	if schema_version == 1 {
		catalog_revision!: #Hash
	}
	if schema_version == 2 {
		capability_revisions!: [...{
			id!: #Text
			sha256!: #Hash
		}]
	}
	contract!: #Text
	module?: {
		file!: #Text
		version!: #Text
		allow_local_replace!: bool
	}
	artifacts!: [...{
		path!: #Text
		sha256!: #Hash
	}]
	agents!: bool
}

package catalog

#Text:     string & =~"[^\\s]"
#Layer:    "L0" | "L1" | "L2" | "L3a" | "L3b" | "L4"
#Profile:  "portable" | "go-web" | "static-web" | "react-web" | "cli"
#Kind:     "token" | "primitive" | "recipe" | "reference"
#Maturity: "experimental" | "available" | "proven"

// Metadata describes integration and evidence; it does not contain source bodies.
#Entry: {
	id!:      string & =~"^[a-z0-9]+([._/-][a-z0-9]+)*$"
	name!:    #Text
	kind!:    #Kind
	summary!: #Text
	tasks!: [#Text, ...#Text]
	// Terminal runtime entries can have no applicable web adoption layer.
	layers!: [...#Layer]
	profiles!: [#Profile, ...#Profile]
	dependencies!: [...#Text]
	source!: {
		path!: #Text
		symbols!: [...#Text]
	}
	usage!: #Text
	states!: [...#Text]
	app_owns!: [...#Text]
	maturity!: #Maturity
	proof!: [...#Text]
	gaps!: [...#Text]
	provenance!: {
		source!:   #Text
		license!:  #Text
		audience!: #Text
	}
	consumers!: [...#Text]
	// Empty means no browser preview is available, including terminal entries.
	preview!: string
}

#Catalog: {
	schema_version!: 1
	entries!: [...#Entry]
}

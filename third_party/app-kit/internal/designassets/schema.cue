package designassets

#Hash: string & =~"^[a-f0-9]{64}$"
#Path: string & !=""
#Dependencies: {[string]: string}
#File: {
	path!:   #Path
	size!:   int & >=0 & <=33554432
	sha256!: #Hash
	kind!:   "source" | "manifest" | "readme" | "license" | "theme" | "asset" | "documentation" | "config" | "sensitive" | "other"
}
#Project: {
	path!:                  #Path
	name!:                  string
	version!:               string
	dependencies!:          #Dependencies
	dev_dependencies!:      #Dependencies
	peer_dependencies!:     #Dependencies
	optional_dependencies!: #Dependencies
}
#Archive: {
	id!:         string & =~"^[a-z0-9-]+@[a-f0-9]{64}$"
	family!:     string & =~"^[a-z0-9-]+$"
	filename!:   #Path
	sha256!:     #Hash
	size!:       int & >0 & <=268435456
	kind!:       "template" | "ui-kit" | "ui-blocks"
	vendor!:     "tailwindplus"
	source_url!: string & =~"^https://tailwindcss.com/plus/"
	files!: [...#File]
	projects!: [...#Project]
}
#Catalog: {
	schema_version!: 1
	revision!:       #Hash
	archives!: [...#Archive]
}

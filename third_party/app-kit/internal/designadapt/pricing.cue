package designadapt

import "list"

#Hash: string & =~"^[a-f0-9]{64}$"
// This is a generated-source envelope, NOT a target compatibility receipt.
// It has no passed/verified state and cannot enter verified-target discovery.
#PricingArtifact: {
	schema_version!: 1
	adapter!: "p02-pricing-v1"
	source!: {
		archive_id!: string & =~"^[a-z0-9-]+@[a-f0-9]{64}$"
		path!: string & !=""
		sha256!: #Hash
		size!: int & >=1 & <=262144
	}
	syntax!: {
		id!: "html-to-templ-v1"
		sha256!: #Hash
		go_version!: string & !=""
		html_parser!: "golang.org/x/net@v0.54.0"
		required_templ!: "github.com/a-h/templ@v0.3.1020"
	}
	rule_sha256!: #Hash
	syntax_sha256!: syntax.sha256
	templ_sha256!: #Hash
	model_sha256!: #Hash
	defaults_sha256!: #Hash
	css_sha256!: #Hash
	templ!: string & !=""
	model!: string & !=""
	css!: string & !=""
	defaults!: {
		Heading!: string & !=""
		Introduction!: string & !=""
		Plan!: string & !=""
		Description!: string & !=""
		Included!: string & !=""
		Features!: [...string & !=""] & list.MinItems(1) & list.MaxItems(12)
		Payment!: string & !=""
		Price!: string & !=""
		Currency!: string & !=""
		Action!: string & !=""
		Note!: string & !=""
		URL!: ""
	}
	verification!: "not-verified"
}

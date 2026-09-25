package designadapt

import "list"

#FAQHash: string & =~"^[a-f0-9]{64}$"

#FAQArtifact: {
	schema_version!: 1
	adapter!: "p03-faq-v1"
	source!: {
		archive_id!: string & =~"^[a-z0-9-]+@[a-f0-9]{64}$"
		path!: string & !=""
		sha256!: #FAQHash
		size!: int & >=1 & <=262144
	}
	syntax!: {
		id!: "html-elements-to-templ-v1"
		sha256!: #FAQHash
		go_version!: string & !=""
		html_parser!: "golang.org/x/net@v0.54.0"
		required_templ!: "github.com/a-h/templ@v0.3.1020"
	}
	runtime!: {
		source_reference!: {
			package!: "@tailwindplus/elements"
			version!: "1.0.22"
			esm_sha256!: "120ff5b81c8b3bb7e6de13d8b83a92da984c4312aa017b7392c9cb876cd65013"
			tgz_sha256!: "fc684f17d960fb60dadfdf9a0b49ba8fd295af4b8fd1715a26cd20b832ea245b"
			delivery!: "host-provided-module"
		}
		native_details!: null
	}
	rule_sha256!: #FAQHash
	syntax_sha256!: syntax.sha256
	templ_sha256!: #FAQHash
	model_sha256!: #FAQHash
	defaults_sha256!: #FAQHash
	css_sha256!: #FAQHash
	templ!: string & !=""
	model!: string & !=""
	css!: string & !=""
	defaults!: {
		IDPrefix!: string & !=""
		Heading!: string & !=""
		Items!: [...{
			Key!: string & !=""
			Question!: string & !=""
			Answer!: string & !=""
			Open!: bool
		}] & list.MinItems(6) & list.MaxItems(6)
	}
	verification!: "not-verified"
}

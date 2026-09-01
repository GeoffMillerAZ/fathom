// Package tailwindplus is the CUE source of truth (D7) for the shape of the
// TailwindPlus corpus, authored to match the REAL dump (downloader v3.0.0,
// dump 2026-02-02-230729) rather than the idealized payload in the original
// design doc. Differences that matter, verified against the 336MB dump:
//
//   - The dump's snippet payload carries `language` ("html" | "jsx", the
//     syntax) and an integer `version` that is the Tailwind major (3 | 4).
//   - The ACTUAL framework lives in the snippet's `name` field ("html" |
//     "vue" | "react"): language "html" backs both html and vue templates,
//     "jsx" backs react. normalize copies `name` into `framework` and sets
//     the variant's display name from the taxonomy. `full_name` is the joined
//     taxonomy path. Both framework and full_name are added before a
//     component reaches this schema.
//   - Ecommerce variants arrive with a JSON null mode; normalize maps null →
//     "none" before validation, so "none" only ever appears under Ecommerce.
//
// Generated Go types are deliberately NOT produced via `cue exp gengotypes`
// here: the pipeline handwrites its Go structs (the D7-documented fallback)
// and this schema is the runtime validation law via cuecontext.
package tailwindplus

#Language:        "html" | "jsx"
#Framework:       "html" | "vue" | "react"
#TailwindVersion: 3 | 4
#Mode:            "light" | "dark" | "system" | "none"
#Section:         "Application UI" | "Ecommerce" | "Marketing"

#Variant: {
	full_name:        string & !=""
	name:             string & !=""
	code:             string & !=""
	language:         #Language
	framework:        #Framework
	tailwind_version: #TailwindVersion
	mode:             #Mode
	supportsDarkMode: bool
}

#Component: {
	section:   #Section
	group:     string & !=""
	category:  string & !=""
	name:      string & !=""
	full_name: string & !=""
	variants: [#Variant, ...#Variant] // at least one

	// Ecommerce ships unthemed; everything else never ships "none".
	if section == "Ecommerce" {
		variants: [...{mode: "none"}]
	}
	if section != "Ecommerce" {
		variants: [...{mode: "light" | "dark" | "system"}]
	}
}

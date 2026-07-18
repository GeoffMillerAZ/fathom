// Package taxonomymanager — valid test data that must unify with schema.cue.
package taxonomymanager

import "github.com/geoffmilleraz/agentic-kit/foundations/schema/taxonomy-manager:taxonomymanager"

// validRootCategory demonstrates a well-formed root category (no parent).
validRootCategory: taxonomymanager.#Category & {
	ID:     "engineering"
	Parent: ""
	Label:  "Engineering"
}

// validChildCategory demonstrates a well-formed child category.
validChildCategory: taxonomymanager.#Category & {
	ID:     "platform-eng"
	Parent: "engineering"
	Label:  "Platform Engineering"
}

// validTaxonomy_Empty demonstrates a taxonomy with no categories yet (valid
// on construction; categories are added via the Manager API).
validTaxonomy_Empty: taxonomymanager.#Taxonomy & {
	Name:       "cost-center"
	Version:    "1.0.0"
	Categories: []
}

// validTaxonomy_WithCategories demonstrates a taxonomy with a two-level
// hierarchy.
validTaxonomy_WithCategories: taxonomymanager.#Taxonomy & {
	Name:    "cost-center"
	Version: "1.0.0"
	Categories: [
		{ID: "engineering", Parent: "", Label: "Engineering"},
		{ID: "platform-eng", Parent: "engineering", Label: "Platform Engineering"},
		{ID: "data-eng", Parent: "engineering", Label: "Data Engineering"},
	]
}

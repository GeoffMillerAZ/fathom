// Package project defines #Project, the one closed value a repository's
// `.fathom/*.cue` files compose into (docs/design/fathom-dir.md section 5,
// ADR-061). Each file keeps its name and meaning; the schema keys the files
// by their top-level key, so today's files are valid as written because
// they already use disjoint keys.
//
// Composition by key:
//   capabilities, budgets   .fathom/capabilities.cue
//   packs, concerns         .fathom/concerns.cue
//   bindings, exclude_packs .fathom/modules.cue    (schema/binding)
//   guards, layer_rules     .fathom/guard.cue      (schema/guard)
//   orientation, generated  .fathom/pack.cue       (schema/techpack)
//
// Mapped by FILE NAME, not by key (flat content, retired by a later merge):
// `.fathom/risk.cue` is a schema/risk #RiskTable and `.fathom/learning.cue`
// a schema/learning #Learning. `task cue-vet` vets those two on their own.
//
// The capabilities and concerns shapes are mirrored from the embedded
// internal/capability/capability_schema.cue and
// internal/custodian/custodian_schema.cue (those files carry no package
// and cannot be imported); the Go loaders there remain the stricter gate.
// Keys the design adds later (formats) are added here with the change that defines their schema.
package project

import (
	"github.com/geoffmilleraz/fathom/schema/binding"
	"github.com/geoffmilleraz/fathom/schema/guard"
	"github.com/geoffmilleraz/fathom/schema/techpack"
)

#Project: {
	capabilities?: [string]: {on?: bool}
	budgets?: #Budgets

	packs?: [...#PackName]
	concerns?: [string]: #Concern

	bindings?: [...binding.#Binding]

	// exclude_packs names tech packs (by pack name) that no consumer serves
	// (docs/design/fathom-dir.md section 7.2). It lives in modules.cue: a
	// committed, reviewable opt-out, like a binding is an opt-in.
	exclude_packs?: [...(string & !="")]

	guards?:       [...guard.#Guard]
	layer_rules?: [...guard.#LayerRule]

	// The project pack (.fathom/pack.cue): the keys a tech pack shares
	// with the project (schema/techpack). Orientation is the project's
	// alone; a kit pack may not carry it.
	orientation?: [...techpack.#Orientation]
	// generated declares outputs produced from sources (the tech pack
	// shape): integrity reads it before its same-directory heuristic.
	generated?: [...techpack.#Generated]
}

#Budgets: {
	list?:      int & >0
	composite?: int & >0
	map?:       int & >0
}

#PackName: "claude-code" | "codex" | "lefthook" | "taskfile"

// #Concern and #Check are schema/techpack's: the custodian's contracts are
// one shape whether a project or a tech pack carries them.
#Concern: techpack.#Concern
#Check:   techpack.#Check

# azure-policy — provenance and divergences

LB-Q-azure-policy (wave pc), design `docs/design/language-breadth.md` §4, §13. A doc-engine
dialect (no `tags.scm`, no grammar, no incumbent: a definition was generic JSON, read as bare
keys). The decode is `encoding/json` through the doc engine (`decode: json`). The format is the
Azure Policy definition structure (<https://learn.microsoft.com/azure/governance/policy/concepts/definition-structure>);
nothing is copied from a source tree, the selectors are written from it and from the files of
Azure/azure-policy (MIT).

## How a file is claimed: by content, never by name

The manifest claims no basename, glob or extension. `detect.content_sniff` matches a `policyRule`
key, a `policyDefinitions` key, or a bare `{"if": …, "then": {"effect": …}}` rule.
`fusion.cloudPolicyDispatch` adds a decode check: a definition needs its rule's effect, an
initiative needs a member, so a JSON file that only names `policyRule` as a string stays generic
JSON. A parameters-only file (`azurepolicy.parameters.json`) names no rule and is not claimed.
Earlier signals win exactly as for the other cloud-policy dialects.

## What it models

- **A definition** (`AzurePolicyDefinition`, `policy`): `properties` holding a `policyRule` (the
  ARM shape; the entity is the `properties` object), a root `policyRule`, or a bare `if`/`then`
  rule; named by the file, signed by its `displayName`, else its `mode`.
- **An initiative** (`AzurePolicyInitiative`, `policy`): `properties` holding `policyDefinitions`.
- **Parameters** (`AzurePolicyParameter`, `variable_decl`): `properties.parameters.*` (else a root
  `parameters.*`), named by key, signed by `type`.
- **The effect** (`AzurePolicyEffect`, `rule`): the policyRule's `then`, named by its `effect` as
  written (`deny`, or `[parameters('effect')]`).
- **Initiative members** (`AzurePolicyMember`, `key`): each `policyDefinitions[]` item, named by
  its `policyDefinitionReferenceId`, else its `policyDefinitionId`.
- **References** (`Calls`, L2, intra-file, `declared_only`): `parameters('x')` written in any
  string under a definition or an initiative (a doc-engine `scan`, an additive option) to the
  parameter the file declares; a parameter the file does not declare (a rule file whose
  parameters live in a sibling file) is not a reference.
- **`policy_ref`** (cross-file, emitted, resolved by item k8s's `internal/fusion/policyref.go`):
  `field: type` with `equals`, `in` (each item) or `like` naming `Microsoft.X/y`, edge
  `policy_ref`, strategy `resource-type`, Raw and Target the type as written (a wildcard `like`
  is emitted too). ARM and Bicep providers arrive with wave iac; until then these references are
  unresolved (undeclared) and that is disclosed by the resolver, not hidden here. A resource type
  written only in `then.details.type` (an audit-if-not-exists lookup) is not emitted.

## What is not modelled, and why

- **Field aliases**, `count`, `exists`, `[field()]` and every other expression: only the
  `[parameters('x')]` name is read.
- **A parameter's references from an initiative member to the definition it names**
  (`policyDefinitionId`): a built-in is not in the repository, a custom one is named by a
  subscription path, not a file.
- **A root `parameters` map beside `properties`** (3 definitions in the pinned subject, a
  sample whose parameters sit outside `properties`) is read as parameters although the Azure
  schema places them under `properties`: the dialect reads what is written.

## Calibration oracle

`calibration/oracles/azure-policy/azure_policy_oracle.py` (bench-only), tier C, Python standard
library: a JSON walk with its own line-tracking JSON reader, written apart from the dialect, over
the structure the definition schema names. No Azure tool evaluates a definition offline (the
Azure Resource Graph and Policy services are online), and the JSON schema validates shape, not
the entities. Tier C caps every metric at **good**. Subject: Azure/azure-policy at
7b0fa25ac055d8c3001d5205cd009cd119a289a5 (MIT), `calibration/corpus.d/azure-policy.yaml`.

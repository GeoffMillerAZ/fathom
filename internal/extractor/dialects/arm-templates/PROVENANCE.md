# arm-templates — provenance and divergences

LB-Q-arm-templates, design `docs/design/language-breadth.md` §4, §13 (wave iac). A doc-engine
dialect (no `tags.scm`, no grammar, no incumbent: a template was read by the generic JSON
extractor as bare keys). The decode is `jsonc`: comments and trailing commas, which ARM accepts.

## How a file is claimed: by content, never by name

The manifest claims no basename, glob or extension. `doc.json`'s `detect.content_sniff` is a
`"$schema"` naming `deploymentTemplate.json` (and its subscription-, managementGroup- and
tenant-level variants) or `deploymentParameters.json`; fusion's `appFrameworkDispatch`
(`internal/fusion/armdispatch.go`) claims a `.json` the sniff matches only when it decodes to a
template whose root `$schema` names a deployment template, or to a parameters file whose root
`$schema` names `deploymentParameters.json` and that sets a parameter. A document that only
embeds a template (an Azure Policy definition's deployIfNotExists, wave pc's format) has no root
`$schema` and is never one; a policy resource inside a template stays ARM.
`fusion.AppFrameworkContentDialect` is the same decision, which the harness measures on.

## What it models

- **The template** (`ARMTemplate`, `package`), named by the file, signed by its `$schema`.
- **Parameters** (`ARMParameter`, `variable_decl`), declarations with a `type`; a parameters
  file's entries (`ARMParameterValue`, `variable_decl`), a `value` or a Key Vault `reference`.
- **Variables** (`ARMVariable`, `variable`), a `copy` array's named variables included.
- **Resources** (`ARMResource`, `resource`) named by `name` as written (an expression stays an
  expression: `[variables('vnetName')]`), signed by `type`; nested child resources and the
  resources of an inline nested template (any depth) too; a languageVersion 2.0 template's
  resources by their symbolic names. An inline nested template is modelled through its
  deployment resource and its own declarations (parameters, variables, resources, outputs,
  functions), each an entity.
- **Outputs** (`ARMOutput`, `output`) and **user functions** (`ARMFunction`, `function`).
- **References** (Calls, L2; every one `declared_only`, read with the doc engine's `pattern`,
  LB-F145): `parameters('x')`, `variables('y')` and a user function `ns.fn(` in any string of a
  resource's own fields (never its `dependsOn`, which names resources), an output, a variable or
  a parameter; a resource named by `dependsOn` (as written, `concat('<type>/', name)`, a
  `<type>/<name>` literal), by a one-name `resourceId('T', x)` and by `reference(x)` — `x` read
  back into the name as written (`fmt:[{$}]`). A one-name `resourceId(...)` or `reference(...)`
  names a resource, so the parameter or variable inside it is not also a reference (Bicep's
  decompiler reads it as the resource). A value naming the referring entity itself is none.
- **Linked templates** (L3, `fusion.ResolveDocXrefs`, from the deployment resource to the
  linked template's entity): `templateLink.relativePath`; a `templateLink.uri` of the form
  `uri(<base>, '<file>.json')` or `concat(parameters('<base>'), '<file>.json')`, a file relative
  to the template; and either form held by a variable the uri names (`[variables('netUri')]`,
  the doc engine's `via`). Any other uri (an https URL, an `_artifactsLocation` whose file part is
  a concat with the SAS token) resolves to nothing and is counted by the oracle.

## What is not modelled, and why

- A resource reached by a multi-name `resourceId('T', a, b)` (a child resource or a
  sub-resource), and a child's parent reached through the first segment of its name
  (`format('{0}/{1}', parameters('vnet'), 'default')`): the doc engine composes one capture,
  so the parent's name cannot be rebuilt (the L2 gap, LB-Q-arm-templates-rest).
- A string with a raw line break or tab (a multi-line `scriptContent`, which ARM accepts but
  JSON does not): the file decodes to nothing and is out of the measured scope.
- `copy` loops' instance names, `condition`s, and `reference()` of a resource outside the file.

## Calibration oracle

`calibration/oracles/arm-templates/arm_oracle.py` (bench-only), with the Bicep CLI v0.47.16
(MIT): `bicep decompile` reads each template with Azure's own template and expression libraries
and writes every declaration as Bicep, resolving each expression to the declaration it names
and a linked template to a module path (tier A); `bicep decompile-params` for parameters files.
Subjects: `calibration/corpus.d/arm-templates.yaml`.

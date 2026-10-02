# pulumi-yaml-oracle (tier A)

The language-breadth harness's oracle for the `pulumi-yaml` doc-engine dialect (LB-Q-pulumi-yaml-rest,
design `docs/design/language-breadth.md` §1.3, §10). A separate Go module (`go.mod`): Pulumi's own code,
linked offline as libraries, never imported by fathom, never a product dependency:

- `github.com/pulumi/pulumi-yaml` v1.36.0 (Apache-2.0), the YAML language host the pulumi CLI 3.247.0 ships
  (`go version -m pulumi-language-yaml`: `-X …/version.Version=v1.36.0`);
- `github.com/pulumi/pulumi/sdk/v3` v3.245.0 (Apache-2.0), pulumi-yaml's own pin of the CLI's project code;
- `github.com/goccy/go-yaml` v1.19.2 (MIT), an independent YAML reader, for lines only.

```
cd calibration/oracles/pulumi-yaml-go && GOTOOLCHAIN=local go build -o <scratch>/pulumi-yaml-oracle . &&
  <scratch>/pulumi-yaml-oracle -subject <checkout> -files-out <scratch>/files.txt > <scratch>/pulumi-yaml-<subject>.json
indexbench lang-oracle -dialect pulumi-yaml -subject <checkout> -subject-name <name> \
    -tool-oracle <scratch>/pulumi-yaml-<subject>.json -only "<every Pulumi.yaml and Pulumi.yml, comma-separated>" \
    -out docs/bench/readings/lang/pulumi-yaml.json
```

## The instruments

- **The project: Pulumi's project loader.** `workspace.LoadProject`, the function the pulumi CLI loads every
  `Pulumi.yaml` with (the project JSON schema, then the typed `Project` and its `Validate`). A file it refuses
  yields no entity (stderr names it). It attests the project, a `package` named by `name` when the typed
  project has a runtime (the dialect's rule), and each `config` key, a `variable_decl`.
- **Templates: `pulumi new`'s own instantiation.** Before loading, each file is copied by the CLI's
  `workspace.CopyTemplateFiles` (the step `pulumi new` runs), which replaces `${PROJECT}` and `${DESCRIPTION}`
  and leaves a file without them unchanged. pulumi/templates writes `name: ${PROJECT}`, which pulumi-yaml
  refuses as written (an interpolation where a string is required); instantiated, it loads. The oracle
  substitutes a sentinel name and maps it back to `${PROJECT}`, the name as written, which the dialect keeps.
- **The program: pulumi-yaml's template parse.** `pulumiyaml.LoadYAMLBytes`, the parse the YAML language
  host runs: each `variables` entry (`variable`), each `resources` entry with a `type` (`resource`), each
  `outputs` entry (`output`), read off the typed `TemplateDecl`. A template with error diagnostics yields no
  program entity (stderr names it and the diagnostic).
- **References: pulumi-yaml's interpolation grammar.** Every `SymbolExpr` and `InterpolateExpr` part
  pulumi-yaml parsed in a resource, a variable or an output (properties, options, `fn::` builtins, keys;
  `$${` is its literal), from the entity to the resource, variable or config key its root names. The
  builtin `pulumi`, an undeclared root and a declaration naming itself (an output may) are not references:
  the dialect's rule.
- **Lines: an independent reader.** goccy/go-yaml (its own scanner and parser) reads each key's line and its
  value's last line from the file as written. pulumi-yaml's YAML layer is `gopkg.in/yaml.v3`, the decoder the
  dialect's doc engine reads with too, while the program model and the interpolation grammar are Pulumi's;
  each key line is cross-checked against pulumi-yaml's own syntax range wherever instantiation changed
  nothing (mismatches on stderr: 0 on both subjects).
- Stack files (`Pulumi.<stack>.yaml`) are counted and not attested; neither subject commits one.

## Why tier A

§1.3's A rung is "the tool's own evaluation … the format's reference parser". Every entity and reference
here is one Pulumi's own loaders return (the project loader the CLI runs, the template parse and
interpolation grammar the YAML language host runs), not a rule written against the format; the relabel
from the tier-C reference parse (`calibration/oracles/pulumi-yaml/pulumi_yaml_oracle.js`, kept) cites that
definition. The pulumi CLI 3.247.0, run with a local file backend only (`PULUMI_BACKEND_URL=file://<scratch>`),
cross-checks the loader's refusal: `pulumi config` on pulumi/examples `yaml-math-and-objects` fails with the
same `yaml: unmarshal errors: line 38: cannot unmarshal !!seq into string`.

## What it showed (wave ox)

pulumi/examples 3e98795a: 364 project files, one refused by the project loader (`yaml-math-and-objects`,
above), 543 definitions, 217 references. pulumi/templates 2caa4508: 227 project files, one program refused
by pulumi-yaml (`github-yaml`: `resources.type must be an object`, a resource named `type`), 464 definitions,
273 references. Both refused files stay in the measured set, so fathom's 15 definitions there count against
it.

# Design prototypes

These are bounded, backendless design experiments. They are not kit runtime APIs,
production applications, policy enforcement or adopted consumer interfaces.

| Prototype | Questions | Status and review |
|---|---|---|
| [Governed workbench](governed-workbench/README.md) | One shell for human review and prose? How much capability fits inline, in a panel, in a split view, or in a full workspace? | Exploratory; review 2026-11-21 or next release |

Each experiment owns a small static bundle, `prototype.json`, synthetic scenarios,
design questions and an exit condition. Generated screenshots, recordings, caches
and build outputs stay outside Git. Do not import or embed these files in shipped
runtime packages. Promotion means implementing an accepted design in its owning
module with that module's tests and consumer evidence, not renaming a prototype.
When questions are answered, retain only a deliberately useful reference or remove
the bundle and update this index. Git retains its history.

App-kit's experimental `cmd/prototype-lab` scaffolds, checks and serves this shape.
Its structural check is not semantic, accessibility or security certification.

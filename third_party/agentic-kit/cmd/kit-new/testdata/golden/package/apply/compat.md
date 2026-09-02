# API stability audit + compatibility promise

## Classification counts (for sign-off)

| Classification | Count |
|---|---|
| `stable` | 3 |
| `experimental` | 0 |

## Classification table

| Package | Classification | Rationale |
|---|---|---|
| `foundations/alpha/gadget` | stable | existing |
| `foundations/alpha/widget-gizmo` | experimental | Scaffolded by kit-new; zero consumers proven yet — consumer: kits/beta. Promote to stable only through a real consumer in `task downstream-check` (`docs/intent/repo.md` §Promote). |
| `foundations/data/store` | stable | existing |
| `kits/beta` | stable | existing |

## The compatibility promise

Prose.

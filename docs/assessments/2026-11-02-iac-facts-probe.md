# IaC facts: which provider facts the exports actually carry

Measured 2026-11-02 on `hashicorp/random` 3.7.2 under OpenTofu 1.13.0, and `random` 4.18.0 under Pulumi 3.247.0. One small
provider, chosen because it downloads in seconds. It answers one design question and grades nothing.

## Verdict

The fact an agent most needs before changing infrastructure, that a change destroys and recreates a resource, **is not in
`tofu providers schema -json`**. Nor is it in the bridged Pulumi schema of the same provider. A plan does report it, in a
machine-readable field, with no cloud involved. So a provider's schema export alone cannot deliver this fact, and a
plan is a usable native oracle for grading it from another source. This is why ADR-091 builds catalogs from provider
source as well as exports.

## What was read

`random_id.byte_length` forces replacement in the provider's own behavior:

| Instrument | Result |
|---|---|
| `tofu providers schema -json`, attribute fields across `random_id` | `computed`, `description`, `description_kind`, `optional`, `required`, `type` |
| The whole export, searched for `force_new`, `forceNew`, `requires_replace`, `RequiresReplace"` | no match |
| `pulumi package get-schema random@4.18.0`, `byteLength` | `{"type": "integer"}` |
| The whole Pulumi schema, searched for `replaceOnChanges`, `willReplaceOnChanges` | no match |
| `tofu plan -refresh=false` after editing `byte_length` from 8 to 9 | `random_id.a must be replaced`, `byte_length = 8 -> 9 # forces replacement` |
| `tofu show -json` on that plan | actions `["delete", "create"]`, `replace_paths: [["byte_length"]]` |

Export namespaces present: `provider`, `resource_schemas`, `ephemeral_resource_schemas`, format 1.0.

## Limits

- `random` needs no credentials. Whether AWS, AzureRM and Google plan offline with fake credentials and `-refresh=false`
  is unmeasured: row IF-P3 reads it.
- One bridged provider generalizes to nothing. `random` may be a plugin-framework provider, and bridged SDKv2 schemas may
  carry `willReplaceOnChanges`. Azure Native is a separate schema lineage. Row IF-P4 reads them.
- Running the Pulumi CLI while logged out creates an ephemeral hosted account as a side effect. Probes must set a local
  backend first (`PULUMI_BACKEND_URL=file://<scratch>`).

Earlier size evidence: full exports measured AWS 19,517,134 B, AzureRM 3,204,662 B and Google 9,782,236 B
([feasibility probe](2026-10-26-iac-provider-schema-feasibility.md)). Those are unpruned single-version figures.

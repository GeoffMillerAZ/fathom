# Explicit Unity asset-reference reader

`fathom unity-references` answers one-hop persistent object-reference questions
from a genuinely produced current partial family. It mounts the selected existing
service store read-only, uses its registered ServiceMeta root and revalidates the
producer receipt, active generation, raw/native hashes, authored source/context
membership and bytes on every query. It creates no database, registration,
snapshot table or cached current authority. Immutable producer run directories
retain history; the active marker selects current authority.

```sh
fathom unity-references --store workspace/db/fathom.sqlite --service example \
  --project game/Example --family cart \
  --receipt .artifacts/unity-references/runs/cart/run-02/receipt.json \
  --active .artifacts/unity-references/active/game/Example/cart.json \
  --editor-version 6000.3.15f1 --editor-sha256 <trusted-local-editor-sha256> \
  --target StandaloneOSX --guid <exact-guid> --file-id 9007199254740993 \
  --direction inbound --limit 25 --json
```

There is no `--root` override or automatic acceptance of receipt-selected Editor
pins. Required trusted Editor identity/target values come from the operator's
selected toolchain. All receipt paths are service-relative. GUID plus canonical
signed64 decimal-string fileID identifies an object, including imported
subassets; no name/text search, float conversion or transitive traversal is used.
Direction is `inbound` (default) or `outbound`; limit0 selects25 and the maximum
is100 groups. `--json` is a standard boolean flag (`--json=false` selects text).

The complete compact JSON response is at most16KiB and has no additional newline
beyond that budget. Whole groups may be elided: `total` and `truncated` preserve
the denominator. Text includes bound source/target paths, exact IDs, group counts,
receipt/source/context identities and partial limits. A refused, stale,
incomplete, mismatched, cancelled or overbudget read exits nonzero and prints no
positive response to stdout. A current empty query is an empty selected cohort,
not proof that no Unity object references exist elsewhere.

Raw declaration locations and native property lists are grouped by persistent
identity tuple and remain unpaired where equal-target occurrences collide.
Unbound package targets and native-only properties remain explicit gaps. This
is separate asset provenance: no typed code calls, callback/scene execution,
runtime test certification, art approval or excellent language grade follows.
Complete external package/tool-bundle semantics and interactive Editor state
remain unproven. A new started attempt invalidates the previous current family;
historical receipts are never restamped into current evidence.

The required semantic context schema is `fathom.unity.asset-context/v2`.
Its historical `assets_code` field now binds every Assets `.meta` file, including
Vendor/ThirdParty, folder and importer metadata, in addition to `.cs`, `.asmdef`,
`.asmref` and `.rsp` files. These inputs remain opaque hashes; they do not expand
the parsed asset catalog. All ProjectSettings files are also bound. The existing
8MiB per-file and 64MiB combined bounds remain unchanged. V1 context receipts
remain historical and refuse positive current queries: produce a genuinely new
v2 native run rather than changing a receipt's schema or hashes.

The Go client methods `PrepareUnityAssetReferences` and `UnityAssetReferences`
have the same registered-root and source-current checks. A caller-supplied
prepared DTO cannot bypass them. This initial explicit reader adds no SQLite
tables and no MCP tool or mode; opt-in integration of an existing references
surface is a later separately reviewed compatibility change.

A migrated implementation or authored fixture does not qualify a current v2 producer cohort. Retain historical v1 failures and all input bindings. Review and execute the exact migrated source against a genuinely produced v2 receipt before claiming the adopting repository’s vendor/importer metadata freshness is covered. This reader does not validate native test-assembly MVID relationships.

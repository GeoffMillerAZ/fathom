# harnessctl

`harnessctl config check|explain --registration ABS --project ABS --format json|text`
checks one selected project against a protected registration and its exact signed
layer roster. It emits evidence of authenticated configuration, not permission to
run a tool, disable a hook, or change the host. Exit 0 means the selected snapshot
passed. Exit 1 means a policy contradiction. Exit 2 means source, signature, time,
resource, or output evidence is incomplete. Failure reports contain only fixed
categories and counts. Text output is indented escaped JSON.
`harnessctl help` and `harnessctl config explain --help` require no configuration.
Failure JSON has a fixed `code` and bounded `expectedLayers`, `loadedLayers`,
and `authenticatedLayers` counts. A layer-specific failure may include a
zero-based `sourceOrdinal` into the selected registration. It never includes
the selected source ID or path. Codes are `input_invalid`,
`registration_invalid`, `project_unregistered`, `clock_invalid`,
`source_unavailable`, `source_changed`, `envelope_invalid`,
`signature_invalid`, `binding_mismatch`, `policy_conflict`, and
`preference_denied`.

## Local toy trial

Run from the repository root. The generator accepts only a fresh absolute output
directory. It creates two project roots, one global control catalog, six enrolled
synthetic issuers and five signed sources per project. Each issuer's private key
stays in memory. Shared installation, organization, team and personal sources
live under `sources/`. Each project's signed project source lives in its own
`.agentic/` directory. Every envelope has a matching `*-payload.json` sidecar.

```sh
toy_dir=/absolute/new-toy-directory
go run ./cmd/harnessctl/testdata/generate.go "$toy_dir"
go build -o "$toy_dir/harnessctl" ./cmd/harnessctl
"$toy_dir/harnessctl" help
for project in project-a project-b; do
  "$toy_dir/harnessctl" config check --registration "$toy_dir/registration.json" \
    --project "$toy_dir/$project" --format json
  "$toy_dir/harnessctl" config explain --registration "$toy_dir/registration.json" \
    --project "$toy_dir/$project" --format json
done
cue vet cmd/harnessctl/schema.cue "$toy_dir/registration.json" -d '#Registration'
for source_file in "$toy_dir"/sources/*-payload.json "$toy_dir"/project-*/.agentic/project-payload.json; do
  cue vet cmd/harnessctl/schema.cue "$source_file" -d '#Layer'
done
for source_file in "$toy_dir"/sources/*-envelope.json "$toy_dir"/project-*/.agentic/project-envelope.json; do
  cue vet cmd/harnessctl/schema.cue "$source_file" -d '#Envelope'
done
```

Both projects should report five layers and two controls. `mandatory` remains
sampled at 1,000,000 PPM, holds on fault, permits no maintenance and has no
tuner. The global `review` default is 1,000,000 PPM; organization narrows it to
500,000–900,000, team to 600,000–850,000 with a 75 ms work limit and hold on
fault. Project proposes 650,000, while the higher-priority personal source
proposes 750,000. Effective `review` is 750,000 PPM, 75 ms, hold, no maintenance.
The catalog's `tunableBy` IDs resolve in each project's own layer roster. The
two project sources share the ID `project` but have distinct signed targets and
in-project paths. The shared sources cover both projects without either project
source gaining authority over the other.

Use the built binary to observe exit codes: `help` and authenticated `check`
return 0; a policy contradiction returns 1; incomplete source evidence returns
2. `go run` masks the CLI's distinct nonzero codes behind the launcher status.
For a policy contradiction in this disposable toy, change the protected catalog's
`review.minSamplePPM` to 800,000. The authenticated personal proposal of 750,000
then yields `preference_denied`, exit 1. Restore the minimum to zero before the
next trial. Revoking `issuer-organization` then yields `signature_invalid`, exit
2, with five expected, two loaded, one authenticated layer and ordinal 1:

```sh
python3 - <<'PY'
import json
from pathlib import Path
p = Path('/absolute/new-toy-directory/registration.json')
registration = json.loads(p.read_text())
registration['controls'][1]['minSamplePPM'] = 800000
p.write_text(json.dumps(registration))
PY
/absolute/new-toy-directory/harnessctl config check \
  --registration /absolute/new-toy-directory/registration.json \
  --project /absolute/new-toy-directory/project-a --format json
python3 - <<'PY'
import json
from pathlib import Path
p = Path('/absolute/new-toy-directory/registration.json')
registration = json.loads(p.read_text())
registration['controls'][1]['minSamplePPM'] = 0
next(key for key in registration['keys'] if key['id'] == 'issuer-organization')['revoked'] = True
p.write_text(json.dumps(registration))
PY
/absolute/new-toy-directory/harnessctl config check \
  --registration /absolute/new-toy-directory/registration.json \
  --project /absolute/new-toy-directory/project-a --format json
```

Generation refuses an existing output directory. Re-run with a fresh directory
to restore the positive fixture; no key material can be recovered from its files.

For real adoption, an operator independently protects the registration, enrolled
Ed25519 key roster and dispatcher from candidate writes. Signed layer files may
live inside a candidate project, including `.agentic/`; their availability and
bytes may be candidate controlled. Exact protected pins and issuer signatures
make tampering or withholding hold. Register each project's canonical root,
issuer context, exact source roster, key ID,
revision, payload SHA-256, control coverage and maximum age. Each issuer signs
the exact layer JSON bytes using DSSE PAE and the fixed payload type. The optional
envelope `keyid` is ignored; the registration's `keyId` selects the enrolled key.
Registration control values form the root constraints. Every signed source can
only restrict its covered controls; a source can propose an advisory sample rate
only when the final tuner intersection delegates that source ID. Priority selects
among valid proposals. Mandatory controls remain at full sampling and hold.
Catalog `tunableBy` names resolve against each selected project's local layer IDs;
reuse a source file across projects only when its constraints and proposals are
valid for each selected project.

`check` includes counts, registration hash and epoch, snapshot digest and the
intersection of validity windows. `explain` also includes safe selected IDs,
source revisions and priorities, effective controls and their constraining/tuning
sources. The digest follows HC-4's byte-frozen preimage. Both reports say
`noActionAuthority:true`, `selected-project-only`, and `unobserved` installation.
The config subcommands do not attest OS protection, actor identity, hook installation
or execution. They have no enrollment, signing, network, credential, or write operation.
The read/reread checks detect observed drift but cannot prove atomic capture or
detect ABA. GT9/10/11 own native grants, persistent high-watermarks and dispatch.
The CUE path shape accepts Unix, Windows drive, and UNC absolute syntax; Go
checks canonicality and filesystem identity on the running host. CUE's rune
bound is a structural approximation of Go's 4096-byte path limit.

For focused verification:

```sh
go test -race ./cmd/harnessctl
cue vet cmd/harnessctl/schema.cue cmd/harnessctl/testdata/valid.cue
```

## Exact advisory maintenance

For strict required spawn checks and current-boundary advice sampling, see the
[governed spawn guide](../../docs/harness-spawn-economics.md). It uses the same
explicit maintenance build and protected receiver. It does not install hooks.

An explicit `-tags harness_maintenance` build adds
`maintenance init|prepare|enter|check|end|recover`. The ordinary build reports
this capability as unsupported before input or state access. Read
[the maintenance guide](maintenance.md) before building or qualifying it.
It operates one selected protected receiver. It neither installs hooks nor
disables mandatory controls. The macOS helper is separately built and pinned;
SDK compilation and synthetic tests are not native-authentication qualification.

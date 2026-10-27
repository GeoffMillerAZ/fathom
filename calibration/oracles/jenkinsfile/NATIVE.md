# Jenkins native L1/L2 model audit

The old claim that no tool evaluates a Jenkinsfile offline is withdrawn. Jenkins's
Declarative plugin already parses a typed model and retains Groovy AST source
locations. This diagnostic calls the owner's `Converter.scriptToPipelineDef`,
which stops at CONVERSION, inside a private controller with HTTP/HTTPS disabled.
It never creates a job or executes a subject pipeline. Docker-agent descriptors
are loaded as parser metadata; Docker is never invoked.

The canonical C reading remains unchanged: **163/163 definitions and 133/133
local pairs**. This audit does not compute a new reading or grant a tier. The
native model's nodes are a different population from those dialect units.

## Scope and result

`native-protocol-r1.json` fixes the existing canonical L1/L2 population before
its native run: all 22 declarative examples at the existing pinned commit and
the library's one Jenkinsfile, **23 files / 17,401 bytes**. There is no holdout,
new canonical subject or scope selected from fathom output. The examples' source
bytes are copied unchanged under `.jenkinsfile` staging names, following the
existing shard contract. Scripted pipelines and vars remain the prior tolerated
scope, not a new claim. L3 stays held for D19.

| Population | Inputs accepted | Model nodes | Positioned nodes | Named nodes |
|---|---:|---:|---:|---:|
| examples | 22/22 | 727 | 721 | 193 |
| library | 1/1 | 26 | 26 | 7 |
| total | 23/23 | 753 | 747 | 200 |

The 50-plugin bundle, core 2.568.3, Declarative 2.2293.v6e7193cec599 and private
Temurin 21.0.13+9 are pinned by hashes in `native-bundle-r1.json`. The launcher
checks the Java binary, WAR and exact plugin archive population before startup.
No global tool, dependency or configuration is installed.

The minimum Declarative dependency set first accepted 16/23 inputs. Supplying
pinned Docker-agent and timestamp descriptors accepted 21/23. Jenkins's own
remaining validation errors named three missing tool installations. Three
**declaration-only names** were configured in the private controller, yielding
23/23. Their paths are placeholders, not installed Maven/JDK tools, and no step
runs. The public evidence records only their count and settings hash. Native
model acceptance here is not proof that a pipeline could execute successfully.

`ModelASTScriptBlock` and expression values can be opaque to the Declarative
model. Native nodes include arguments and keys which the dialect does not call
definitions, and default branches which cannot simply be equated with a written
`steps` block. A complete range/ownership/pair comparator is unbuilt. It must
account for every native unit and every canonical definition/pair, preserve
refusals in the denominator, and disclose opaque bodies before any tier-A metric.
No 753-definition, 200-definition or 23-file tier-A support headline is granted.

## Controls and provenance

The four authored inputs yield two valid models, one syntax refusal and one
`no_pipeline`. A top-level file-write expression in the non-pipeline input never
runs. Changing only the authored stage name changes the native model truth;
normalizing that single name makes the remaining model trees identical. The
instrument never reads fathom output, so changing that output cannot change its
truth. These are native API controls, not a sample substituted for the population.

Ten unit tests have zero skips. Three mutants fail then restore GREEN: ignore
plugin drift, silently drop an unknown receipt field, and truncate cap+1 inputs.
Caps are 128 inputs, 1 MiB per source, 8 MiB total source, 100,000 model nodes per
file, 32 MiB raw JSON, 180 seconds and a 768 MiB JVM heap. Unit controls prove
input-count and node-count cap+1 refusal; the other bounds are declared limits,
not separately measured cap+1 claims. Throughput is unmeasured.

The exact design section 13.3.1 manifest recipe runs before and after the owner
probe. The examples subpath has 25 tracked files / 18,490 bytes; three metadata
files / 1,496 bytes are outside the `.groovy` source population, reconciling to
22 / 16,994. The library subpath has 1 / 407. Both manifest digests are unchanged;
there are no symlinks or gitlinks. Subject checkouts are not modified; staging
copies live separately. No generated language input appeared in the subjects.
Raw models, source lists, error text, tool names and controller logs remain private.
`native-measurement-r1.json` carries only counts, hashes and flags.

## Reproduce

This receipt uses macOS arm64 and its pinned JDK binary. Another platform needs
an explicit JVM pin and a new receipt; do not disable the identity check. All
large files belong under ignored workspace. Check `df -h ~` and `uptime` first;
use one job at nice 10, at least 20 GiB free and load at most 6.

```sh
nice -n 10 python3 -B calibration/oracles/jenkinsfile/prepare_native.py sdk --out workspace/artifacts/jenkins-replay-sdk
nice -n 10 python3 -B calibration/oracles/jenkinsfile/prepare_native.py subjects --out workspace/artifacts/jenkins-replay-subjects
```

Using the downloaded JDK at
`workspace/artifacts/jenkins-replay-sdk/jdk-21.0.13+9/Contents/Home/bin/java`:

```sh
SDK=workspace/artifacts/jenkins-replay-sdk
INPUTS=workspace/artifacts/jenkins-replay-subjects
nice -n 10 python3 -B calibration/oracles/jenkinsfile/native_model_probe.py --java "$SDK/jdk-21.0.13+9/Contents/Home/bin/java" --war "$SDK/jenkins.war" --plugins "$SDK/plugins" --inputs calibration/oracles/jenkinsfile/native-fixtures --out workspace/artifacts/jenkins-replay-authored
python3 -B calibration/oracles/jenkinsfile/validate_native_controls.py workspace/artifacts/jenkins-replay-authored/raw.json
nice -n 10 python3 -B calibration/oracles/jenkinsfile/native_model_probe.py --java "$SDK/jdk-21.0.13+9/Contents/Home/bin/java" --war "$SDK/jenkins.war" --plugins "$SDK/plugins" --inputs "$INPUTS/inputs" --out workspace/artifacts/jenkins-replay-first
python3 -B calibration/oracles/jenkinsfile/prepare_native.py tools --log workspace/artifacts/jenkins-replay-first/controller.log --out "$INPUTS/tool-declarations.json"
nice -n 10 python3 -B calibration/oracles/jenkinsfile/native_model_probe.py --java "$SDK/jdk-21.0.13+9/Contents/Home/bin/java" --war "$SDK/jenkins.war" --plugins "$SDK/plugins" --inputs "$INPUTS/inputs" --tool-declarations "$INPUTS/tool-declarations.json" --out workspace/artifacts/jenkins-replay-final
python3 -B calibration/oracles/jenkinsfile/prepare_native.py after --out "$INPUTS"
python3 -B -m unittest discover -s calibration/oracles/jenkinsfile -p 'test_native_model_probe.py'
python3 -B calibration/oracles/jenkinsfile/native_receipt_controls.py --out workspace/artifacts/jenkins-replay-controls
```

The measured preparation used curl for the checksum-pinned SDK and Jenkins's
plugin manager 2.14.0 to resolve the initial dependency sets; the final 50 archives
are now locked individually for direct reproduction. The convenience SDK fetcher
was added afterward and was not independently redownloaded end-to-end. The owner
runs, identity checks, authored controls and manifest checks above were executed.

## Candidate search

`candidate-audit-r1.json` records all five X-R4 source groups, searched sources
and access gaps; its `candidate-inventory-r1.json` records the full named
versions, public commit pins, licenses and dispositions. Jenkins native model work
is an available route, not an absence ceiling. gvy really emits SCIP/LSIF, but its
pinned index exporter visits class/method definitions and leaves body traversal
unimplemented; it does not enumerate this Pipeline DSL population. Ordinary
independent language-server analysis stays C under P-T1. Runtime FlowNodes and
mock/interceptor traces describe exercised calls, not all written branches.

Check the committed source pins without reinstalling an SDK:

```sh
python3 -B calibration/oracles/jenkinsfile/check_native_receipt.py
```

When retaining private raw outputs, add `--canonical-raw <final/raw.json>` and
`--authored-raw <authored/raw.json>` to replay their exact hashes and summaries.
The published receipt contains only their hashes and aggregate counts, not raw
subject names or model text. The measured SDK, clones and controller builds have
been pruned; replay preparation creates fresh private copies.

# Windows contributor setup

Status: Proposed contributor route under CT-D7. Native Windows runtime support is unknown.
The supported operating environments documented today are macOS and Linux, and Windows is
explicitly untested (`docs/early-testers.md:15-18`). Use WSL2 for lanes. This is a prescribed
route awaiting the CTR-8 witness, not a claim that WSL2 passed the gate.

## Start as a steward

Use git, gh and an agent to draft issues and review their evidence. No daemon or full build is
needed to write a draft. Follow [the lane guide](contributor-lanes.md), step 1a. Independent
red-proof evidence is required before `ready`. Keep the issue draft if that evidence is unavailable.
Authenticate gh with your own account. Outside contributors fork, then propose pull requests.
No shared API credential is required by this model.

## Run a lane in WSL2

Use a Linux environment under WSL2. Keep the clone and its `workspace/` in the Linux filesystem.
Run git, gh, the agent, builds and proofs in that same environment. Check the issue's machine
requirement before claiming. A macOS-specific reading still requires macOS.

Install git, gh, a C compiler, rsync and the pinned Go toolchain in that environment. The repository
requires its `go.mod` toolchain, currently `go1.27.3` (`go.mod:3-5`). Tool pins and the separate tool
build toolchain are in `scripts/cloud-tools.env:12-29`. Read that file instead of upgrading pins.
The documented gate tools are Task, CUE and lefthook (`docs/early-testers.md:85-94`).

From the clone, after the tools are installed:

```sh
. scripts/cloud-tools.env
GOTOOLCHAIN="$TOOLS_GOTOOLCHAIN" go install "${TASK_MODULE}@${TASK_VERSION}"
GOTOOLCHAIN="$TOOLS_GOTOOLCHAIN" go install "${CUE_MODULE}@${CUE_VERSION}"
GOTOOLCHAIN="$TOOLS_GOTOOLCHAIN" go install "${LEFTHOOK_MODULE}@${LEFTHOOK_VERSION}"
lefthook install
task merge-drivers
mkdir -p workspace/artifacts workspace/logs workspace/tmp
CGO_ENABLED=0 go build -o workspace/artifacts/fathom ./cmd/fathom
workspace/artifacts/fathom version
```

Place the built binary on your session PATH. Use `docs/guides/agent-waves.md`, Isolation and
shared-daemon sections, for per-worker state and scratch verification. Keep repository artifacts
under `workspace/`. Never reuse the owner's daemon store. Run `task test:affected` and the lane's
proof. Race testing on Linux requires a C compiler with `CGO_ENABLED=1`
(`Taskfile.yml:11-12`, `docs/early-testers.md:94`). Do not weaken hooks for WSL2.
Use a foreground daemon if the lane needs one. The managed launchd daemon is macOS only
(`docs/early-testers.md:15-18,135-160`). Do not run the macOS deployment procedure in WSL2.

## What is known and unknown

| Surface | Evidence | Limit |
|---|---|---|
| Linux CI | `.github/workflows/ci.yml:62-73` | Linux runners do not prove WSL2 |
| Linux build and race configuration | `Taskfile.yml:6-12` | Configuration is not a local passing run |
| Non-Unix language cache fallback | `pkg/lang/cache_other.go:1-9` | Source presence does not prove native Windows build |
| Native Windows library cross-build | Not run in this spec round | Build and runtime result unknown |
| Native Windows CLI, hooks, daemon and gate | No witness supplied in the brief | Unknown, not offered as the lane route |
| WSL2 full gate, filesystem behavior and oracle availability | CTR-8 pending | Unknown until a reading on the exact commit |

CTR-8 runs the Linux lane proof and full required gate on WSL2 and records tool versions,
commit, filesystem placement and failures. It may additionally cross-build the library with
`GOOS=windows CGO_ENABLED=0 go build ./pkg/lang/...`. A cross-build cannot establish runtime
or hook support. A failing environment-specific test is evidence to return, never permission
to bypass a gate. See CT-R3 in the lane guide.

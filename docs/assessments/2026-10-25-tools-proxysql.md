# ProxySQL: matching native instrument and literal-key mismatch

The original runtime-edition ceiling is withdrawn. The reviewed branch was
wrong to call all 17 extras valid, audited the wrong release, missed a matching
debug package and did not check the PROXYSQL40 build route.

The unchanged native recipe and original pinned corpus now measure 357/373
definitions: precision .9544 → .9571, recall 1, L1 good; references remain
10/10, L2 excellent. The extra debug definition is attested. Eighteen of nineteen
files load; the same bare-word libconfig file is refused. All required kinds
and every mismatch stay in scope. No product extractor changed.

| Candidate / route | Version / source identity | Concrete result |
|---|---|---|
| Regular Ubuntu24 amd64 deb | 3.0.9-618-g7ddb3dc | Run offline: 356 definitions, 10 references, reproduces the baseline. |
| Matching debug deb | 3.0.9-618-g7ddb3dc_DEBUG | Run offline: 357 definitions, 10 references; adds `admin_variables.debug`. |
| Same-pin innovative debug release | v3.1.9 at 7ddb3dc | Release and published asset digest checked; not run. Its PROXYSQL31 tier lacks the PROXYSQL40 plugin chassis, so it cannot settle the MCP question. Matching 3.0.9 debug already attests debug. |
| Plugin-chassis debug deb | 4.0.10-426-gf5f1e14_DEBUG | Nonmatching source disclosed. Initial run loads 15/19: one emulation jemalloc assertion, two missing absolute plugin paths, plus the known invalid config. Partial 301-definition output is not graded. Superseded by the matching source build. |
| Earlier plugin-chassis release | v4.0.8 at 6ef036a | Published asset identities checked, not run: source differs from the corpus pin, so cannot provide source-matched truth. |
| Matching PROXYSQL40 debug source build | 7ddb3dc, native Linux arm64; upstream pinned packaging image | **Succeeds**, core plus GenAI/MySQLX plugins, tracked source clean. Whole-corpus run with actual plugin paths and networking disabled: 357 definitions, 10 references. No build-availability ceiling remains. |
| Native plugin variable registry | Same built binary/plugin, `SAVE MCP/GENAI VARIABLES FROM RUNTIME` then SELECT of names only | Run offline on an authored probe: 14 MCP names and 32 GenAI names. No calibration subject is edited; registry output is not added to grading truth. |

The two GenAI literals `var1` and `var2` are placeholders absent from both
source and the measured native registry: genuine false positives under the
current known-variable contract. The 14 MCP literals require controller Q5:
the sample uses `mcp_enabled`, `mcp_port`, etc., but the native registry reports
`mcp-enabled`, `mcp-port`, etc. The unchanged oracle qualifies literal keys as
`mcp-mcp_enabled`, `mcp-mcp_port`, etc.; none exists. Dropping the extra `mcp_`
matches all 14, but no such normalization is applied to grading. The source
setter accepts `enabled`, while the generic configuration loader strips an
already present `mcp-` prefix only, not `mcp_`. Also, the sample does not load
the plugin, so merely building the feature does not change its loaded profile.

The committed native registry and comparison JSON, source/build hashes and
runnable recipes are under `calibration/oracles/proxysql/`. Q5 requests a
ruling on these actual literal names before a final ceiling is claimed. The
recommendation is to preserve the current native contract and count the 16
misses honestly, rather than invent aliases or drop declarations. No trigger
requires an impossible all-variables match or a macOS binary.

Timing retains three identity-checked product runs: 26.24, 26.74 and 25.85
MB/CPU-s, median 26.24, darwin/arm64 Apple M4, nice 10, one heavy job, start
load at most 6. The 31,981-byte scope is small; no dialect-wide performance
improvement is claimed. Oracle execution itself is Linux/arm64 and untimed.
Decision models: none; native membership, exact names and counts are facts.

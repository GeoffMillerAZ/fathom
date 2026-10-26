# Click pager snapshot and offline retest

Measured on 2026-10-26 (UTC). The current setup replaces the live Debian mirror
with [this timestamped snapshot](https://snapshot.debian.org/archive/debian/20261025T000000Z/pool/main/l/less/less_590-2.1~deb12u2_arm64.deb). Archive SHA-256:
`eb430d92921f98b163031ee3ac81a96110d1f20bd84f67faab06dad50c75d744`. A host fetch and the setup's `sha256sum -c` both verify that digest;
it equals the historical payload, so only acquisition changed.

The recipe in `click-snapshot-r2.json` downloads that URL with `curl --fail`,
verifies the hash before extraction, and extracts with `tar --no-same-owner`
into the job-owned tool directory. It then runs the same frozen uv test group
as B2; tests add the extracted tool directory to PATH. No live apt mirror or
apt index is consulted. The manifest contains this complete recipe under the section-5 mount contract.

One network-denied warm-up precedes three fresh network-denied timed processes:
4.781, 3.944, 3.424 seconds, exits
0,0,0. The JSON binds commands,
source and uv-lock pins, image, individual times and log hashes. The pinned
Python image, four CPUs and test selection match the B2 comparison. C4 remains
unrun; native macOS and twelve cores remain unmeasured.

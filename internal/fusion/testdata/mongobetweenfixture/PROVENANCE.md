# mongobetween fixture (LB-F100)

Written for fathom from the usage section of coinbase/mongobetween's README
(Apache-2.0, commit ea5a87ec253f): `mongobetween [OPTIONS] address1=uri1
[address2=uri2] ...`, its TCP and unix-socket examples, the flags that take a
value (`-loglevel`, `-network`, `-password`, `-statsd`, `-username`,
`-dynamic`) and the boolean ones (`-ping`, `-pretty`, `-unlink`,
`-enable-sdam-metrics`, `-enable-sdam-logging`). No file is copied from the
project; hosts are org-agnostic (`example.com`, `acme`), ADR-011.

The shape of a real deployment: `gh search code mongobetween` (2026-10-01)
found no plain manifest that launches it, only Helm charts
(MyCarrier-DevOps/helm-charts `charts/mongobetween`, Apache-2.0; a values file
in stafftastic/jitsu-chart, MIT) whose arguments are templates and whose URI
comes from a Secret through `$(MONGODB_URI)` — the `variable` case below.

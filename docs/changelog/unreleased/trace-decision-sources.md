### Breaking

- Trace exports accept repeatable `--decision-source` paths in place of legacy decision inputs. Explicit sources are pinned in `readings.json` and read at the pack's commit; verify requires an updated binary. Exports without the flag preserve legacy sources and pack bytes. See the [migration note](../../upgrades/unreleased-trace-decision-sources.md).

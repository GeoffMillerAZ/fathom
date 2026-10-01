### `config propose` — the diff-offering verb (CC-9)

```sh
fathom config propose [--repo .] [--json | --patch]
```

Runs the same audit and, for each violated concern, offers a unified diff
where the mend is mechanical — v1: a dead pointer or dead script path whose
basename names exactly one file in the repo (it moved; a markdown link is
rewritten file-relative, a backtick path root-relative) — and names the
rest: no file of that name ("gone, not moved"), several ("ambiguous"), a
renamed task target, and every other kind's remedy (prose, budgets, and
fathom's own stanza region, which only `fathom init` / `fleet reconcile`
write). A dead pointer or script on a line inside fathom's markers — the
stanza, or the `INTENT.md` playbook `fathom intent render` generates — is
never offered a diff, however mechanical: it is listed with the
regenerating remedy (`fathom fleet reconcile --repo R --apply`, or fix the
source under `docs/intent/` and re-render). Relocation candidates are the
working tree on git's authority, so an unstaged `mv` is a move, and a
non-ASCII name matches. **It writes nothing** — no repo file, no beat. `--patch` prints only
the combined diff (`fathom config propose --patch | git apply` from the
repo root); `--json` is `{repo_root, violated, proposals, patch}`. Exit `0`
whenever it ran — `config audit` is the gate — and `1` only when a
concerns file would not load.

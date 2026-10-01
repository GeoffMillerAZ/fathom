### Hook fan-out: what a hook does per tool call

Hooks fire on a large share of every agent's tool calls, so each one does the least it
can (RT-130..RT-133, RT-169, design §11.3):

- `gitToplevel` is memoized in process and cached per working directory across hook
  processes (`$TMPDIR/fathom-toplevel-cache`, 10 min, keyed by the cwd and `<top>/.git`'s
  mtime; a `git init` under a cached "not a repository" misses) — no git fork per hook.
- The Read|Grep|Glob observer and a post-Bash call with no write targets are observe-only:
  they exit before any git fork or daemon request (the observer finds its repository root
  by stat). `fathom init` registers the observer `"async": true` on Claude Code, so a Read
  never waits on it.
- The Stop hook folds only what the transcript gained since the session's last Stop (a
  per-session offset stamp beside the session stamps), not up to 16 MB every turn.
- The Stop hook reads every plan's HEAD text through one `git cat-file --batch` (not a
  `git show` per plan file) and asks the daemon nothing until a plan gained a claim, so a
  Stop that claims nothing costs one git process and no RPC however many plans exist; it
  skips its retro once its deadline is spent (O140).
- Identity lookups ask the daemon's `GET /identity` through the cached resolver instead of
  listing every service, and `SubagentStart` reuses its parent's SessionStart resolution
  from the session stamp — zero RPCs.

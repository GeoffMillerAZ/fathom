<!-- fathom-observations:v1 -->
# Fathom observations

A project-local friction log, fed by `fathom observe` — the flywheel's
intake (agents are the primary users, so agent-filed observations are the
flywheel's fuel). Entries graduate into fixes (and a CHANGELOG entry) or are
closed with a recorded reason; `fathom instrument --adoption` reports how
many are still open.

Entry shape: **symptom → evidence → root cause → fix → status → impact**.
One entry per file: `fathom observe` creates `O<n>.md` here with an
exclusive create and never touches an existing entry — correct an entry by
hand (root cause / fix / status) once it's understood, never by re-running
the command over it. A legacy `.fathom/observations.md` log, when a repo
has one, is still read alongside this directory and is never written.

Entry ids (O1, O2, …) are minted over the legacy log and this directory
together; an id already taken is skipped, so two sessions filing at once
never collide and no entry is lost.

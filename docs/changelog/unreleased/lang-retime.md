### Added

- `indexbench lang-retime` re-times a committed language reading on the current machine (design 1.9 T2): it refuses unless each subject sits at its recorded commit with a clean tree and the extraction reproduces the reading's fathom-side counts, then keeps every oracle measurement, replaces only the timing with the median of at least three runs, and names every run's pooled CPU throughput and the machine class in the reading's notes. `-tenancy-only` limits the identity check to the timing run's own facts for a reading whose counts were scored on another scope, and the note says so.

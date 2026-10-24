### Fixed

- Intent audit refuses condition annotations that point outside the repository or at another repository's service instead of evaluating them. Refused annotations print as UNEVALUABLE lines, count in the unevaluable buckets, and an unreadable service registry leaves service annotations unevaluable with its own wording while path confinement still runs.

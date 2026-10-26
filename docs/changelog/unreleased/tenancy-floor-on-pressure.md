### Changed

- The tenancy ladder's swap-volume floor now refuses work only while host memory pressure is at `warn` or above. `[tenancy.host] floor_on_pressure` sets the level (`normal` restores the old behavior, refusing at any pressure), and `fathom resources` shows it under host triggers.

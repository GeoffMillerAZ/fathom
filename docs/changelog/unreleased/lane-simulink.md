### Added

- Simulink models (`.slx`, `.mdl`) extraction at L3 (good), measured against an independent zipfile and expat walk (tier C) on WEC-Sim, Thermodynamics, LADAC, FMT-Model and etherlab-examples: every block by its full path, signal lines as references, model references to the model of that name. A `.slx` is read in memory under entry, inflated-byte and ratio caps, the one NUL-sniffed format a dialect may claim (ADR-021, amended); a text-package `.mdl` is disclosed, not read.

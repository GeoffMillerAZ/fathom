### Added

- Simulink models (`.slx`, `.mdl`) extract at L2 (good): blocks by full path, signal lines as references, model references to the model of that name (L3 deferred). A `.slx` is read in memory under the caps of ADR-021 (amended); a model the reader refuses is counted on the ingest report (`models-refused`, JSON `models_refused`) with one stderr line naming the file.

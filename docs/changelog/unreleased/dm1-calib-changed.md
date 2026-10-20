### Changed

- A decision-port site other than `certify suggest` serves a provider's answer only when its latest calibration reading for that provider, model and template clears the site's bar and the provider beat the deterministic answer on the same set. Otherwise the provider is not asked, the site serves its deterministic answer, and one `decide: not calibrated` line says why; a model move makes the site uncalibrated until it is graded again. Provider `none` is unchanged.

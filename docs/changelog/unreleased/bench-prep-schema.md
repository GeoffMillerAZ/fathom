### Added

- Benchmark subject and preregistration CUE contracts (`schema/bench`), checked by `task cue-vet`. A subject manifest keeps its corpus metadata and pins its toolchain (an exact prerelease is accepted) and source provenance; the model cutoff date may stay empty until the model is pinned. A preregistration declares a baseline, predictions for every comparison arm and every stratum, fixed primary criteria, Holm correction, the three-outcome rule, and the predicted-null equivalence, harm, inconclusive and unexpected-effect rules.

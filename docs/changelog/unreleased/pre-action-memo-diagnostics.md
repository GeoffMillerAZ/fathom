### Changed

- On entry to shed or refuse, the daemon logs a bounded pre-action snapshot of sampled footprint and each memo group's estimated resident bytes, entries, and cumulative activity counters before cache eviction and scavenging. The record excludes cache contents and does not attribute the footprint to a particular allocator.

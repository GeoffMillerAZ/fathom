### Added

- The prometheus dialect reads an alerting rule's `for`, `keep_firing_for` and annotations (`annotations.<name>`) as facts, and the alertmanager dialect a route's `group_wait`, `group_interval` and `repeat_interval`, each at its key's line with the value as written. Ingest stores them as `EntityFact` rows and `symbol_context` serves them under `facts`.

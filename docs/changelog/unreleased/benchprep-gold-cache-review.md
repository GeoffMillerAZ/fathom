### Fixed

- Gold-patch fetching refuses tracked or nonignored cache paths inside git work trees, including nonexistent descendants, directory symlinks, and cache hits, while allowing ignored caches such as `workspace/scenariobench/gold`.

### Fixed
- The CUE typed pass no longer builds the default module registry, so ingesting a CUE module with external dependencies cannot contact the public registry. Such imports now report as failed packages naming the dependency.

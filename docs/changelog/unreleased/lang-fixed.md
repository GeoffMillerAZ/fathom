### Fixed

- Ingest stores the file-resolved edges of a file a dialect claims by basename, not only by extension: a `BUILD`, `WORKSPACE` or `MODULE.bazel` file's `load()`s, a `meson.build`'s `subdir()`s and a `CMakeLists.txt`'s `add_subdirectory()`s, which the file-resolve pass skipped before (O379).

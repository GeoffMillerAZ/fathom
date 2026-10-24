### Added

- Conan recipes reach L3: a `requires`, `tool_requires`, `build_requires` or `test_requires` reference (class attribute or `self.requires(...)`) in a `conanfile.py` now draws a file-resolved edge to the one recipe of the service that declares that name. The reference's version, user/channel and revision are cut; a name two recipes declare binds nothing.

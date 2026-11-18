### Added

- The dependency reader detects TypeScript and JavaScript imports and re-exports, including CommonJS and dynamic imports. It resolves supported relative paths, configuration aliases and package exports, and reports unknowns for computed requests and unsupported resolution settings. JavaScript can report complete files. TypeScript reports dependency evidence with completeness disabled pending qualifying compiler coverage.

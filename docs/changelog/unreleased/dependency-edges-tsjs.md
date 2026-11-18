### Added

- The dependency reader detects TypeScript and JavaScript imports and re-exports, including CommonJS and dynamic imports. It resolves supported relative paths, configuration aliases and package exports, and reports unknowns for computed requests and unsupported resolution settings. JavaScript and TypeScript can report complete files when all dependency constructs are detected and resolved.

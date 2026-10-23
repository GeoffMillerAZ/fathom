### Changed

- Typed Go indexing keeps only your own packages' syntax and type details in memory, roughly halving peak memory (a 20-package module went from about 800 MiB to 270 MiB and now completes under a 512 MiB budget). Standard-library and module-cache dependencies are type-checked without function bodies. A type error inside such a dependency's function body no longer fails your packages. Declaration errors and errors in first-party or replaced dependencies still do. The typed pass also ignores an inherited `GOPACKAGESDRIVER`, so an external package driver cannot change it.

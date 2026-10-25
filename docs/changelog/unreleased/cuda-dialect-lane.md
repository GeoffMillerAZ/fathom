### Added

- CUDA (`.cu`, `.cuh`) files are indexed as a language of their own through a tree-sitter grammar: kernels, device and host functions, classes, structs, enums, namespaces, macros, fields and `k<<<g, b>>>(args)` launches become symbols and call edges, and `#include` edges resolve between `.cu` and `.cuh` files. A `.h` header a CUDA file includes is still read as C or C++.

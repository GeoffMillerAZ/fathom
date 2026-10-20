### Changed

- SCSS records mixin, function and variable references across its module graph: a `$variable` read, a namespaced `ns.m`, `ns.f()` or `ns.$v` member and an `@extend %placeholder` join the `@include` and function calls it recorded, each owned by the enclosing `@mixin` or `@function` or by the file, including inside a top-level variable's value or a selector's interpolation.

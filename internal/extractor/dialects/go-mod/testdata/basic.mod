// The module under test: every directive the dialect reads.
module example.com/acme/app

go 1.21

toolchain go1.21.4

require (
	github.com/pkg/errors v0.9.1
	golang.org/x/sync v0.5.0 // indirect
	example.com/acme/lib v0.0.0
)

require github.com/stretchr/testify v1.8.4

replace example.com/acme/lib => ../lib

replace (
	github.com/pkg/errors v0.9.1 => github.com/acme/errors v0.9.2
	golang.org/x/sync => ./third_party/sync
	example.com/acme/climb => ../../../elsewhere
)

exclude golang.org/x/net v0.1.0

retract v1.0.0 // published by mistake

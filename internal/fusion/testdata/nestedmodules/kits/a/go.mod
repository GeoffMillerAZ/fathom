module example.com/nestedroot/kits/a

go 1.22

require (
	example.com/cli v0.0.0
	example.com/nestedroot/shared v0.0.0
)

// An in-repo module outside this one.
replace example.com/nestedroot/shared => ../../shared

// A vendored-style command framework kept inside the module: its module
// path is not the repository's, so the typed pass does not load it.
replace example.com/cli => ./third_party/cli

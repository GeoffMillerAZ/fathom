module example.com/nestedroot/kits/a

go 1.22

require (
	example.com/cli v0.0.0
	example.com/shared v0.0.0
)

// An in-repo module outside this one.
replace example.com/shared => ../../shared

// A vendored-style command framework kept inside the module.
replace example.com/cli => ./third_party/cli

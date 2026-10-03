// Note that this file must have no imports and all values must be concrete.

@if(!debug)

package main

// Defaults
values: {
	message:  "Hello World"
	replicas: 2
	image: repository: "registry.example.test/demo"
}

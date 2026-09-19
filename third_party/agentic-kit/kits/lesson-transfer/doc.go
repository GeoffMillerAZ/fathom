// Package lessontransfer validates bounded public-synthetic lesson bundles and
// runs receiver-owned adoption checks. It never executes bundle commands, writes
// a receiver, authenticates attestations, or provides a shared lesson store.
//
// Bundle values are copied before crossing every callback boundary. CheckAdoption
// defaults its receipt clock to time.Now; CheckAdoptionAt accepts an injected clock
// for deterministic callers and tests. Concurrent calls are safe when the supplied
// callbacks and clock are safe.
package lessontransfer

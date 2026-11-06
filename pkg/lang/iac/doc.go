// Package iac reads what an infrastructure-as-code file declares: each resource, data source or module call, and
// each attribute it sets, with a path, a value class and a span. It is part of fathom's language library (ADR-092,
// docs/design/lang-library.md section 5) and, like pkg/lang, needs no store, daemon, index, git or network.
//
// A read never runs init, a plan, a provider or a program, and never expands a module. It reads Terraform and
// OpenTofu (.tf, .tofu, and a directory as one module), CloudFormation and SAM templates (YAML or JSON) and Pulumi
// YAML programs.
//
// # Classes, not values
//
// Every attribute reads as one of five classes: literal (written as a constant), defaulted (resolves only through
// a variable default or a local chain ending in one), reference (names another resource, data source, module
// output or parameter), expression (computed, with no static answer) or unknown (the reader could not classify
// it). Absence is not a class: "this attribute is unset" comes from comparing a read with a contract.
//
// Attr.Value is empty unless the caller passes WithValues. Even then the reader withholds what its source marks
// secret (a sensitive or ephemeral Terraform variable's default, a CloudFormation NoEcho parameter, a Pulumi
// secret and fn::secret), and the caller still learns that a literal or default is there.
//
// # The read guarantee
//
// ReadFile and ReadModule return by a deadline, DefaultReadDeadline, which WithDeadline may lower and nothing
// raises. Go cannot interrupt a goroutine, so a read that overruns is abandoned, not stopped: it keeps its CPU
// until it returns on its own and its result is discarded. Once GOMAXPROCS abandoned reads are still running, a
// read starts nothing and returns ErrBusy. A caller reading input it does not trust reads through a process it can
// kill: `fathom-lang resources` does, by default.
//
// Output is deterministic. Every slice is non-nil and sorted, and equal inputs give byte-equal JSON.
package iac

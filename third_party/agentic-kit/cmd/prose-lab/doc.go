// Command prose-lab runs explicitly selected synthetic prose experiments.
// It is an opt-in development utility, not a publication authority or a service.
// Concurrency: one process owns a journal lock for the entire run; a journal left
// locked by a crash requires operator inspection, never automatic replay.
package main

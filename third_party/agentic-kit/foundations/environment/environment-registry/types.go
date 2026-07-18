// Package environmentregistry manages a named catalogue of runtime environments.
// It provides a thread-safe in-process registry that callers can populate at
// start-up and query at runtime — no third-party dependencies.
//
// CUE is authoritative for the shape of Environment; Go is authoritative for
// the Registry behaviour.
//
// Concurrency contract: Registry is safe for concurrent use; all methods are
// guarded by a sync.RWMutex.
package environmentregistry

import (
	"errors"
	"fmt"
	"strings"
	"sync"
)

// Kind is the tier classification for a runtime environment.
// The valid values mirror #Kind in schema.cue.
type Kind string

const (
	// KindDev is a local or ephemeral developer environment.
	KindDev Kind = "dev"

	// KindStaging is a pre-production verification environment.
	KindStaging Kind = "staging"

	// KindProd is a live production environment.
	KindProd Kind = "prod"
)

var validKinds = map[Kind]struct{}{
	KindDev:     {},
	KindStaging: {},
	KindProd:    {},
}

// Environment describes a named runtime context. It mirrors #Environment in
// schema.cue.
type Environment struct {
	// Name is the unique, human-readable identifier for the environment.
	// Must be non-empty and contain no whitespace.
	Name string `json:"Name"`

	// Kind classifies the environment tier. Must be one of dev / staging / prod.
	Kind Kind `json:"Kind"`
}

// Validate enforces the same constraints that #Environment encodes in
// schema.cue. It returns a non-nil error when any constraint is violated.
func (e Environment) Validate() error {
	var errs []string

	if strings.TrimSpace(e.Name) == "" {
		errs = append(errs, "Name must be non-empty")
	} else if e.Name != strings.TrimSpace(e.Name) {
		errs = append(errs, "Name must not have leading or trailing whitespace")
	}
	if _, ok := validKinds[e.Kind]; !ok {
		errs = append(errs, fmt.Sprintf(
			"Kind %q is not valid (must be one of: dev, staging, prod)",
			e.Kind,
		))
	}

	if len(errs) > 0 {
		return errors.New("Environment: " + strings.Join(errs, "; "))
	}
	return nil
}

// Registry is a thread-safe, in-memory store for Environment records.
type Registry struct {
	mu   sync.RWMutex
	envs map[string]Environment
}

// NewRegistry returns an empty, ready-to-use Registry.
func NewRegistry() *Registry {
	return &Registry{envs: make(map[string]Environment)}
}

// Register adds or replaces an Environment in the registry.
// It returns a non-nil error if the environment fails validation.
func (r *Registry) Register(env Environment) error {
	if err := env.Validate(); err != nil {
		return fmt.Errorf("Register: %w", err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.envs[env.Name] = env
	return nil
}

// Get returns the Environment associated with name, along with a boolean that
// is true when the name was found.
func (r *Registry) Get(name string) (Environment, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	env, ok := r.envs[name]
	return env, ok
}

// List returns all registered Environments in an unspecified order.
func (r *Registry) List() []Environment {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Environment, 0, len(r.envs))
	for _, env := range r.envs {
		out = append(out, env)
	}
	return out
}

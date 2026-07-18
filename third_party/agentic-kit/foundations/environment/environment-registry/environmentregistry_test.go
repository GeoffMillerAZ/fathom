package environmentregistry_test

import (
	"strings"
	"testing"

	environmentregistry "github.com/geoffmilleraz/agentic-kit/foundations/environment/environment-registry"
)

// ── Environment.Validate ──────────────────────────────────────────────────────

func TestEnvironment_Validate_valid(t *testing.T) {
	cases := []struct {
		name string
		env  environmentregistry.Environment
	}{
		{"dev", environmentregistry.Environment{Name: "local-dev", Kind: environmentregistry.KindDev}},
		{"staging", environmentregistry.Environment{Name: "ci-staging", Kind: environmentregistry.KindStaging}},
		{"prod", environmentregistry.Environment{Name: "production", Kind: environmentregistry.KindProd}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.env.Validate(); err != nil {
				t.Fatalf("expected nil error, got: %v", err)
			}
		})
	}
}

func TestEnvironment_Validate_invalid(t *testing.T) {
	cases := []struct {
		name    string
		env     environmentregistry.Environment
		wantErr string
	}{
		{
			name:    "empty Name",
			env:     environmentregistry.Environment{Name: "", Kind: environmentregistry.KindDev},
			wantErr: "Name must be non-empty",
		},
		{
			name:    "whitespace Name",
			env:     environmentregistry.Environment{Name: "   ", Kind: environmentregistry.KindDev},
			wantErr: "Name must be non-empty",
		},
		{
			name:    "unknown Kind",
			env:     environmentregistry.Environment{Name: "env-x", Kind: "unknown"},
			wantErr: "not valid",
		},
		{
			name:    "empty Kind",
			env:     environmentregistry.Environment{Name: "env-x", Kind: ""},
			wantErr: "not valid",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.env.Validate()
			if err == nil {
				t.Fatal("expected a validation error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

// ── Registry ──────────────────────────────────────────────────────────────────

func TestRegistry_Register_and_Get(t *testing.T) {
	r := environmentregistry.NewRegistry()

	dev := environmentregistry.Environment{Name: "local-dev", Kind: environmentregistry.KindDev}
	if err := r.Register(dev); err != nil {
		t.Fatalf("Register: unexpected error: %v", err)
	}

	got, ok := r.Get("local-dev")
	if !ok {
		t.Fatal("Get: expected to find registered environment")
	}
	if got != dev {
		t.Errorf("Get: got %+v, want %+v", got, dev)
	}
}

func TestRegistry_Get_missing(t *testing.T) {
	r := environmentregistry.NewRegistry()
	_, ok := r.Get("nonexistent")
	if ok {
		t.Fatal("Get: expected false for unregistered name")
	}
}

func TestRegistry_Register_invalid(t *testing.T) {
	r := environmentregistry.NewRegistry()
	bad := environmentregistry.Environment{Name: "", Kind: environmentregistry.KindDev}
	if err := r.Register(bad); err == nil {
		t.Fatal("Register: expected error for invalid environment")
	}
}

func TestRegistry_Register_replace(t *testing.T) {
	r := environmentregistry.NewRegistry()
	first := environmentregistry.Environment{Name: "env-a", Kind: environmentregistry.KindDev}
	second := environmentregistry.Environment{Name: "env-a", Kind: environmentregistry.KindProd}

	if err := r.Register(first); err != nil {
		t.Fatalf("Register first: %v", err)
	}
	if err := r.Register(second); err != nil {
		t.Fatalf("Register second: %v", err)
	}

	got, _ := r.Get("env-a")
	if got.Kind != environmentregistry.KindProd {
		t.Errorf("expected Kind to be updated to prod, got %q", got.Kind)
	}
}

func TestRegistry_List(t *testing.T) {
	r := environmentregistry.NewRegistry()
	envs := []environmentregistry.Environment{
		{Name: "env-1", Kind: environmentregistry.KindDev},
		{Name: "env-2", Kind: environmentregistry.KindStaging},
		{Name: "env-3", Kind: environmentregistry.KindProd},
	}
	for _, e := range envs {
		if err := r.Register(e); err != nil {
			t.Fatalf("Register: %v", err)
		}
	}

	list := r.List()
	if len(list) != len(envs) {
		t.Errorf("List: got %d items, want %d", len(list), len(envs))
	}
}

func TestRegistry_List_empty(t *testing.T) {
	r := environmentregistry.NewRegistry()
	list := r.List()
	if len(list) != 0 {
		t.Errorf("List on empty registry: got %d items, want 0", len(list))
	}
}

func TestRegistry_Concurrent(t *testing.T) {
	r := environmentregistry.NewRegistry()
	done := make(chan struct{})

	// One writer and one reader running concurrently to exercise the mutex.
	go func() {
		for i := 0; i < 100; i++ {
			_ = r.Register(environmentregistry.Environment{
				Name: "concurrent-env",
				Kind: environmentregistry.KindDev,
			})
		}
		close(done)
	}()
	for i := 0; i < 100; i++ {
		r.Get("concurrent-env")
	}
	<-done
}

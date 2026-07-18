package awscred

import "testing"

func TestResolveRegion_Precedence(t *testing.T) {
	t.Run("ExplicitWins", func(t *testing.T) {
		t.Setenv(envRegion, "env-region")
		t.Setenv(envDefaultRegion, "default-env-region")
		if got := resolveRegion(Config{Region: "explicit-region"}, "shared-region"); got != "explicit-region" {
			t.Errorf("got %q, want explicit-region", got)
		}
	})
	t.Run("EnvRegionSecond", func(t *testing.T) {
		t.Setenv(envRegion, "env-region")
		t.Setenv(envDefaultRegion, "default-env-region")
		if got := resolveRegion(Config{}, "shared-region"); got != "env-region" {
			t.Errorf("got %q, want env-region", got)
		}
	})
	t.Run("DefaultRegionThird", func(t *testing.T) {
		t.Setenv(envRegion, "")
		t.Setenv(envDefaultRegion, "default-env-region")
		if got := resolveRegion(Config{}, "shared-region"); got != "default-env-region" {
			t.Errorf("got %q, want default-env-region", got)
		}
	})
	t.Run("SharedProfileRegionLast", func(t *testing.T) {
		t.Setenv(envRegion, "")
		t.Setenv(envDefaultRegion, "")
		if got := resolveRegion(Config{}, "shared-region"); got != "shared-region" {
			t.Errorf("got %q, want shared-region", got)
		}
	})
	t.Run("EmptyWhenNothingResolves", func(t *testing.T) {
		t.Setenv(envRegion, "")
		t.Setenv(envDefaultRegion, "")
		if got := resolveRegion(Config{}, ""); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
}

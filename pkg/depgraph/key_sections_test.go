package depgraph

import "testing"

// Equal scalar streams must not make distinct action inputs interchangeable.
// These are serialization collisions, not SHA-256 collisions.
func TestInputKeySeparatesSections(t *testing.T) {
	base := KeyInputs{Domain: "review/action", Epoch: 3, Task: "test", Platform: "linux/amd64"}
	withEmptyArg := base
	withEmptyArg.Args = []string{""}
	if InputKey(base) == InputKey(withEmptyArg) {
		t.Fatal("an empty positional argument must change the key")
	}
	cases := []struct {
		name string
		a, b KeyInputs
	}{
		{"arguments and secret names", func() KeyInputs { n := base; n.Args = []string{"token"}; return n }(), func() KeyInputs { n := base; n.SecretNames = []string{"token"}; return n }()},
		{"toolchain packages and environment", func() KeyInputs { n := base; n.ToolchainPackages = map[string]string{"MODE": "strict"}; return n }(), func() KeyInputs { n := base; n.Env = map[string]string{"MODE": "strict"}; return n }()},
		{"environment and arguments", func() KeyInputs { n := base; n.Env = map[string]string{"MODE": "strict"}; return n }(), func() KeyInputs { n := base; n.Args = []string{"MODE", "strict"}; return n }()},
		{"dependencies and toolchain scalars", func() KeyInputs {
			n := base
			n.Deps = []string{"a"}
			n.ToolchainLockDigest = "b"
			n.CueLang = "c"
			return n
		}(), func() KeyInputs {
			n := base
			n.ToolchainLockDigest = "a"
			n.CueLang = "b"
			n.Args = []string{"c"}
			return n
		}()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if InputKey(tc.a) == InputKey(tc.b) {
				t.Fatal("distinct input sections produced the same key")
			}
		})
	}
}

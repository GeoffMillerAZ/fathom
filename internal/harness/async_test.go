package harness

import "testing"

// RT-169 (design §11.3): observe-only hooks are registered non-blocking
// (async) where the harness supports it, so the agent's tool call never
// waits on them; a binding that injects or blocks, or a harness without
// async hooks, is never registered async.
func TestObserveOnlyHooksRegisteredNonBlocking(t *testing.T) {
	sawAsync := false
	for _, d := range All() {
		for _, b := range d.Hooks {
			got := hookRegistrationAsync(d, b)
			if got != d.RegisterAsync(b) {
				t.Fatalf("%s %s: RegisterAsync disagrees with hookRegistrationAsync", d.ID, b.Harness)
			}
			if got && b.Vehicle != VehicleHookSilent {
				t.Errorf("%s %s (%s): a %s binding registered async — its output would never be read", d.ID, b.Harness, b.Matcher, b.Vehicle)
			}
			if got && !d.AsyncHooks {
				t.Errorf("%s: async on a harness without async hooks", d.ID)
			}
			if b.Capability == CapDiscoveryObserve && d.AsyncHooks && !got {
				t.Errorf("%s: the Read|Grep|Glob observer is observe-only and must be registered async", d.ID)
			}
			if got {
				sawAsync = true
			}
		}
	}
	if !sawAsync {
		t.Fatal("no binding is registered async on any harness")
	}
	// The rule itself, independent of today's registry.
	d := Descriptor{AsyncHooks: true}
	if !hookRegistrationAsync(d, EventBinding{ObserveOnly: true, Vehicle: VehicleHookSilent}) {
		t.Fatal("observe-only + silent + async harness → async")
	}
	if hookRegistrationAsync(d, EventBinding{ObserveOnly: true, Vehicle: VehicleHookInject}) {
		t.Fatal("an injecting binding is never async")
	}
	if hookRegistrationAsync(Descriptor{}, EventBinding{ObserveOnly: true, Vehicle: VehicleHookSilent}) {
		t.Fatal("a harness without async hooks never gets async")
	}
}

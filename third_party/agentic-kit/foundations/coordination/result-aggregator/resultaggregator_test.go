package resultaggregator

import (
	"testing"
)

func TestAggregate_Empty(t *testing.T) {
	c := Aggregate(nil)
	if c.Count != 0 {
		t.Fatalf("expected Count=0, got %d", c.Count)
	}
	if len(c.Items) != 0 {
		t.Fatalf("expected empty Items, got %v", c.Items)
	}
}

func TestAggregate_EmptySlice(t *testing.T) {
	c := Aggregate([]Result{})
	if c.Count != 0 {
		t.Fatalf("expected Count=0, got %d", c.Count)
	}
}

func TestAggregate_SingleResult(t *testing.T) {
	parts := []Result{{Key: "k1", Value: "v1"}}
	c := Aggregate(parts)
	if c.Count != 1 {
		t.Fatalf("expected Count=1, got %d", c.Count)
	}
	if c.Items[0].Key != "k1" || c.Items[0].Value != "v1" {
		t.Fatalf("unexpected item: %+v", c.Items[0])
	}
}

func TestAggregate_SortedByKey(t *testing.T) {
	parts := []Result{
		{Key: "z", Value: "last"},
		{Key: "a", Value: "first"},
		{Key: "m", Value: "middle"},
	}
	c := Aggregate(parts)
	if c.Count != 3 {
		t.Fatalf("expected Count=3, got %d", c.Count)
	}
	expectedKeys := []string{"a", "m", "z"}
	for i, r := range c.Items {
		if r.Key != expectedKeys[i] {
			t.Errorf("position %d: expected key %q, got %q", i, expectedKeys[i], r.Key)
		}
	}
}

func TestAggregate_DedupByKey_LastWins(t *testing.T) {
	parts := []Result{
		{Key: "k1", Value: "first"},
		{Key: "k2", Value: "only"},
		{Key: "k1", Value: "second"},
		{Key: "k1", Value: "last"},
	}
	c := Aggregate(parts)
	if c.Count != 2 {
		t.Fatalf("expected Count=2 after dedup, got %d", c.Count)
	}
	// k1 should have value "last" (final occurrence wins)
	var k1 Result
	for _, r := range c.Items {
		if r.Key == "k1" {
			k1 = r
		}
	}
	if k1.Value != "last" {
		t.Fatalf("expected k1.Value='last', got %q", k1.Value)
	}
}

func TestAggregate_CountMatchesItems(t *testing.T) {
	parts := []Result{
		{Key: "a", Value: "1"},
		{Key: "b", Value: "2"},
		{Key: "a", Value: "3"}, // dup
		{Key: "c", Value: "4"},
	}
	c := Aggregate(parts)
	if c.Count != len(c.Items) {
		t.Fatalf("Count=%d does not match len(Items)=%d", c.Count, len(c.Items))
	}
}

func TestAggregate_Determinism(t *testing.T) {
	// The same input always produces the same output.
	parts := []Result{
		{Key: "z", Value: "z-val"},
		{Key: "a", Value: "a-val"},
		{Key: "m", Value: "m-val"},
		{Key: "a", Value: "a-val-2"},
	}
	first := Aggregate(parts)
	second := Aggregate(parts)

	if first.Count != second.Count {
		t.Fatalf("non-deterministic Count: %d vs %d", first.Count, second.Count)
	}
	for i := range first.Items {
		if first.Items[i].Key != second.Items[i].Key || first.Items[i].Value != second.Items[i].Value {
			t.Errorf("position %d: %+v != %+v", i, first.Items[i], second.Items[i])
		}
	}
}

func TestAggregate_AllSameKey(t *testing.T) {
	// All results share one key — output should have exactly 1 item (last value).
	parts := []Result{
		{Key: "singleton", Value: "v1"},
		{Key: "singleton", Value: "v2"},
		{Key: "singleton", Value: "v3"},
	}
	c := Aggregate(parts)
	if c.Count != 1 {
		t.Fatalf("expected Count=1, got %d", c.Count)
	}
	if c.Items[0].Value != "v3" {
		t.Fatalf("expected last value 'v3', got %q", c.Items[0].Value)
	}
}

func TestAggregate_PreservesEmptyValue(t *testing.T) {
	// An empty Value string is valid; it should be preserved.
	parts := []Result{{Key: "k", Value: ""}}
	c := Aggregate(parts)
	if c.Count != 1 {
		t.Fatalf("expected Count=1, got %d", c.Count)
	}
	if c.Items[0].Value != "" {
		t.Fatalf("expected empty Value to be preserved, got %q", c.Items[0].Value)
	}
}

func TestAggregate_LargeInput(t *testing.T) {
	// Verify correct behavior and determinism with a larger input set.
	const n = 100
	parts := make([]Result, n)
	for i := 0; i < n; i++ {
		// Every other result uses the same key to exercise dedup.
		key := "even"
		if i%2 != 0 {
			key = "odd"
		}
		parts[i] = Result{Key: key, Value: itoa(i)}
	}
	c := Aggregate(parts)
	if c.Count != 2 {
		t.Fatalf("expected 2 unique keys, got %d", c.Count)
	}
	// Last even index is 98, last odd is 99.
	for _, r := range c.Items {
		switch r.Key {
		case "even":
			if r.Value != "98" {
				t.Errorf("even: expected '98', got %q", r.Value)
			}
		case "odd":
			if r.Value != "99" {
				t.Errorf("odd: expected '99', got %q", r.Value)
			}
		}
	}
}

// itoa converts an int to its decimal string representation without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	buf := [20]byte{}
	pos := len(buf)
	for n > 0 {
		pos--
		buf[pos] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}

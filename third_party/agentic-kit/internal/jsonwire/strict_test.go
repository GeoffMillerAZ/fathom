package jsonwire

import "testing"

type strictSample struct {
	Value bool `json:"value"`
	Count int  `json:"count"`
}

func TestDecodeStrictAndObject(t *testing.T) {
	good := []byte(`{"value":false,"count":0}`)
	var s strictSample
	if err := DecodeStrict(good, &s); err != nil {
		t.Fatal(err)
	}
	for _, b := range [][]byte{[]byte(`{"value":false}`), []byte(`{"Value":false,"count":0}`), []byte(`{"value":null,"count":0}`), []byte(`{"value":false,"value":true,"count":0}`), []byte(`{"value":false,"count":9223372036854775808}`), []byte(`{"value":false,"count":0,"x":1}`), []byte(`{"value":false,"count":0} true`), []byte(`{"value":"\ud800","count":0}`)} {
		if DecodeStrict(b, &s) == nil {
			t.Errorf("strict accepted %s", b)
		}
	}
	if _, err := Object([]byte(`{"used":false,"future":null}`)); err != nil {
		t.Fatal(err)
	}
	if _, err := Object([]byte(`{"x":1,"x":2}`)); err == nil {
		t.Error("object accepted duplicate")
	}
}

func TestValidateStrictJSONPolicyPreflight(t *testing.T) {
	good := []byte(`{"spawn":{"require_model":true},"tiers":[1]}`)
	if err := ValidateStrictJSON(good); err != nil {
		t.Fatal(err)
	}
	depth := []byte(`0`)
	for i := 0; i < 25; i++ {
		depth = append([]byte{'['}, depth...)
		depth = append(depth, ']')
	}
	for _, bad := range [][]byte{
		[]byte(`{"spawn":{"require_model":false,"require_model":true}}`),
		[]byte(`{"text":"\ud800"}`),
		[]byte{'{', '"', 'x', '"', ':', '"', 0xff, '"', '}'},
		[]byte(`{"x":null}`),
		[]byte(`{} {}`),
		depth,
	} {
		if ValidateStrictJSON(bad) == nil {
			t.Errorf("accepted invalid policy JSON: %q", bad)
		}
	}
}

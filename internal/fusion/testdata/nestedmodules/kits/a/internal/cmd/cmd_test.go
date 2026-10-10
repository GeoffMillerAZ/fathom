package cmd

import "testing"

func TestRunDoctor(t *testing.T) {
	if err := runDoctor(&options{}, nil); err != nil {
		t.Fatal(err)
	}
}

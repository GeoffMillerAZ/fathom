package command_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/geoffmilleraz/fathom/third_party/app-kit/appkit/cli/command"
)

func TestExplicitCodeDistinguishesExplicitCodesFromDefaults(t *testing.T) {
	target := errors.New("target")
	tests := []struct {
		name     string
		err      error
		want     int
		explicit bool
		wantCode int
	}{
		{name: "nil", wantCode: 0},
		{name: "plain", err: target, wantCode: 1},
		{name: "Exit nil", err: command.Exit(2, nil), wantCode: 0},
		{name: "zero", err: command.Exit(0, target), want: 0, explicit: true, wantCode: 0},
		{name: "one", err: command.Exit(1, target), want: 1, explicit: true, wantCode: 1},
		{name: "two", err: command.Exit(2, target), want: 2, explicit: true, wantCode: 2},
		{name: "positive", err: command.Exit(73, target), want: 73, explicit: true, wantCode: 73},
		{name: "negative", err: command.Exit(-9, target), want: -9, explicit: true, wantCode: -9},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, explicit := command.ExplicitCode(test.err)
			if got != test.want || explicit != test.explicit {
				t.Fatalf("ExplicitCode() = (%d, %v), want (%d, %v)", got, explicit, test.want, test.explicit)
			}
			if got := command.Code(test.err); got != test.wantCode {
				t.Fatalf("Code() = %d, want %d", got, test.wantCode)
			}
			if test.explicit && !errors.Is(test.err, target) {
				t.Fatal("Exit wrapper lost the underlying error identity")
			}
		})
	}
}

func TestExplicitCodePreservesErrorsAsTraversalOrder(t *testing.T) {
	first := errors.New("first")
	second := errors.New("second")
	tests := []struct {
		name string
		err  error
		want int
	}{
		{name: "ordinary wrap", err: fmt.Errorf("context: %w", command.Exit(2, first)), want: 2},
		{name: "reported wrap", err: command.Reported(command.Exit(3, first)), want: 3},
		{name: "wrap around reported", err: fmt.Errorf("context: %w", command.Reported(command.Exit(4, first))), want: 4},
		{name: "outer nested exit", err: command.Exit(5, command.Exit(6, first)), want: 5},
		{name: "joined first selection", err: errors.Join(command.Exit(7, first), command.Exit(8, second)), want: 7},
		{name: "joined reverse selection", err: errors.Join(command.Exit(8, second), command.Exit(7, first)), want: 8},
		{name: "plain then explicit join", err: errors.Join(first, command.Exit(9, second)), want: 9},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, explicit := command.ExplicitCode(test.err)
			if got != test.want || !explicit {
				t.Fatalf("ExplicitCode() = (%d, %v), want (%d, true)", got, explicit, test.want)
			}
			if got := command.Code(test.err); got != test.want {
				t.Fatalf("Code() = %d, want %d", got, test.want)
			}
			if !errors.Is(test.err, first) && !errors.Is(test.err, second) {
				t.Fatal("composed error lost all underlying identities")
			}
		})
	}
}

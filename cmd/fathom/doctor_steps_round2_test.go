package main

import (
	"context"
	"testing"
	"time"
)

func TestDoctorInteractiveLocalCheckHasNoDeadline(t *testing.T) {
	rows := runDoctorStepsContext(context.Background(), []doctorStep{{name: "local", probe: func(ctx context.Context) doctorCheck {
		if _, ok := ctx.Deadline(); ok {
			return doctorCheck{Name: "local", Status: doctorFail, Reason: "interactive local check was given a deadline"}
		}
		return doctorCheck{Name: "local", Status: doctorPass}
	}}})
	if rows[0].Status != doctorPass {
		t.Fatal(rows)
	}
}

func TestDoctorAbandonedCheckKeepsDetail(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Hour))
	defer cancel()
	rows := runDoctorStepsContext(ctx, []doctorStep{{name: "detail", probe: func(ctx context.Context) doctorCheck {
		cancel()
		<-ctx.Done()
		return doctorCheck{Name: "detail", Status: doctorUnknown, Reason: "specific unreadable source", probeErr: ctx.Err()}
	}}})
	if rows[0].Reason == "" || rows[0].Reason == "check did not finish within its remaining deadline" {
		t.Fatal(rows)
	}
}

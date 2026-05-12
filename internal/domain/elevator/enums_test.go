package elevator

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// These tests pin the string values to the API contract (docs/api.md §2 /
// docs/openapi.yaml). Renaming a constant is fine; changing the underlying
// string is a breaking API change and must fail loudly here.

func TestDoorState_StringValuesMatchAPI(t *testing.T) {
	cases := []struct {
		name string
		got  DoorState
		want string
	}{
		{"open", DoorStateOpen, "open"},
		{"opening", DoorStateOpening, "opening"},
		{"closed", DoorStateClosed, "closed"},
		{"closing", DoorStateClosing, "closing"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if diff := cmp.Diff(c.want, string(c.got)); diff != "" {
				t.Errorf("DoorState %q mismatch (-want +got):\n%s", c.got, diff)
			}
		})
	}
}

func TestOperationState_StringValuesMatchAPI(t *testing.T) {
	cases := []struct {
		name string
		got  OperationState
		want string
	}{
		{"running", OperationStateRunning, "running"},
		{"stopped", OperationStateStopped, "stopped"},
		{"maintenance", OperationStateMaintenance, "maintenance"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if diff := cmp.Diff(c.want, string(c.got)); diff != "" {
				t.Errorf("OperationState %q mismatch (-want +got):\n%s", c.got, diff)
			}
		})
	}
}

func TestHallCallStatus_StringValuesMatchAPI(t *testing.T) {
	cases := []struct {
		name string
		got  HallCallStatus
		want string
	}{
		{"waiting", HallCallStatusWaiting, "waiting"},
		{"assigned", HallCallStatusAssigned, "assigned"},
		{"served", HallCallStatusServed, "served"},
		{"canceled", HallCallStatusCanceled, "canceled"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if diff := cmp.Diff(c.want, string(c.got)); diff != "" {
				t.Errorf("HallCallStatus %q mismatch (-want +got):\n%s", c.got, diff)
			}
		})
	}
}

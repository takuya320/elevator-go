package elevator

import (
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func mkHallCall(t *testing.T, floor int, dir Direction) *HallCall {
	t.Helper()
	id, err := NewHallCallID("call-1")
	if err != nil {
		t.Fatalf("NewHallCallID: %v", err)
	}
	c, err := NewHallCall(id, NewFloor(floor), dir, time.Unix(0, 0))
	if err != nil {
		t.Fatalf("NewHallCall: %v", err)
	}
	return c
}

// HallCall の検証フィールドだけを切り出した snapshot。cmp.Diff で差分可視化する。
type hallCallSnapshot struct {
	Status        HallCallStatus
	HasAssignedID bool
	AssignedID    string
}

func snapshotHallCall(c *HallCall) hallCallSnapshot {
	s := hallCallSnapshot{Status: c.Status()}
	if id := c.AssignedElevatorID(); id != nil {
		s.HasAssignedID = true
		s.AssignedID = id.String()
	}
	return s
}

func TestNewHallCall_Direction(t *testing.T) {
	cases := []struct {
		name    string
		dir     Direction
		wantErr error
	}{
		{"up ok", DirectionUp, nil},
		{"down ok", DirectionDown, nil},
		{"idle rejected", DirectionIdle, ErrInvalidHallCallDirection},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			id, _ := NewHallCallID("call-1")
			_, err := NewHallCall(id, NewFloor(5), c.dir, time.Unix(0, 0))
			if c.wantErr == nil {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				return
			}
			if !errors.Is(err, c.wantErr) {
				t.Errorf("err = %v, want %v", err, c.wantErr)
			}
		})
	}
}

func TestHallCall_InitialState(t *testing.T) {
	got := snapshotHallCall(mkHallCall(t, 5, DirectionUp))
	want := hallCallSnapshot{Status: HallCallStatusWaiting}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("initial state mismatch (-want +got):\n%s", diff)
	}
}

func TestHallCall_AssignTo(t *testing.T) {
	c := mkHallCall(t, 5, DirectionUp)
	eid, _ := NewElevatorID("ev-1")
	if err := c.AssignTo(eid); err != nil {
		t.Fatalf("AssignTo: %v", err)
	}
	want := hallCallSnapshot{Status: HallCallStatusAssigned, HasAssignedID: true, AssignedID: "ev-1"}
	if diff := cmp.Diff(want, snapshotHallCall(c)); diff != "" {
		t.Errorf("after AssignTo mismatch (-want +got):\n%s", diff)
	}
}

func TestHallCall_MarkServed(t *testing.T) {
	cases := []struct {
		name      string
		setup     func(*HallCall) // 直前に呼び出す（既存 status を変えるため）
		wantOK    bool
		wantState HallCallStatus
	}{
		{
			name:   "waiting -> served",
			setup:  func(*HallCall) {},
			wantOK: true, wantState: HallCallStatusServed,
		},
		{
			name: "served is idempotent",
			setup: func(c *HallCall) {
				if !c.MarkServed() {
					t.Fatalf("setup: first MarkServed returned false")
				}
			},
			wantOK: false, wantState: HallCallStatusServed,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := mkHallCall(t, 5, DirectionUp)
			c.setup(h)
			if got := h.MarkServed(); got != c.wantOK {
				t.Errorf("MarkServed = %v want %v", got, c.wantOK)
			}
			if diff := cmp.Diff(c.wantState, h.Status()); diff != "" {
				t.Errorf("status mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestHallCall_Cancel(t *testing.T) {
	cases := []struct {
		name       string
		setup      func() *HallCall
		wantOK     bool
		wantStatus HallCallStatus
	}{
		{
			name:   "waiting -> canceled",
			setup:  func() *HallCall { return mkHallCall(t, 5, DirectionUp) },
			wantOK: true, wantStatus: HallCallStatusCanceled,
		},
		{
			name: "assigned -> canceled",
			setup: func() *HallCall {
				c := mkHallCall(t, 5, DirectionUp)
				eid, _ := NewElevatorID("ev-1")
				_ = c.AssignTo(eid)
				return c
			},
			wantOK: true, wantStatus: HallCallStatusCanceled,
		},
		{
			name: "served -> immutable",
			setup: func() *HallCall {
				c := mkHallCall(t, 5, DirectionUp)
				c.MarkServed()
				return c
			},
			wantOK: false, wantStatus: HallCallStatusServed,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			h := c.setup()
			if got := h.Cancel(); got != c.wantOK {
				t.Errorf("Cancel = %v want %v", got, c.wantOK)
			}
			if diff := cmp.Diff(c.wantStatus, h.Status()); diff != "" {
				t.Errorf("status mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

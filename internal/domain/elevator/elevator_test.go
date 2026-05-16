package elevator

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func mkElevator(t *testing.T, id string, floor int) *Elevator {
	t.Helper()
	eid, err := NewElevatorID(id)
	if err != nil {
		t.Fatalf("NewElevatorID: %v", err)
	}
	return NewElevator(eid, NewFloor(floor))
}

func tickN(e *Elevator, n int) {
	for range n {
		e.AdvanceOneTick()
	}
}

// Elevator の検証フィールドだけを抽出した snapshot。
// 多フィールドの状態を一括 cmp.Diff で見るために使う。
type elevatorSnapshot struct {
	Floor          int
	Direction      Direction
	Door           DoorState
	OperationState OperationState
	Destinations   []int
	HoldOpen       bool
}

func snapshotElevator(e *Elevator) elevatorSnapshot {
	return elevatorSnapshot{
		Floor:          e.CurrentFloor().Value(),
		Direction:      e.Direction(),
		Door:           e.DoorState(),
		OperationState: e.OperationState(),
		Destinations:   floorsToInts(e.Destinations()),
		HoldOpen:       e.HoldOpen(),
	}
}

func TestElevator_InitialState(t *testing.T) {
	want := elevatorSnapshot{
		Floor: 1, Direction: DirectionIdle, Door: DoorStateClosed,
		OperationState: OperationStateRunning, Destinations: []int{},
	}
	got := snapshotElevator(mkElevator(t, "ev-1", 1))
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("initial state mismatch (-want +got):\n%s", diff)
	}
}

func TestElevator_AddDestination(t *testing.T) {
	cases := []struct {
		name      string
		current   int
		dest      int
		opState   OperationState
		wantErr   error
		wantState elevatorSnapshot
	}{
		{
			name:    "remote destination keeps door closed",
			current: 1, dest: 5,
			wantState: elevatorSnapshot{
				Floor: 1, Direction: DirectionIdle, Door: DoorStateClosed,
				OperationState: OperationStateRunning, Destinations: []int{5},
			},
		},
		{
			name:    "same floor opens door immediately",
			current: 5, dest: 5,
			wantState: elevatorSnapshot{
				Floor: 5, Direction: DirectionIdle, Door: DoorStateOpen,
				OperationState: OperationStateRunning, Destinations: []int{},
			},
		},
		{
			name:    "stopped rejects with no side effect",
			current: 1, dest: 5,
			opState: OperationStateStopped, wantErr: ErrElevatorNotRunning,
			wantState: elevatorSnapshot{
				Floor: 1, Direction: DirectionIdle, Door: DoorStateClosed,
				OperationState: OperationStateStopped, Destinations: []int{},
			},
		},
		{
			name:    "maintenance rejects with no side effect",
			current: 1, dest: 5,
			opState: OperationStateMaintenance, wantErr: ErrElevatorNotRunning,
			wantState: elevatorSnapshot{
				Floor: 1, Direction: DirectionIdle, Door: DoorStateClosed,
				OperationState: OperationStateMaintenance, Destinations: []int{},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := mkElevator(t, "ev-1", c.current)
			if c.opState != "" {
				e.operationState = c.opState
			}
			err := e.AddDestination(NewFloor(c.dest))
			if c.wantErr != nil {
				if !errors.Is(err, c.wantErr) {
					t.Errorf("err = %v, want %v", err, c.wantErr)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if diff := cmp.Diff(c.wantState, snapshotElevator(e)); diff != "" {
				t.Errorf("state mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// AdvanceOneTick の動作を「初期状態 + 進める tick 数 → 期待 state」で表にする。
func TestElevator_AdvanceOneTick(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T) *Elevator
		ticks int
		want  elevatorSnapshot
	}{
		{
			name:  "idle empty schedule is noop",
			setup: func(t *testing.T) *Elevator { return mkElevator(t, "ev-1", 3) },
			ticks: 1,
			want: elevatorSnapshot{
				Floor: 3, Direction: DirectionIdle, Door: DoorStateClosed,
				OperationState: OperationStateRunning, Destinations: []int{},
			},
		},
		{
			name: "moves toward next stop",
			setup: func(t *testing.T) *Elevator {
				e := mkElevator(t, "ev-1", 3)
				if err := e.AddDestination(NewFloor(5)); err != nil {
					t.Fatalf("AddDestination: %v", err)
				}
				return e
			},
			ticks: 1,
			want: elevatorSnapshot{
				Floor: 4, Direction: DirectionUp, Door: DoorStateClosed,
				OperationState: OperationStateRunning, Destinations: []int{5},
			},
		},
		{
			name: "opens door on arrival",
			setup: func(t *testing.T) *Elevator {
				e := mkElevator(t, "ev-1", 3)
				_ = e.AddDestination(NewFloor(5))
				return e
			},
			ticks: 2,
			want: elevatorSnapshot{
				Floor: 5, Direction: DirectionUp, Door: DoorStateOpen,
				OperationState: OperationStateRunning, Destinations: []int{},
			},
		},
		{
			name: "open door stays open during dwell tick",
			setup: func(t *testing.T) *Elevator {
				e := mkElevator(t, "ev-1", 5)
				_ = e.AddDestination(NewFloor(5)) // opens door
				return e
			},
			ticks: 1, // dwell を 1 消費するだけで扉は開いたまま。
			want: elevatorSnapshot{
				Floor: 5, Direction: DirectionIdle, Door: DoorStateOpen,
				OperationState: OperationStateRunning, Destinations: []int{},
			},
		},
		{
			name: "open door closes after dwell expires",
			setup: func(t *testing.T) *Elevator {
				e := mkElevator(t, "ev-1", 5)
				_ = e.AddDestination(NewFloor(5))
				return e
			},
			ticks: 2, // dwell 消費 → 閉扉。
			want: elevatorSnapshot{
				Floor: 5, Direction: DirectionIdle, Door: DoorStateClosed,
				OperationState: OperationStateRunning, Destinations: []int{},
			},
		},
		{
			name: "goes idle when schedule empties",
			setup: func(t *testing.T) *Elevator {
				e := mkElevator(t, "ev-1", 3)
				_ = e.AddDestination(NewFloor(5))
				return e
			},
			ticks: 5, // 3->4, 4->5 (open), dwell, close, idle
			want: elevatorSnapshot{
				Floor: 5, Direction: DirectionIdle, Door: DoorStateClosed,
				OperationState: OperationStateRunning, Destinations: []int{},
			},
		},
		{
			name: "stopped does not move",
			setup: func(t *testing.T) *Elevator {
				e := mkElevator(t, "ev-1", 3)
				e.stopSchedule.Add(NewFloor(5))
				e.operationState = OperationStateStopped
				return e
			},
			ticks: 1,
			want: elevatorSnapshot{
				Floor: 3, Direction: DirectionIdle, Door: DoorStateClosed,
				OperationState: OperationStateStopped, Destinations: []int{5},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := c.setup(t)
			tickN(e, c.ticks)
			if diff := cmp.Diff(c.want, snapshotElevator(e)); diff != "" {
				t.Errorf("state mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// auto-return オン時、schedule が空かつ非ホーム階なら home に向けて自動的に
// schedule が積み直されて移動が始まる。
func TestElevator_AutoReturn(t *testing.T) {
	mk := func(initial, home int, enabled bool) *Elevator {
		e := mkElevator(t, "ev-1", initial)
		e.setHomeFloor(NewFloor(home))
		e.setAutoReturnEnabled(enabled)
		return e
	}

	t.Run("disabled stays idle off-home", func(t *testing.T) {
		e := mk(5, 1, false)
		tickN(e, 3)
		if e.CurrentFloor().Value() != 5 || e.Direction() != DirectionIdle {
			t.Errorf("expected to stay at 5/idle, got %d/%s", e.CurrentFloor().Value(), e.Direction())
		}
	})

	t.Run("enabled returns to home then stays", func(t *testing.T) {
		e := mk(3, 1, true)
		// 3→2 (t1), 2→1 (t2, open + dwell), dwell (t3), close + idle (t4)
		tickN(e, 4)
		want := elevatorSnapshot{
			Floor: 1, Direction: DirectionIdle, Door: DoorStateClosed,
			OperationState: OperationStateRunning, Destinations: []int{},
		}
		if diff := cmp.Diff(want, snapshotElevator(e)); diff != "" {
			t.Errorf("after auto-return mismatch (-want +got):\n%s", diff)
		}
		// home に居る間は再度動かない。
		tickN(e, 3)
		if e.CurrentFloor().Value() != 1 || len(e.Destinations()) != 0 {
			t.Errorf("home elevator moved unexpectedly: floor=%d dests=%v",
				e.CurrentFloor().Value(), e.Destinations())
		}
	})

	t.Run("hall call interrupts auto-return", func(t *testing.T) {
		e := mk(10, 1, true)
		// 1 tick で 10→9 に向かう。途中で 7 へ呼ばれる想定。
		e.AdvanceOneTick()
		if err := e.AddDestination(NewFloor(7)); err != nil {
			t.Fatalf("AddDestination: %v", err)
		}
		// 9→8→7 (open) で 7 に到着。
		tickN(e, 2)
		if e.CurrentFloor().Value() != 7 || e.DoorState() != DoorStateOpen {
			t.Errorf("expected to stop at 7 with door open, got %d/%s",
				e.CurrentFloor().Value(), e.DoorState())
		}
	})
}

func TestElevator_HoldOpen(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T) *Elevator
		want  elevatorSnapshot
	}{
		{
			name: "OpenDoor pins door across ticks",
			setup: func(t *testing.T) *Elevator {
				e := mkElevator(t, "ev-1", 1)
				_ = e.AddDestination(NewFloor(5))
				e.OpenDoor()
				e.AdvanceOneTick() // hold-open 中は動かない
				return e
			},
			want: elevatorSnapshot{
				Floor: 1, Direction: DirectionIdle, Door: DoorStateOpen,
				OperationState: OperationStateRunning, Destinations: []int{5}, HoldOpen: true,
			},
		},
		{
			name: "CloseDoor resumes operation",
			setup: func(t *testing.T) *Elevator {
				e := mkElevator(t, "ev-1", 1)
				_ = e.AddDestination(NewFloor(5))
				e.OpenDoor()
				e.AdvanceOneTick()
				e.CloseDoor()
				e.AdvanceOneTick()
				return e
			},
			want: elevatorSnapshot{
				Floor: 2, Direction: DirectionUp, Door: DoorStateClosed,
				OperationState: OperationStateRunning, Destinations: []int{5},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if diff := cmp.Diff(c.want, snapshotElevator(c.setup(t))); diff != "" {
				t.Errorf("state mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestElevator_VisibleStatusFrom(t *testing.T) {
	cases := []struct {
		name   string
		setup  func() *Elevator
		viewer int
		want   VisibleElevatorStatus
	}{
		{
			name: "non-running -> unavailable",
			setup: func() *Elevator {
				e := mkElevator(t, "ev-1", 5)
				e.operationState = OperationStateMaintenance
				return e
			},
			viewer: 5, want: VisibleStatusUnavailable,
		},
		{
			name: "same floor + door open -> arrived",
			setup: func() *Elevator {
				e := mkElevator(t, "ev-1", 5)
				e.doorState = DoorStateOpen
				return e
			},
			viewer: 5, want: VisibleStatusArrived,
		},
		{
			name: "going up below viewer -> approaching",
			setup: func() *Elevator {
				e := mkElevator(t, "ev-1", 3)
				e.direction = DirectionUp
				return e
			},
			viewer: 7, want: VisibleStatusApproaching,
		},
		{
			name: "going down above viewer -> approaching",
			setup: func() *Elevator {
				e := mkElevator(t, "ev-1", 9)
				e.direction = DirectionDown
				return e
			},
			viewer: 5, want: VisibleStatusApproaching,
		},
		{
			name: "same floor, door closed, moving -> passing",
			setup: func() *Elevator {
				e := mkElevator(t, "ev-1", 5)
				e.direction = DirectionUp
				return e
			},
			viewer: 5, want: VisibleStatusPassing,
		},
		{
			name: "off elsewhere -> away",
			setup: func() *Elevator {
				e := mkElevator(t, "ev-1", 9)
				e.direction = DirectionUp
				return e
			},
			viewer: 5, want: VisibleStatusAway,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := c.setup().VisibleStatusFrom(NewFloor(c.viewer))
			if diff := cmp.Diff(c.want, got); diff != "" {
				t.Errorf("VisibleStatusFrom mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestElevator_StopOrder_SCAN(t *testing.T) {
	// docs/behavior.md §2 examples
	cases := []struct {
		name      string
		current   int
		direction Direction
		stops     []int
		want      []int
	}{
		{"up scan: 5,7,2", 3, DirectionUp, []int{2, 5, 7}, []int{5, 7, 2}},
		{"down scan: 6,3,10", 8, DirectionDown, []int{3, 6, 10}, []int{6, 3, 10}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := mkElevator(t, "ev-1", c.current)
			e.direction = c.direction
			for _, f := range c.stops {
				_ = e.AddDestination(NewFloor(f))
			}
			got := captureArrivals(e, 30)
			if diff := cmp.Diff(c.want, got); diff != "" {
				t.Errorf("visit order mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// captureArrivals advances the elevator up to maxTicks ticks and returns the
// floors at which the door opened, in order.
func captureArrivals(e *Elevator, maxTicks int) []int {
	visited := []int{}
	prevDoor := e.DoorState()
	for range maxTicks {
		e.AdvanceOneTick()
		if prevDoor != DoorStateOpen && e.DoorState() == DoorStateOpen {
			visited = append(visited, e.CurrentFloor().Value())
		}
		prevDoor = e.DoorState()
		if len(e.Destinations()) == 0 && e.DoorState() == DoorStateClosed {
			break
		}
	}
	return visited
}

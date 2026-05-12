package elevator

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// elevSetup はテーブルから Elevator を組み立てるためのコンパクト記述。
type elevSetup struct {
	id        string
	floor     int
	direction Direction
	opState   OperationState
}

func buildElevators(t *testing.T, specs []elevSetup) []*Elevator {
	t.Helper()
	out := make([]*Elevator, len(specs))
	for i, s := range specs {
		e := mkElevator(t, s.id, s.floor)
		if s.direction != "" {
			e.direction = s.direction
		}
		if s.opState != "" {
			e.operationState = s.opState
		}
		out[i] = e
	}
	return out
}

func TestNearestPolicy_Select(t *testing.T) {
	cases := []struct {
		name      string
		callFloor int
		callDir   Direction
		setups    []elevSetup
		wantID    string
		wantErr   error
	}{
		{
			name:      "skips non-running",
			callFloor: 5, callDir: DirectionUp,
			setups: []elevSetup{
				{id: "ev-a", floor: 4, opState: OperationStateStopped},
				{id: "ev-b", floor: 9},
			},
			wantID: "ev-b",
		},
		{
			name:      "picks nearest by distance",
			callFloor: 5, callDir: DirectionUp,
			setups: []elevSetup{
				{id: "ev-a", floor: 1}, // distance 4
				{id: "ev-b", floor: 7}, // distance 2
			},
			wantID: "ev-b",
		},
		{
			name:      "tie breaks toward idle over moving",
			callFloor: 5, callDir: DirectionUp,
			setups: []elevSetup{
				{id: "ev-a", floor: 3, direction: DirectionUp}, // distance 2, moving
				{id: "ev-b", floor: 7},                         // distance 2, idle
			},
			wantID: "ev-b",
		},
		{
			name:      "tie breaks by lowest ID",
			callFloor: 5, callDir: DirectionUp,
			setups: []elevSetup{
				{id: "ev-b", floor: 3},
				{id: "ev-a", floor: 7},
			},
			wantID: "ev-a",
		},
		{
			// 上昇中の近い号機を down 呼びに割り当ててはいけない（上昇途中で扉を開けてしまう）。
			name:      "prefers direction-compatible (down call avoids ascending elevator)",
			callFloor: 4, callDir: DirectionDown,
			setups: []elevSetup{
				{id: "ev-1", floor: 3, direction: DirectionUp}, // 距離 1, 上昇中（不整合）
				{id: "ev-2", floor: 10},                        // 距離 6, idle
			},
			wantID: "ev-2",
		},
		{
			// 上昇中で呼び階より下の号機は up 呼びと整合する（道中で拾える）。
			name:      "up-toward elevator below up-call is compatible",
			callFloor: 5, callDir: DirectionUp,
			setups: []elevSetup{
				{id: "ev-1", floor: 3, direction: DirectionUp}, // 距離 2, up（整合）
				{id: "ev-2", floor: 8},                         // 距離 3, idle
			},
			wantID: "ev-1",
		},
		{
			// 整合する号機が無いときは距離最短にフォールバック。
			name:      "fallback to nearest when no compatible elevator",
			callFloor: 4, callDir: DirectionDown,
			setups: []elevSetup{
				{id: "ev-1", floor: 3, direction: DirectionUp},
				{id: "ev-2", floor: 9, direction: DirectionUp},
			},
			wantID: "ev-1",
		},
		{
			name:      "no candidates -> ErrNoAvailableElevator",
			callFloor: 5, callDir: DirectionUp,
			setups: []elevSetup{
				{id: "ev-a", floor: 3, opState: OperationStateStopped},
				{id: "ev-b", floor: 7, opState: OperationStateMaintenance},
			},
			wantErr: ErrNoAvailableElevator,
		},
	}
	policy := NewNearestAvailableElevatorPolicy()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			call := mkHallCall(t, c.callFloor, c.callDir)
			got, err := policy.SelectElevator(call, buildElevators(t, c.setups))
			if c.wantErr != nil {
				if !errors.Is(err, c.wantErr) {
					t.Errorf("err = %v, want %v", err, c.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if diff := cmp.Diff(c.wantID, got.ID().String()); diff != "" {
				t.Errorf("selected elevator mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

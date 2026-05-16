package usecase

import (
	"time"

	"elevator-go/internal/domain/elevator"
)

type FloorRangeSnapshot struct {
	Min int
	Max int
}

type HallCallSnapshot struct {
	ID                 string
	Floor              int
	Direction          string
	Status             string
	AssignedElevatorID *string
	CreatedAt          time.Time
}

type VisibleElevatorSnapshot struct {
	ID             string
	CurrentFloor   int
	Direction      string
	DoorState      string
	OperationState string
	VisibleStatus  string
}

type ElevatorSnapshot struct {
	ID                string
	CurrentFloor      int
	Direction         string
	DoorState         string
	OperationState    string
	FloorRange        FloorRangeSnapshot
	DestinationFloors []int
	AssignedHallCalls []HallCallSnapshot
	DoorHoldOpen      bool
	HomeFloor         int
	AutoReturnEnabled bool
}

type ElevatorInit struct {
	ID                string
	InitialFloor      int
	HomeFloor         *int // nil の場合 InitialFloor を採用
	AutoReturnEnabled bool
}

// EventSnapshot は domain.DomainEvent を JSON 配信向けにフラット化したもの。
// type 別に必要なフィールドだけがゼロ値以外になる（pointer 型 = 省略可）。
type EventSnapshot struct {
	Type       string
	Timestamp  time.Time
	CallID     *string
	Floor      *int
	Direction  *string
	ElevatorID *string
	From       *string
	To         *string
}

func eventSnapshotsFromDomain(events []elevator.DomainEvent, ts time.Time) []EventSnapshot {
	if len(events) == 0 {
		return nil
	}
	out := make([]EventSnapshot, 0, len(events))
	for _, e := range events {
		snap := EventSnapshot{Type: e.EventName(), Timestamp: ts}
		switch v := e.(type) {
		case elevator.HallCallRequested:
			snap.CallID = strPtr(string(v.CallID))
			snap.Floor = intPtr(v.Floor.Value())
			snap.Direction = strPtr(string(v.Direction))
			snap.ElevatorID = strPtr(string(v.ElevatorID))
		case elevator.HallCallServed:
			snap.CallID = strPtr(string(v.CallID))
			snap.Floor = intPtr(v.Floor.Value())
			snap.ElevatorID = strPtr(string(v.ElevatorID))
		case elevator.HallCallCanceled:
			snap.CallID = strPtr(string(v.CallID))
		case elevator.CarCallRequested:
			snap.ElevatorID = strPtr(string(v.ElevatorID))
			snap.Floor = intPtr(v.Floor.Value())
		case elevator.ElevatorArrived:
			snap.ElevatorID = strPtr(string(v.ElevatorID))
			snap.Floor = intPtr(v.Floor.Value())
		case elevator.ElevatorStateChanged:
			snap.ElevatorID = strPtr(string(v.ElevatorID))
			snap.From = strPtr(string(v.From))
			snap.To = strPtr(string(v.To))
		}
		out = append(out, snap)
	}
	return out
}

func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }

func toFloorRange(spec elevator.BuildingSpec) FloorRangeSnapshot {
	return FloorRangeSnapshot{Min: spec.Min().Value(), Max: spec.Max().Value()}
}

func toHallCallSnapshot(c *elevator.HallCall) HallCallSnapshot {
	out := HallCallSnapshot{
		ID:        string(c.ID()),
		Floor:     c.Floor().Value(),
		Direction: string(c.Direction()),
		Status:    string(c.Status()),
		CreatedAt: c.CreatedAt(),
	}
	if eid := c.AssignedElevatorID(); eid != nil {
		s := string(*eid)
		out.AssignedElevatorID = &s
	}
	return out
}

func toVisibleElevatorSnapshot(v elevator.VisibleElevator) VisibleElevatorSnapshot {
	return VisibleElevatorSnapshot{
		ID:             string(v.ElevatorID),
		CurrentFloor:   v.CurrentFloor.Value(),
		Direction:      string(v.Direction),
		DoorState:      string(v.DoorState),
		OperationState: string(v.OperationState),
		VisibleStatus:  string(v.VisibleStatus),
	}
}

func toElevatorSnapshot(e *elevator.Elevator, bank *elevator.ElevatorBank) ElevatorSnapshot {
	dests := e.Destinations()
	floors := make([]int, len(dests))
	for i, f := range dests {
		floors[i] = f.Value()
	}

	var assigned []HallCallSnapshot
	for _, c := range bank.HallCalls() {
		if c.Status() != elevator.HallCallStatusAssigned {
			continue
		}
		eid := c.AssignedElevatorID()
		if eid == nil || *eid != e.ID() {
			continue
		}
		assigned = append(assigned, toHallCallSnapshot(c))
	}

	return ElevatorSnapshot{
		ID:                string(e.ID()),
		CurrentFloor:      e.CurrentFloor().Value(),
		Direction:         string(e.Direction()),
		DoorState:         string(e.DoorState()),
		OperationState:    string(e.OperationState()),
		FloorRange:        toFloorRange(bank.Spec()),
		DestinationFloors: floors,
		AssignedHallCalls: assigned,
		DoorHoldOpen:      e.HoldOpen(),
		HomeFloor:         e.HomeFloor().Value(),
		AutoReturnEnabled: e.AutoReturnEnabled(),
	}
}

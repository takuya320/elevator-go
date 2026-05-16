package server

import (
	"elevator-go/internal/interface/http/oapi"
	"elevator-go/internal/usecase"
)

func hallCallToOAPI(s usecase.HallCallSnapshot) oapi.HallCall {
	return oapi.HallCall{
		Id:                 s.ID,
		Floor:              s.Floor,
		Direction:          oapi.HallDirection(s.Direction),
		Status:             oapi.HallCallStatus(s.Status),
		AssignedElevatorId: s.AssignedElevatorID,
		CreatedAt:          s.CreatedAt,
	}
}

func visibleElevatorToOAPI(s usecase.VisibleElevatorSnapshot) oapi.VisibleElevator {
	return oapi.VisibleElevator{
		Id:             s.ID,
		CurrentFloor:   s.CurrentFloor,
		Direction:      oapi.Direction(s.Direction),
		DoorState:      oapi.DoorState(s.DoorState),
		OperationState: oapi.OperationState(s.OperationState),
		VisibleStatus:  oapi.VisibleElevatorStatus(s.VisibleStatus),
	}
}

func floorRangeToOAPI(s usecase.FloorRangeSnapshot) oapi.FloorRange {
	return oapi.FloorRange{Min: s.Min, Max: s.Max}
}

func elevatorToOAPI(s usecase.ElevatorSnapshot) oapi.Elevator {
	hallCalls := make([]oapi.HallCall, 0, len(s.AssignedHallCalls))
	for _, h := range s.AssignedHallCalls {
		hallCalls = append(hallCalls, hallCallToOAPI(h))
	}
	// JSON では空配列 [] にしたいので nil → 空スライスへ正規化。
	dests := s.DestinationFloors
	if dests == nil {
		dests = []int{}
	}
	return oapi.Elevator{
		Id:                s.ID,
		CurrentFloor:      s.CurrentFloor,
		Direction:         oapi.Direction(s.Direction),
		DoorState:         oapi.DoorState(s.DoorState),
		OperationState:    oapi.OperationState(s.OperationState),
		FloorRange:        floorRangeToOAPI(s.FloorRange),
		DestinationFloors: dests,
		AssignedHallCalls: hallCalls,
		DoorHoldOpen:      s.DoorHoldOpen,
		HomeFloor:         s.HomeFloor,
		AutoReturnEnabled: s.AutoReturnEnabled,
	}
}

func elevatorInitToOAPI(s usecase.ElevatorInit) oapi.ElevatorInit {
	out := oapi.ElevatorInit{Id: s.ID, InitialFloor: s.InitialFloor}
	// AutoReturnEnabled は false が既定なので false でもポインタを立てて返す
	// （クライアントに「明示的に無効化」と読めるよう常に出す）。
	ar := s.AutoReturnEnabled
	out.AutoReturnEnabled = &ar
	if s.HomeFloor != nil {
		out.HomeFloor = s.HomeFloor
	} else {
		// 入力で省略された場合は initialFloor を home として返す（サーバ側の解決結果）。
		hf := s.InitialFloor
		out.HomeFloor = &hf
	}
	return out
}

func eventToOAPI(s usecase.EventSnapshot) oapi.SimulationEvent {
	out := oapi.SimulationEvent{
		Type:       oapi.SimulationEventType(s.Type),
		Timestamp:  s.Timestamp,
		CallId:     s.CallID,
		Floor:      s.Floor,
		ElevatorId: s.ElevatorID,
	}
	if s.Direction != nil {
		d := oapi.HallDirection(*s.Direction)
		out.Direction = &d
	}
	if s.From != nil {
		v := oapi.OperationState(*s.From)
		out.From = &v
	}
	if s.To != nil {
		v := oapi.OperationState(*s.To)
		out.To = &v
	}
	return out
}

func eventsToOAPI(events []usecase.EventSnapshot) []oapi.SimulationEvent {
	out := make([]oapi.SimulationEvent, 0, len(events))
	for _, e := range events {
		out = append(out, eventToOAPI(e))
	}
	return out
}

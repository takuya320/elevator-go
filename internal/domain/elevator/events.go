package elevator

// EventName の値はそのまま API 上の event type として配信されるため、
// 変更すると public な契約が壊れる。
type DomainEvent interface {
	EventName() string
}

type HallCallRequested struct {
	CallID     HallCallID
	Floor      Floor
	Direction  Direction
	ElevatorID ElevatorID
}

func (HallCallRequested) EventName() string { return "hall_call.requested" }

type HallCallServed struct {
	CallID     HallCallID
	Floor      Floor
	ElevatorID ElevatorID
}

func (HallCallServed) EventName() string { return "hall_call.served" }

type HallCallCanceled struct {
	CallID HallCallID
}

func (HallCallCanceled) EventName() string { return "hall_call.canceled" }

type CarCallRequested struct {
	ElevatorID ElevatorID
	Floor      Floor
}

func (CarCallRequested) EventName() string { return "car_call.requested" }

type ElevatorArrived struct {
	ElevatorID ElevatorID
	Floor      Floor
}

func (ElevatorArrived) EventName() string { return "elevator.arrived" }

type ElevatorStateChanged struct {
	ElevatorID ElevatorID
	From       OperationState
	To         OperationState
}

func (ElevatorStateChanged) EventName() string { return "elevator.state_changed" }

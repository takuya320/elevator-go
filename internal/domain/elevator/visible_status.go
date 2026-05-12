package elevator

// 表示用 Read Model。Entity 状態と切り離してあるのは、レンダリング規則の変更が
// 集約の不変条件に影響しないようにするため。
type VisibleElevatorStatus string

const (
	VisibleStatusArrived     VisibleElevatorStatus = "arrived"
	VisibleStatusApproaching VisibleElevatorStatus = "approaching"
	VisibleStatusPassing     VisibleElevatorStatus = "passing"
	VisibleStatusAway        VisibleElevatorStatus = "away"
	VisibleStatusUnavailable VisibleElevatorStatus = "unavailable"
)

type VisibleElevator struct {
	ElevatorID     ElevatorID
	CurrentFloor   Floor
	Direction      Direction
	DoorState      DoorState
	OperationState OperationState
	VisibleStatus  VisibleElevatorStatus
}

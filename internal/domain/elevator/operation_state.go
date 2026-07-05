package elevator

type OperationState string

const (
	OperationStateRunning     OperationState = "running"
	OperationStateStopped     OperationState = "stopped"
	OperationStateMaintenance OperationState = "maintenance"
)

func (s OperationState) IsValid() bool {
	switch s {
	case OperationStateRunning, OperationStateStopped, OperationStateMaintenance:
		return true
	}
	return false
}

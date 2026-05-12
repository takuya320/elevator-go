package elevator

type OperationState string

const (
	OperationStateRunning     OperationState = "running"
	OperationStateStopped     OperationState = "stopped"
	OperationStateMaintenance OperationState = "maintenance"
)

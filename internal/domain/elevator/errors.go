package elevator

import "errors"

// HTTP コードへのマッピングは interface/http 層が一元管理する。
var (
	ErrInvalidFloor             = errors.New("invalid floor")
	ErrInvalidElevatorID        = errors.New("invalid elevator id")
	ErrInvalidHallCallDirection = errors.New("invalid hall call direction")
	ErrInvalidDestinationFloor  = errors.New("invalid destination floor")
	ErrInvalidDirection         = errors.New("invalid direction")
	ErrInvalidDoorState         = errors.New("invalid door state")
	ErrInvalidOperationState    = errors.New("invalid operation state")
	ErrElevatorNotFound         = errors.New("elevator not found")
	ErrElevatorAlreadyExists    = errors.New("elevator already exists")
	ErrHallCallNotFound         = errors.New("hall call not found")
	ErrElevatorNotRunning       = errors.New("elevator is not running")
	ErrNoAvailableElevator      = errors.New("no available elevator")
	ErrInvalidBuildingSpec      = errors.New("invalid building spec")
)

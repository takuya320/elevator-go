package elevator

import "fmt"

type ElevatorID string

func NewElevatorID(s string) (ElevatorID, error) {
	if s == "" {
		return "", fmt.Errorf("%w: id is empty", ErrInvalidElevatorID)
	}
	return ElevatorID(s), nil
}

func (id ElevatorID) String() string { return string(id) }

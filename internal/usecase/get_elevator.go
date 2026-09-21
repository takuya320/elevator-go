package usecase

import (
	"context"
	"fmt"

	"elevator-go/internal/domain/elevator"
)

type GetElevatorInput struct {
	ElevatorID string
}

type GetElevator struct {
	repo   elevator.ElevatorBankRepository
	locker Locker
}

func NewGetElevator(repo elevator.ElevatorBankRepository, locker Locker) *GetElevator {
	return &GetElevator{repo: repo, locker: locker}
}

func (u *GetElevator) Execute(ctx context.Context, in GetElevatorInput) (*ElevatorSnapshot, error) {
	u.locker.Lock()
	defer u.locker.Unlock()

	bank, err := u.repo.Find(ctx)
	if err != nil {
		return nil, err
	}
	e, ok := bank.Elevator(elevator.ElevatorID(in.ElevatorID))
	if !ok {
		return nil, fmt.Errorf("%w: %s", elevator.ErrElevatorNotFound, in.ElevatorID)
	}
	snap := toElevatorSnapshot(e, bank)
	return &snap, nil
}

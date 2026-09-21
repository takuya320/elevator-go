package usecase

import (
	"context"

	"elevator-go/internal/domain/elevator"
)

type AddElevatorInput struct {
	ID           string
	InitialFloor int
}

type AddElevator struct {
	repo   elevator.ElevatorBankRepository
	locker Locker
}

func NewAddElevator(repo elevator.ElevatorBankRepository, locker Locker) *AddElevator {
	return &AddElevator{repo: repo, locker: locker}
}

func (u *AddElevator) Execute(ctx context.Context, in AddElevatorInput) (*ElevatorSnapshot, error) {
	u.locker.Lock()
	defer u.locker.Unlock()

	bank, err := u.repo.Find(ctx)
	if err != nil {
		return nil, err
	}
	id, err := elevator.NewElevatorID(in.ID)
	if err != nil {
		return nil, err
	}
	e, err := bank.AddElevator(id, elevator.NewFloor(in.InitialFloor))
	if err != nil {
		return nil, err
	}
	if err := u.repo.Save(ctx, bank); err != nil {
		return nil, err
	}
	snap := toElevatorSnapshot(e, bank)
	return &snap, nil
}

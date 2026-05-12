package usecase

import (
	"context"

	"elevator-go/internal/domain/elevator"
)

type PressCarButtonInput struct {
	ElevatorID       string
	DestinationFloor int
}

type PressCarButton struct {
	repo   elevator.ElevatorBankRepository
	locker Locker
}

func NewPressCarButton(repo elevator.ElevatorBankRepository, locker Locker) *PressCarButton {
	return &PressCarButton{repo: repo, locker: locker}
}

func (u *PressCarButton) Execute(ctx context.Context, in PressCarButtonInput) error {
	u.locker.Lock()
	defer u.locker.Unlock()

	bank, err := u.repo.Find(ctx)
	if err != nil {
		return err
	}
	if err := bank.PressCarButton(
		elevator.ElevatorID(in.ElevatorID),
		elevator.NewFloor(in.DestinationFloor),
	); err != nil {
		return err
	}
	return u.repo.Save(ctx, bank)
}

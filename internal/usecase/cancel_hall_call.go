package usecase

import (
	"context"

	"elevator-go/internal/domain/elevator"
)

type CancelHallCallInput struct {
	CallID string
}

type CancelHallCall struct {
	repo   elevator.ElevatorBankRepository
	locker Locker
}

func NewCancelHallCall(repo elevator.ElevatorBankRepository, locker Locker) *CancelHallCall {
	return &CancelHallCall{repo: repo, locker: locker}
}

func (u *CancelHallCall) Execute(ctx context.Context, in CancelHallCallInput) error {
	u.locker.Lock()
	defer u.locker.Unlock()

	bank, err := u.repo.Find(ctx)
	if err != nil {
		return err
	}
	if err := bank.CancelHallCall(elevator.HallCallID(in.CallID)); err != nil {
		return err
	}
	return u.repo.Save(ctx, bank)
}

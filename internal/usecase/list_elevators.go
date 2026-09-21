package usecase

import (
	"context"

	"elevator-go/internal/domain/elevator"
)

type ListElevators struct {
	repo   elevator.ElevatorBankRepository
	locker Locker
}

func NewListElevators(repo elevator.ElevatorBankRepository, locker Locker) *ListElevators {
	return &ListElevators{repo: repo, locker: locker}
}

func (u *ListElevators) Execute(ctx context.Context) ([]ElevatorSnapshot, error) {
	u.locker.Lock()
	defer u.locker.Unlock()

	bank, err := u.repo.Find(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]ElevatorSnapshot, 0, len(bank.Elevators()))
	for _, e := range bank.Elevators() {
		out = append(out, toElevatorSnapshot(e, bank))
	}
	return out, nil
}

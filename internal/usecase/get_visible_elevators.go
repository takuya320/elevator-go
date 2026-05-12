package usecase

import (
	"context"

	"elevator-go/internal/domain/elevator"
)

type GetVisibleElevatorsInput struct {
	Floor int
}

type GetVisibleElevatorsOutput struct {
	Floor     int
	Elevators []VisibleElevatorSnapshot
}

type GetVisibleElevators struct {
	repo   elevator.ElevatorBankRepository
	locker Locker
}

func NewGetVisibleElevators(repo elevator.ElevatorBankRepository, locker Locker) *GetVisibleElevators {
	return &GetVisibleElevators{repo: repo, locker: locker}
}

func (u *GetVisibleElevators) Execute(ctx context.Context, in GetVisibleElevatorsInput) (*GetVisibleElevatorsOutput, error) {
	u.locker.Lock()
	defer u.locker.Unlock()

	bank, err := u.repo.Find(ctx)
	if err != nil {
		return nil, err
	}
	visibles, err := bank.VisibleElevatorsFrom(elevator.NewFloor(in.Floor))
	if err != nil {
		return nil, err
	}
	out := &GetVisibleElevatorsOutput{Floor: in.Floor}
	for _, v := range visibles {
		out.Elevators = append(out.Elevators, toVisibleElevatorSnapshot(v))
	}
	return out, nil
}

package usecase

import (
	"context"

	"elevator-go/internal/domain/elevator"
)

// 開閉ボタンは双子の usecase。run 共通部分を share するために 1 ファイルにまとめる。

type OpenDoor struct {
	repo   elevator.ElevatorBankRepository
	locker Locker
}

func NewOpenDoor(repo elevator.ElevatorBankRepository, locker Locker) *OpenDoor {
	return &OpenDoor{repo: repo, locker: locker}
}

func (u *OpenDoor) Execute(ctx context.Context, elevatorID string) (*ElevatorSnapshot, error) {
	u.locker.Lock()
	defer u.locker.Unlock()
	bank, err := u.repo.Find(ctx)
	if err != nil {
		return nil, err
	}
	e, err := bank.OpenDoor(elevator.ElevatorID(elevatorID))
	if err != nil {
		return nil, err
	}
	if err := u.repo.Save(ctx, bank); err != nil {
		return nil, err
	}
	snap := toElevatorSnapshot(e, bank)
	return &snap, nil
}

type CloseDoor struct {
	repo   elevator.ElevatorBankRepository
	locker Locker
}

func NewCloseDoor(repo elevator.ElevatorBankRepository, locker Locker) *CloseDoor {
	return &CloseDoor{repo: repo, locker: locker}
}

func (u *CloseDoor) Execute(ctx context.Context, elevatorID string) (*ElevatorSnapshot, error) {
	u.locker.Lock()
	defer u.locker.Unlock()
	bank, err := u.repo.Find(ctx)
	if err != nil {
		return nil, err
	}
	e, err := bank.CloseDoor(elevator.ElevatorID(elevatorID))
	if err != nil {
		return nil, err
	}
	if err := u.repo.Save(ctx, bank); err != nil {
		return nil, err
	}
	snap := toElevatorSnapshot(e, bank)
	return &snap, nil
}

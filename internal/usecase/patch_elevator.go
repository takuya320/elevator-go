package usecase

import (
	"context"
	"fmt"

	"elevator-go/internal/domain/elevator"
)

// 全フィールド省略可。指定したものだけ更新する。
type PatchElevatorInput struct {
	ElevatorID        string
	CurrentFloor      *int
	Direction         *string
	DoorState         *string
	OperationState    *string
	HomeFloor         *int
	AutoReturnEnabled *bool
}

type PatchElevator struct {
	repo   elevator.ElevatorBankRepository
	locker Locker
}

func NewPatchElevator(repo elevator.ElevatorBankRepository, locker Locker) *PatchElevator {
	return &PatchElevator{repo: repo, locker: locker}
}

func (u *PatchElevator) Execute(ctx context.Context, in PatchElevatorInput) (*ElevatorSnapshot, error) {
	u.locker.Lock()
	defer u.locker.Unlock()

	bank, err := u.repo.Find(ctx)
	if err != nil {
		return nil, err
	}
	patch, err := toDomainPatch(in)
	if err != nil {
		return nil, err
	}
	e, err := bank.PatchElevator(elevator.ElevatorID(in.ElevatorID), patch)
	if err != nil {
		return nil, err
	}
	if err := u.repo.Save(ctx, bank); err != nil {
		return nil, err
	}
	snap := toElevatorSnapshot(e, bank)
	return &snap, nil
}

func toDomainPatch(in PatchElevatorInput) (elevator.ElevatorPatch, error) {
	var p elevator.ElevatorPatch
	if in.CurrentFloor != nil {
		f := elevator.NewFloor(*in.CurrentFloor)
		p.CurrentFloor = &f
	}
	if in.Direction != nil {
		d := elevator.Direction(*in.Direction)
		p.Direction = &d
	}
	if in.DoorState != nil {
		s := elevator.DoorState(*in.DoorState)
		p.DoorState = &s
	}
	if in.OperationState != nil {
		s := elevator.OperationState(*in.OperationState)
		p.OperationState = &s
	}
	if in.HomeFloor != nil {
		f := elevator.NewFloor(*in.HomeFloor)
		p.HomeFloor = &f
	}
	if in.AutoReturnEnabled != nil {
		p.AutoReturnEnabled = in.AutoReturnEnabled
	}
	if p.CurrentFloor == nil && p.Direction == nil && p.DoorState == nil &&
		p.OperationState == nil && p.HomeFloor == nil && p.AutoReturnEnabled == nil {
		return p, fmt.Errorf("patch is empty: at least one field must be set")
	}
	return p, nil
}

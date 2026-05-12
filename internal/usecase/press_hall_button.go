package usecase

import (
	"context"
	"fmt"

	"elevator-go/internal/domain/elevator"
)

type PressHallButtonInput struct {
	Floor     int
	Direction string
}

type PressHallButtonOutput struct {
	Call    HallCallSnapshot
	Created bool // 既存 active call の冪等返却なら false
}

type PressHallButton struct {
	repo   elevator.ElevatorBankRepository
	locker Locker
	clock  Clock
	ids    IDGenerator
}

func NewPressHallButton(
	repo elevator.ElevatorBankRepository,
	locker Locker,
	clock Clock,
	ids IDGenerator,
) *PressHallButton {
	return &PressHallButton{repo: repo, locker: locker, clock: clock, ids: ids}
}

func (u *PressHallButton) Execute(ctx context.Context, in PressHallButtonInput) (*PressHallButtonOutput, error) {
	u.locker.Lock()
	defer u.locker.Unlock()

	bank, err := u.repo.Find(ctx)
	if err != nil {
		return nil, err
	}
	id, err := elevator.NewHallCallID(u.ids.NewID())
	if err != nil {
		return nil, fmt.Errorf("press hall button: %w", err)
	}
	call, created, err := bank.PressHallButton(
		id,
		elevator.NewFloor(in.Floor),
		elevator.Direction(in.Direction),
		u.clock.Now(),
	)
	if err != nil {
		return nil, err
	}
	if err := u.repo.Save(ctx, bank); err != nil {
		return nil, err
	}
	return &PressHallButtonOutput{Call: toHallCallSnapshot(call), Created: created}, nil
}

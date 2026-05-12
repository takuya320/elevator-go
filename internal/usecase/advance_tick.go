package usecase

import (
	"context"
	"fmt"

	"elevator-go/internal/domain/elevator"
)

type AdvanceTickOutput struct {
	Tick      int
	Elevators []ElevatorSnapshot
	Events    []EventSnapshot
}

type AdvanceTick struct {
	repo   elevator.ElevatorBankRepository
	simClk SimulationClock
	locker Locker
	clock  Clock
}

func NewAdvanceTick(
	repo elevator.ElevatorBankRepository,
	simClk SimulationClock,
	locker Locker,
	clock Clock,
) *AdvanceTick {
	return &AdvanceTick{repo: repo, simClk: simClk, locker: locker, clock: clock}
}

func (u *AdvanceTick) Execute(ctx context.Context) (*AdvanceTickOutput, error) {
	u.locker.Lock()
	defer u.locker.Unlock()

	bank, err := u.repo.Find(ctx)
	if err != nil {
		return nil, err
	}
	bank.AdvanceOneTick()
	if err := u.repo.Save(ctx, bank); err != nil {
		return nil, err
	}
	if err := u.simClk.Advance(ctx); err != nil {
		return nil, fmt.Errorf("advance simulation clock: %w", err)
	}
	tick, err := u.simClk.Tick(ctx)
	if err != nil {
		return nil, fmt.Errorf("read simulation clock: %w", err)
	}

	// drain の対象は tick 中のイベントだけでなく、前回 drain 以降の mutation 由来も含む。
	// すべて同じ tick の wall clock 時刻でタイムスタンプする。
	domainEvents := bank.DrainEvents()
	now := u.clock.Now()

	out := &AdvanceTickOutput{Tick: tick, Events: eventSnapshotsFromDomain(domainEvents, now)}
	for _, e := range bank.Elevators() {
		out.Elevators = append(out.Elevators, toElevatorSnapshot(e, bank))
	}
	return out, nil
}

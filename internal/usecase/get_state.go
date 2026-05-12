package usecase

import (
	"context"

	"elevator-go/internal/domain/elevator"
)

type GetStateOutput struct {
	Tick      int
	Elevators []ElevatorSnapshot
}

// SSE 接続直後の初期状態送信に使う。Tick を進めない。
type GetState struct {
	repo   elevator.ElevatorBankRepository
	simClk SimulationClock
	locker Locker
}

func NewGetState(repo elevator.ElevatorBankRepository, simClk SimulationClock, locker Locker) *GetState {
	return &GetState{repo: repo, simClk: simClk, locker: locker}
}

func (u *GetState) Execute(ctx context.Context) (*GetStateOutput, error) {
	u.locker.Lock()
	defer u.locker.Unlock()

	bank, err := u.repo.Find(ctx)
	if err != nil {
		return nil, err
	}
	tick, err := u.simClk.Tick(ctx)
	if err != nil {
		return nil, err
	}
	out := &GetStateOutput{Tick: tick}
	for _, e := range bank.Elevators() {
		out.Elevators = append(out.Elevators, toElevatorSnapshot(e, bank))
	}
	return out, nil
}

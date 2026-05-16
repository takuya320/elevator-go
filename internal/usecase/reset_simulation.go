package usecase

import (
	"context"
	"fmt"

	"elevator-go/internal/domain/elevator"
)

// 仕様上の defaults。main 側で env から組み立てた値で上書きされる前提。
var (
	DefaultFloorRange = FloorRangeSnapshot{Min: 1, Max: 10}
	DefaultElevators  = []ElevatorInit{
		{ID: "ev-1", InitialFloor: 1},
		{ID: "ev-2", InitialFloor: 10},
	}
)

type ResetSimulationInput struct {
	FloorRange *FloorRangeSnapshot
	Elevators  []ElevatorInit
}

type ResetSimulationOutput struct {
	FloorRange FloorRangeSnapshot
	Elevators  []ElevatorInit
}

// 入力で省略されたときに使う defaults を構築時に固定する。
// UI の「reset」ボタン（空 body の POST）でも起動時設定が再現される。
type ResetSimulation struct {
	repo             elevator.ElevatorBankRepository
	simClk           SimulationClock
	locker           Locker
	defaultRange     FloorRangeSnapshot
	defaultElevators []ElevatorInit
}

func NewResetSimulation(
	repo elevator.ElevatorBankRepository,
	simClk SimulationClock,
	locker Locker,
	defaultRange FloorRangeSnapshot,
	defaultElevators []ElevatorInit,
) *ResetSimulation {
	return &ResetSimulation{
		repo:             repo,
		simClk:           simClk,
		locker:           locker,
		defaultRange:     defaultRange,
		defaultElevators: defaultElevators,
	}
}

// bank と tick カウンタを単一 Locker 配下で同時に置き換える。
func (u *ResetSimulation) Execute(ctx context.Context, in ResetSimulationInput) (*ResetSimulationOutput, error) {
	u.locker.Lock()
	defer u.locker.Unlock()

	rng := u.defaultRange
	if in.FloorRange != nil {
		rng = *in.FloorRange
	}
	inits := in.Elevators
	if len(inits) == 0 {
		inits = u.defaultElevators
	}

	spec, err := elevator.NewBuildingSpec(elevator.NewFloor(rng.Min), elevator.NewFloor(rng.Max))
	if err != nil {
		return nil, err
	}
	bank := elevator.NewElevatorBank(spec, nil)
	for _, init := range inits {
		eid, err := elevator.NewElevatorID(init.ID)
		if err != nil {
			return nil, fmt.Errorf("reset: elevator id %q: %w", init.ID, err)
		}
		if _, err := bank.AddElevator(eid, elevator.NewFloor(init.InitialFloor)); err != nil {
			return nil, fmt.Errorf("reset: elevator %s: %w", init.ID, err)
		}
		// home floor 指定があれば Patch 経由で範囲チェックしつつ反映する。
		// 省略時の home は AddElevator で initialFloor が初期値として入っている。
		patch := elevator.ElevatorPatch{}
		dirty := false
		if init.HomeFloor != nil {
			f := elevator.NewFloor(*init.HomeFloor)
			patch.HomeFloor = &f
			dirty = true
		}
		if init.AutoReturnEnabled {
			ar := true
			patch.AutoReturnEnabled = &ar
			dirty = true
		}
		if dirty {
			if _, err := bank.PatchElevator(eid, patch); err != nil {
				return nil, fmt.Errorf("reset: elevator %s config: %w", init.ID, err)
			}
		}
	}
	if err := u.repo.Save(ctx, bank); err != nil {
		return nil, err
	}
	if err := u.simClk.Reset(ctx); err != nil {
		return nil, fmt.Errorf("reset simulation clock: %w", err)
	}
	return &ResetSimulationOutput{FloorRange: rng, Elevators: inits}, nil
}

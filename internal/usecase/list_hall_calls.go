package usecase

import (
	"context"
	"fmt"

	"elevator-go/internal/domain/elevator"
)

// Statuses が空なら status で絞り込まない（served / canceled も含む全件）。
type ListHallCallsInput struct {
	Statuses []string
	Floor    *int
}

// 絞り込みは表示都合のクエリなのでドメインに持ち込まず UseCase で行う。
type ListHallCalls struct {
	repo   elevator.ElevatorBankRepository
	locker Locker
}

func NewListHallCalls(repo elevator.ElevatorBankRepository, locker Locker) *ListHallCalls {
	return &ListHallCalls{repo: repo, locker: locker}
}

func (u *ListHallCalls) Execute(ctx context.Context, in ListHallCallsInput) ([]HallCallSnapshot, error) {
	wanted, err := parseStatuses(in.Statuses)
	if err != nil {
		return nil, err
	}

	u.locker.Lock()
	defer u.locker.Unlock()

	bank, err := u.repo.Find(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]HallCallSnapshot, 0)
	for _, c := range bank.HallCalls() {
		if len(wanted) > 0 {
			if _, ok := wanted[c.Status()]; !ok {
				continue
			}
		}
		if in.Floor != nil && c.Floor().Value() != *in.Floor {
			continue
		}
		out = append(out, toHallCallSnapshot(c))
	}
	return out, nil
}

type ListFloorHallCallsInput struct {
	Floor int
}

// /floors/{floor}/... は階そのものがリソースなので、範囲外は空配列ではなく
// ErrInvalidFloor (400) にする。横断検索の ?floor= は単なる絞り込みなので検証しない。
type ListFloorHallCalls struct {
	repo   elevator.ElevatorBankRepository
	locker Locker
}

func NewListFloorHallCalls(repo elevator.ElevatorBankRepository, locker Locker) *ListFloorHallCalls {
	return &ListFloorHallCalls{repo: repo, locker: locker}
}

func (u *ListFloorHallCalls) Execute(ctx context.Context, in ListFloorHallCallsInput) ([]HallCallSnapshot, error) {
	u.locker.Lock()
	defer u.locker.Unlock()

	bank, err := u.repo.Find(ctx)
	if err != nil {
		return nil, err
	}
	floor := elevator.NewFloor(in.Floor)
	spec := bank.Spec()
	if !spec.Contains(floor) {
		return nil, fmt.Errorf("%w: floor=%d outside [%d, %d]",
			elevator.ErrInvalidFloor, in.Floor, spec.Min().Value(), spec.Max().Value())
	}
	out := make([]HallCallSnapshot, 0)
	for _, c := range bank.HallCalls() {
		if !c.Floor().Equals(floor) {
			continue
		}
		out = append(out, toHallCallSnapshot(c))
	}
	return out, nil
}

func parseStatuses(raw []string) (map[elevator.HallCallStatus]struct{}, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	out := make(map[elevator.HallCallStatus]struct{}, len(raw))
	for _, s := range raw {
		st := elevator.HallCallStatus(s)
		if !st.IsValid() {
			return nil, fmt.Errorf("%w: %q", elevator.ErrInvalidHallCallStatus, s)
		}
		out[st] = struct{}{}
	}
	return out, nil
}

package elevator

import "sort"

// 上昇中の号機が下方向呼びを取ると、4F で扉を開けてから 10F に向かうような
// 利用者にとって不自然な挙動になる。それを避けるため、距離より「進行方向の整合」を
// 優先する。整合する候補が居なければ従来どおり距離最短にフォールバックする。
//
// 整合の定義:
//   - idle: 常に整合
//   - up + 現在階 ≤ 呼び階: 整合（その階を通過する道中に拾える）
//   - down + 現在階 ≥ 呼び階: 整合
//   - 上記以外: 不整合（呼び階を通り過ぎている / 逆方向）
//
// タイブレーク順:
//  1. running のみ候補
//  2. 整合する候補を優先
//  3. 距離最小
//  4. 同距離は idle を優先
//  5. それでも同点なら ElevatorID 昇順
type NearestAvailableElevatorPolicy struct{}

func NewNearestAvailableElevatorPolicy() NearestAvailableElevatorPolicy {
	return NearestAvailableElevatorPolicy{}
}

func (NearestAvailableElevatorPolicy) SelectElevator(call *HallCall, candidates []*Elevator) (*Elevator, error) {
	type scored struct {
		e          *Elevator
		distance   int
		idle       bool
		compatible bool
	}
	pool := make([]scored, 0, len(candidates))
	for _, e := range candidates {
		if !e.IsRunning() {
			continue
		}
		pool = append(pool, scored{
			e:          e,
			distance:   e.CurrentFloor().Distance(call.Floor()),
			idle:       e.Direction() == DirectionIdle,
			compatible: directionCompatible(e, call),
		})
	}
	if len(pool) == 0 {
		return nil, ErrNoAvailableElevator
	}
	sort.SliceStable(pool, func(i, j int) bool {
		if pool[i].compatible != pool[j].compatible {
			return pool[i].compatible
		}
		if pool[i].distance != pool[j].distance {
			return pool[i].distance < pool[j].distance
		}
		if pool[i].idle != pool[j].idle {
			return pool[i].idle
		}
		return pool[i].e.ID() < pool[j].e.ID()
	})
	return pool[0].e, nil
}

func directionCompatible(e *Elevator, call *HallCall) bool {
	switch e.Direction() {
	case DirectionIdle:
		return true
	case DirectionUp:
		return call.Direction() == DirectionUp && e.CurrentFloor().Value() <= call.Floor().Value()
	case DirectionDown:
		return call.Direction() == DirectionDown && e.CurrentFloor().Value() >= call.Floor().Value()
	}
	return false
}

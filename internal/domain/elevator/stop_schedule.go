package elevator

import "sort"

type StopSchedule struct {
	floors map[int]struct{}
}

func NewStopSchedule() StopSchedule {
	return StopSchedule{floors: map[int]struct{}{}}
}

func (s *StopSchedule) Add(f Floor) {
	if s.floors == nil {
		s.floors = map[int]struct{}{}
	}
	s.floors[f.Value()] = struct{}{}
}

func (s *StopSchedule) Remove(f Floor) {
	delete(s.floors, f.Value())
}

func (s StopSchedule) Has(f Floor) bool {
	_, ok := s.floors[f.Value()]
	return ok
}

func (s StopSchedule) IsEmpty() bool { return len(s.floors) == 0 }
func (s StopSchedule) Size() int     { return len(s.floors) }

// 返り値は新規スライス。呼び出し側の変更は schedule に影響しない。
func (s StopSchedule) Floors() []Floor {
	if len(s.floors) == 0 {
		return nil
	}
	xs := make([]int, 0, len(s.floors))
	for v := range s.floors {
		xs = append(xs, v)
	}
	sort.Ints(xs)
	out := make([]Floor, len(xs))
	for i, v := range xs {
		out[i] = NewFloor(v)
	}
	return out
}

// SCAN ライク:
//   - up:   現在階より上で最も近い停止階。なければ最下階に折り返す。
//   - down: 現在階より下で最も近い停止階。なければ最上階に折り返す。
//   - idle: 距離最小。同距離は上方を優先（決定論のため）。
func (s StopSchedule) NextFloor(current Floor, dir Direction) (Floor, bool) {
	if s.IsEmpty() {
		return Floor{}, false
	}
	floors := s.Floors()
	cur := current.Value()

	switch dir {
	case DirectionUp:
		for _, f := range floors {
			if f.Value() > cur {
				return f, true
			}
		}
		return floors[0], true
	case DirectionDown:
		for i := len(floors) - 1; i >= 0; i-- {
			if floors[i].Value() < cur {
				return floors[i], true
			}
		}
		return floors[len(floors)-1], true
	default:
		best := floors[0]
		bestDist := best.Distance(current)
		for _, f := range floors[1:] {
			d := f.Distance(current)
			if d < bestDist || (d == bestDist && f.Value() > best.Value()) {
				best, bestDist = f, d
			}
		}
		return best, true
	}
}

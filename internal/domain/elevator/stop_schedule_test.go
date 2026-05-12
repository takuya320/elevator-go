package elevator

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// floorsToInts は cmp.Diff で比較しやすいよう []Floor を []int に展開する。
func floorsToInts(fs []Floor) []int {
	out := make([]int, len(fs))
	for i, f := range fs {
		out[i] = f.Value()
	}
	return out
}

func TestStopSchedule_Add(t *testing.T) {
	cases := []struct {
		name string
		ops  []int // Add 呼び出し順
		want []int // Floors() の期待値（昇順）
	}{
		{"single", []int{5}, []int{5}},
		{"idempotent dup", []int{5, 5}, []int{5}},
		{"multi sorted", []int{7, 2, 5}, []int{2, 5, 7}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := NewStopSchedule()
			for _, v := range c.ops {
				s.Add(NewFloor(v))
			}
			if diff := cmp.Diff(c.want, floorsToInts(s.Floors())); diff != "" {
				t.Errorf("Floors() mismatch (-want +got):\n%s", diff)
			}
			if diff := cmp.Diff(len(c.want), s.Size()); diff != "" {
				t.Errorf("Size() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestStopSchedule_RemoveMissingIsNoop(t *testing.T) {
	s := NewStopSchedule()
	s.Remove(NewFloor(3)) // must not panic
	if !s.IsEmpty() {
		t.Errorf("expected empty schedule")
	}
}

func TestStopSchedule_FloorsReturnsFreshSlice(t *testing.T) {
	s := NewStopSchedule()
	for _, v := range []int{7, 2, 5} {
		s.Add(NewFloor(v))
	}
	got := s.Floors()
	got[0] = NewFloor(100) // 返却スライスを汚染しても内部状態に影響してはならない
	if !s.Has(NewFloor(2)) {
		t.Errorf("returned slice was not a fresh copy")
	}
}

func TestStopSchedule_NextFloor(t *testing.T) {
	mk := func(vs ...int) StopSchedule {
		s := NewStopSchedule()
		for _, v := range vs {
			s.Add(NewFloor(v))
		}
		return s
	}
	cases := []struct {
		name    string
		current int
		dir     Direction
		stops   []int
		wantOK  bool
		want    int
	}{
		// behavior.md §2 examples
		{"up: nearest above", 3, DirectionUp, []int{2, 5, 7}, true, 5},
		{"up: reverse to lowest after passing all", 8, DirectionUp, []int{2, 5, 7}, true, 2},
		{"down: nearest below", 8, DirectionDown, []int{3, 6, 10}, true, 6},
		{"down: reverse to highest", 1, DirectionDown, []int{3, 6, 10}, true, 10},
		{"idle: closest wins (7 over 2 from 5)", 5, DirectionIdle, []int{2, 7}, true, 7},
		{"idle: tie broken by going up", 5, DirectionIdle, []int{3, 7}, true, 7},
		{"empty schedule -> ok=false", 5, DirectionUp, nil, false, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			next, ok := mk(c.stops...).NextFloor(NewFloor(c.current), c.dir)
			if ok != c.wantOK {
				t.Fatalf("ok = %v want %v", ok, c.wantOK)
			}
			if !c.wantOK {
				return
			}
			if diff := cmp.Diff(c.want, next.Value()); diff != "" {
				t.Errorf("NextFloor mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

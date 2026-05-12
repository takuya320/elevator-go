package elevator

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestNewBuildingSpec(t *testing.T) {
	cases := []struct {
		name     string
		min, max int
		wantErr  error
	}{
		{name: "valid", min: 1, max: 10},
		{name: "negative min ok", min: -2, max: 20},
		{name: "min == max rejected", min: 5, max: 5, wantErr: ErrInvalidBuildingSpec},
		{name: "min > max rejected", min: 10, max: 1, wantErr: ErrInvalidBuildingSpec},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := NewBuildingSpec(NewFloor(c.min), NewFloor(c.max))
			if c.wantErr != nil {
				if !errors.Is(err, c.wantErr) {
					t.Errorf("err = %v, want %v", err, c.wantErr)
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestBuildingSpec_Contains(t *testing.T) {
	spec, _ := NewBuildingSpec(NewFloor(1), NewFloor(10))
	cases := []struct {
		name string
		f    int
		want bool
	}{
		{"min boundary", 1, true},
		{"max boundary", 10, true},
		{"interior", 5, true},
		{"below min", 0, false},
		{"above max", 11, false},
		{"negative", -1, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if diff := cmp.Diff(c.want, spec.Contains(NewFloor(c.f))); diff != "" {
				t.Errorf("Contains(%d) mismatch (-want +got):\n%s", c.f, diff)
			}
		})
	}
}

func TestBuildingSpec_CanCall(t *testing.T) {
	spec, _ := NewBuildingSpec(NewFloor(1), NewFloor(10))
	cases := []struct {
		name string
		f    int
		d    Direction
		want bool
	}{
		{"interior up", 5, DirectionUp, true},
		{"interior down", 5, DirectionDown, true},
		{"top up rejected", 10, DirectionUp, false},
		{"bottom down rejected", 1, DirectionDown, false},
		{"top down ok", 10, DirectionDown, true},
		{"bottom up ok", 1, DirectionUp, true},
		{"idle rejected", 5, DirectionIdle, false},
		{"out of range below", 0, DirectionUp, false},
		{"out of range above", 11, DirectionDown, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if diff := cmp.Diff(c.want, spec.CanCall(NewFloor(c.f), c.d)); diff != "" {
				t.Errorf("CanCall(%d,%s) mismatch (-want +got):\n%s", c.f, c.d, diff)
			}
		})
	}
}

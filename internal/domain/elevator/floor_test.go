package elevator

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestFloor_Value(t *testing.T) {
	cases := []struct {
		name string
		in   int
		want int
	}{
		{"positive", 3, 3},
		{"zero", 0, 0},
		{"negative", -2, -2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if diff := cmp.Diff(c.want, NewFloor(c.in).Value()); diff != "" {
				t.Errorf("Value() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestFloor_AboveBelow(t *testing.T) {
	cases := []struct {
		name string
		in   int
		want struct{ above, below int }
	}{
		{"positive", 3, struct{ above, below int }{4, 2}},
		{"around zero", 0, struct{ above, below int }{1, -1}},
		{"negative", -5, struct{ above, below int }{-4, -6}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := struct{ above, below int }{
				above: NewFloor(c.in).Above().Value(),
				below: NewFloor(c.in).Below().Value(),
			}
			if diff := cmp.Diff(c.want, got, cmp.AllowUnexported(got)); diff != "" {
				t.Errorf("Above/Below mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestFloor_Equals(t *testing.T) {
	cases := []struct {
		name string
		a, b int
		want bool
	}{
		{"same positive", 1, 1, true},
		{"different", 1, 2, false},
		{"same negative", -1, -1, true},
		{"zero", 0, 0, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if diff := cmp.Diff(c.want, NewFloor(c.a).Equals(NewFloor(c.b))); diff != "" {
				t.Errorf("Equals(%d,%d) mismatch (-want +got):\n%s", c.a, c.b, diff)
			}
		})
	}
}

func TestFloor_Distance(t *testing.T) {
	cases := []struct {
		name string
		a, b int
		want int
	}{
		{"upward", 3, 7, 4},
		{"downward", 7, 3, 4},
		{"same", 5, 5, 0},
		{"crosses zero", -2, 3, 5},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if diff := cmp.Diff(c.want, NewFloor(c.a).Distance(NewFloor(c.b))); diff != "" {
				t.Errorf("Distance(%d,%d) mismatch (-want +got):\n%s", c.a, c.b, diff)
			}
		})
	}
}

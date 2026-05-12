package elevator

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestDirection_IsMoving(t *testing.T) {
	cases := []struct {
		name string
		d    Direction
		want bool
	}{
		{"up moves", DirectionUp, true},
		{"down moves", DirectionDown, true},
		{"idle stays", DirectionIdle, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if diff := cmp.Diff(c.want, c.d.IsMoving()); diff != "" {
				t.Errorf("(%s).IsMoving() mismatch (-want +got):\n%s", c.d, diff)
			}
		})
	}
}

// docs/openapi.yaml の enum と一致しないと API が壊れる。renames は OK だが
// underlying string の変更は破壊的変更なのでここで失敗させる。
func TestDirection_StringValuesMatchAPI(t *testing.T) {
	cases := []struct {
		name string
		d    Direction
		want string
	}{
		{"up", DirectionUp, "up"},
		{"down", DirectionDown, "down"},
		{"idle", DirectionIdle, "idle"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if diff := cmp.Diff(c.want, string(c.d)); diff != "" {
				t.Errorf("Direction string mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

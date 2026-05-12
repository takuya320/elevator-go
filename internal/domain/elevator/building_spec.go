package elevator

import "fmt"

type BuildingSpec struct {
	min Floor
	max Floor
}

func NewBuildingSpec(min, max Floor) (BuildingSpec, error) {
	if min.Value() >= max.Value() {
		return BuildingSpec{}, fmt.Errorf("%w: min=%d max=%d", ErrInvalidBuildingSpec, min.Value(), max.Value())
	}
	return BuildingSpec{min: min, max: max}, nil
}

func (b BuildingSpec) Min() Floor { return b.min }
func (b BuildingSpec) Max() Floor { return b.max }

func (b BuildingSpec) Contains(f Floor) bool {
	return f.Value() >= b.min.Value() && f.Value() <= b.max.Value()
}

// 範囲外、idle 方向、最上階 up、最下階 down はいずれも不可。
func (b BuildingSpec) CanCall(f Floor, d Direction) bool {
	if !b.Contains(f) {
		return false
	}
	switch d {
	case DirectionUp:
		return !f.Equals(b.max)
	case DirectionDown:
		return !f.Equals(b.min)
	default:
		return false
	}
}

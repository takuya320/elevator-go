package elevator

// 範囲は BuildingSpec が判断する。Floor 自身は範囲を持たない（地下階に対応するため）。
type Floor struct {
	value int
}

func NewFloor(v int) Floor { return Floor{value: v} }

func (f Floor) Value() int          { return f.value }
func (f Floor) Above() Floor        { return Floor{value: f.value + 1} }
func (f Floor) Below() Floor        { return Floor{value: f.value - 1} }
func (f Floor) Equals(o Floor) bool { return f.value == o.value }

func (f Floor) Distance(o Floor) int {
	if f.value > o.value {
		return f.value - o.value
	}
	return o.value - f.value
}

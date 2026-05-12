package elevator

// 文字列値は API enum と一致させる。
type Direction string

const (
	DirectionUp   Direction = "up"
	DirectionDown Direction = "down"
	DirectionIdle Direction = "idle"
)

func (d Direction) IsMoving() bool {
	return d == DirectionUp || d == DirectionDown
}

func (d Direction) IsValid() bool {
	switch d {
	case DirectionUp, DirectionDown, DirectionIdle:
		return true
	}
	return false
}

package usecase

// 用途は HallCallID 生成のみ。ElevatorID は設定値として受け取る。
type IDGenerator interface {
	NewID() string
}

package elevator

// 純粋関数として実装すること。ログから配車決定が再現できることを保証する。
type DispatchPolicy interface {
	SelectElevator(call *HallCall, candidates []*Elevator) (*Elevator, error)
}

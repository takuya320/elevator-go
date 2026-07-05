package elevator

// MVP では open / closed のみ遷移する。
// opening / closing は将来のドア多段遷移用に enum に残してある。
type DoorState string

const (
	DoorStateOpen    DoorState = "open"
	DoorStateOpening DoorState = "opening"
	DoorStateClosed  DoorState = "closed"
	DoorStateClosing DoorState = "closing"
)

// 外部入力（PATCH）の検証用。opening / closing は API enum に予約されているだけで
// MVP では遷移しない値のため、状態として書き込ませない
// （レスポンスに「返さない」はずの値が混入するのを防ぐ）。
func (d DoorState) IsValid() bool {
	switch d {
	case DoorStateOpen, DoorStateClosed:
		return true
	}
	return false
}

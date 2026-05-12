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

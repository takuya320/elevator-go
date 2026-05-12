package elevator

type HallCallStatus string

const (
	HallCallStatusWaiting  HallCallStatus = "waiting"
	HallCallStatusAssigned HallCallStatus = "assigned"
	HallCallStatusServed   HallCallStatus = "served"
	HallCallStatusCanceled HallCallStatus = "canceled"
)

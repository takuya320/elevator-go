package elevator

type HallCallStatus string

const (
	HallCallStatusWaiting  HallCallStatus = "waiting"
	HallCallStatusAssigned HallCallStatus = "assigned"
	HallCallStatusServed   HallCallStatus = "served"
	HallCallStatusCanceled HallCallStatus = "canceled"
)

func (s HallCallStatus) IsValid() bool {
	switch s {
	case HallCallStatusWaiting, HallCallStatusAssigned, HallCallStatusServed, HallCallStatusCanceled:
		return true
	}
	return false
}

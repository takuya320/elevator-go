package elevator

import (
	"fmt"
	"time"
)

// id と createdAt を引数で受けることで、ドメインを ID 生成器・wall clock から切り離す。
type HallCall struct {
	id                 HallCallID
	floor              Floor
	direction          Direction
	status             HallCallStatus
	assignedElevatorID *ElevatorID
	createdAt          time.Time
}

// idle 方向のホール呼びは存在しない。
func NewHallCall(id HallCallID, floor Floor, direction Direction, createdAt time.Time) (*HallCall, error) {
	if direction != DirectionUp && direction != DirectionDown {
		return nil, fmt.Errorf("%w: %q", ErrInvalidHallCallDirection, direction)
	}
	return &HallCall{
		id:        id,
		floor:     floor,
		direction: direction,
		status:    HallCallStatusWaiting,
		createdAt: createdAt,
	}, nil
}

func (c *HallCall) ID() HallCallID         { return c.id }
func (c *HallCall) Floor() Floor           { return c.floor }
func (c *HallCall) Direction() Direction   { return c.direction }
func (c *HallCall) Status() HallCallStatus { return c.status }
func (c *HallCall) CreatedAt() time.Time   { return c.createdAt }

// 内部参照を漏らさないようコピーした値へのポインタを返す。
func (c *HallCall) AssignedElevatorID() *ElevatorID {
	if c.assignedElevatorID == nil {
		return nil
	}
	id := *c.assignedElevatorID
	return &id
}

func (c *HallCall) IsActive() bool {
	return c.status == HallCallStatusWaiting || c.status == HallCallStatusAssigned
}

func (c *HallCall) AssignTo(eid ElevatorID) error {
	if !c.IsActive() {
		return fmt.Errorf("hall call %s: cannot assign in status %s", c.id, c.status)
	}
	c.status = HallCallStatusAssigned
	id := eid
	c.assignedElevatorID = &id
	return nil
}

func (c *HallCall) MarkServed() bool {
	if !c.IsActive() {
		return false
	}
	c.status = HallCallStatusServed
	return true
}

// served / canceled は不変。
func (c *HallCall) Cancel() bool {
	if c.status == HallCallStatusServed || c.status == HallCallStatusCanceled {
		return false
	}
	c.status = HallCallStatusCanceled
	return true
}

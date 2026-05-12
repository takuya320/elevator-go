package elevator

import "fmt"

// 生成は usecase の責務。ここでは空文字だけ弾く。
type HallCallID string

func NewHallCallID(s string) (HallCallID, error) {
	if s == "" {
		return "", fmt.Errorf("hall call id is empty")
	}
	return HallCallID(s), nil
}

func (id HallCallID) String() string { return string(id) }

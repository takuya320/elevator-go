package elevator

import "context"

// 並行制御は usecase 層 (Locker) の責務。Repository は Find/Save に留める。
type ElevatorBankRepository interface {
	Find(ctx context.Context) (*ElevatorBank, error)
	Save(ctx context.Context, bank *ElevatorBank) error
}

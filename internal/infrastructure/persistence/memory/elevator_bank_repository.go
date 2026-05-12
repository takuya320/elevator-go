// Package memory はインメモリ実装。並行制御は usecase.Locker に委ねる。
package memory

import (
	"context"
	"errors"

	"elevator-go/internal/domain/elevator"
)

var ErrNotInitialized = errors.New("elevator bank not initialized")

// Find は live ポインタを返す。Save は bank ごと差し替える（Reset 用途）。
// インプロセスでは「live オブジェクトが永続化そのもの」だが、DB 実装では Save が
// トランザクション書き込みになる。
type ElevatorBankRepository struct {
	bank *elevator.ElevatorBank
}

func NewElevatorBankRepository() *ElevatorBankRepository {
	return &ElevatorBankRepository{}
}

func (r *ElevatorBankRepository) SetInitial(bank *elevator.ElevatorBank) {
	r.bank = bank
}

func (r *ElevatorBankRepository) Find(_ context.Context) (*elevator.ElevatorBank, error) {
	if r.bank == nil {
		return nil, ErrNotInitialized
	}
	return r.bank, nil
}

func (r *ElevatorBankRepository) Save(_ context.Context, bank *elevator.ElevatorBank) error {
	r.bank = bank
	return nil
}

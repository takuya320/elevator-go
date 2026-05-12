package usecase

import "context"

// 論理 tick カウンタ。wall clock とは別概念のため Clock とは分離する。
// シミュレーション文脈は将来的に独立したコンテキストになる想定。
type SimulationClock interface {
	Tick(ctx context.Context) (int, error)
	Advance(ctx context.Context) error
	Reset(ctx context.Context) error
}

package usecase

import "time"

// テストで決定論的時刻を注入できるよう抽象化する。
type Clock interface {
	Now() time.Time
}

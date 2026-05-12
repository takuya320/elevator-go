package usecase

// 全 UseCase で共有する一個の Locker により、Find→処理→Save を直列化する。
// ElevatorBank はスレッドセーフでないため、これがないとティックとリクエストが
// 同時に走った際に状態が壊れる。
type Locker interface {
	Lock()
	Unlock()
}

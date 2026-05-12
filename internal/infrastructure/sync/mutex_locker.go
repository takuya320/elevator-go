package sync

import "sync"

type MutexLocker struct {
	mu sync.Mutex
}

func NewMutexLocker() *MutexLocker { return &MutexLocker{} }

func (l *MutexLocker) Lock()   { l.mu.Lock() }
func (l *MutexLocker) Unlock() { l.mu.Unlock() }

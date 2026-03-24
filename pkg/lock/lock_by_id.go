package lock

import "sync"

// LockManager manages locks for different IDs
type lockManager struct {
	locks sync.Map
}

// LockManagerInstance an instance of LockManager
var LockManagerInstance = lockManager{locks: sync.Map{}}

// Size returns the number of entries in the lock manager.
func (lm *lockManager) Size() int {
	count := 0
	lm.locks.Range(func(_, _ any) bool {
		count++
		return true
	})
	return count
}

// GetLock retrieves a lock for the given ID, creating one if it doesn't exist
func (lm *lockManager) GetLock(name string) *Mutex {
	getted, _ := lm.locks.LoadOrStore(name, &Mutex{})
	res, ok := getted.(*Mutex)
	if !ok {
		panic("lockManager GetLock failed")
	}
	return res
}

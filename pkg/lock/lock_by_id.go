package lock

// LockManager manages locks for different IDs
type LockManager struct {
	mu    RWMutex
	locks map[string]*Mutex
}

// Exported instance of LockManager
var LockManagerInstance = &LockManager{
	locks: make(map[string]*Mutex), // Initialize the map
}

// GetLock retrieves a lock for the given ID, creating one if it doesn't exist
func (lm *LockManager) GetLock(name string) *Mutex {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	if lock, ok := lm.locks[name]; ok {
		return lock
	}

	// Create a new lock
	newLock := &Mutex{}
	lm.locks[name] = newLock
	return newLock
}

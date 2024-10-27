//go:build scheduler.threads

package task

// PMutex is a real mutex on systems that can be either preemptive or threaded,
// and a dummy lock on other (purely cooperative) systems.
//
// It is mainly useful for short operations that need a lock when threading may
// be involved, but which do not need a lock with a purely cooperative
// scheduler.
type PMutex struct {
	futex Futex
}

func (m *PMutex) Lock() {
	// Fast path: try to take an uncontended lock.
	if m.futex.CompareAndSwap(0, 1) {
		// We obtained the mutex.
		return
	}

	// Try to lock the mutex. If it changed from 0 to 2, we took a contended
	// lock.
	for m.futex.Swap(2) != 0 {
		// Wait until we get resumed in Unlock.
		m.futex.Wait(2)
	}
}

func (m *PMutex) Unlock() {
	if old := m.futex.Swap(0); old == 2 {
		// Mutex was a contended lock, so we need to wake the next waiter.
		m.futex.Wake()
	}
	// Note: this implementation doesn't check for an unlock of an unlocked
	// mutex to keep it as lightweight as possible.
}

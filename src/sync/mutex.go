package sync

import (
	"internal/task"
	_ "unsafe"
)

type Mutex struct {
	futex task.Futex
}

//go:linkname scheduleTask runtime.scheduleTask
func scheduleTask(*task.Task)

func (m *Mutex) Lock() {
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

func (m *Mutex) Unlock() {
	if old := m.futex.Swap(0); old == 0 {
		// Mutex wasn't locked before.
		panic("sync: unlock of unlocked Mutex")
	} else if old == 2 {
		// Mutex was a contended lock, so we need to wake the next waiter.
		m.futex.Wake()
	}
}

// TryLock tries to lock m and reports whether it succeeded.
//
// Note that while correct uses of TryLock do exist, they are rare,
// and use of TryLock is often a sign of a deeper problem
// in a particular use of mutexes.
func (m *Mutex) TryLock() bool {
	// Fast path: try to take an uncontended lock.
	if m.futex.CompareAndSwap(0, 1) {
		// We obtained the mutex.
		return true
	}
	return false
}

type RWMutex struct {
	// waitingWriters are all of the tasks waiting for write locks.
	waitingWriters task.Stack

	// waitingReaders are all of the tasks waiting for a read lock.
	waitingReaders task.Stack

	// state is the current state of the RWMutex.
	// Iff the mutex is completely unlocked, it contains rwMutexStateUnlocked (aka 0).
	// Iff the mutex is write-locked, it contains rwMutexStateWLocked.
	// While the mutex is read-locked, it contains the current number of readers.
	state uint32
}

const (
	rwMutexStateUnlocked = uint32(0)
	rwMutexStateWLocked  = ^uint32(0)
	rwMutexMaxReaders    = rwMutexStateWLocked - 1
)

func (rw *RWMutex) Lock() {
	if rw.state == 0 {
		// The mutex is completely unlocked.
		// Lock without waiting.
		rw.state = rwMutexStateWLocked
		return
	}

	// Wait for the lock to be released.
	rw.waitingWriters.Push(task.Current())
	task.Pause()
}

func (rw *RWMutex) Unlock() {
	switch rw.state {
	case rwMutexStateWLocked:
		// This is correct.

	case rwMutexStateUnlocked:
		// The mutex is already unlocked.
		panic("sync: unlock of unlocked RWMutex")

	default:
		// The mutex is read-locked instead of write-locked.
		panic("sync: write-unlock of read-locked RWMutex")
	}

	switch {
	case rw.maybeUnblockReaders():
		// Switched over to read mode.

	case rw.maybeUnblockWriter():
		// Transferred to another writer.

	default:
		// Nothing is waiting for the lock.
		rw.state = rwMutexStateUnlocked
	}
}

func (rw *RWMutex) RLock() {
	if rw.state == rwMutexStateWLocked {
		// Wait for the write lock to be released.
		rw.waitingReaders.Push(task.Current())
		task.Pause()
		return
	}

	if rw.state == rwMutexMaxReaders {
		panic("sync: too many readers on RWMutex")
	}

	// Increase the reader count.
	rw.state++
}

func (rw *RWMutex) RUnlock() {
	switch rw.state {
	case rwMutexStateUnlocked:
		// The mutex is already unlocked.
		panic("sync: unlock of unlocked RWMutex")

	case rwMutexStateWLocked:
		// The mutex is write-locked instead of read-locked.
		panic("sync: read-unlock of write-locked RWMutex")
	}

	rw.state--

	if rw.state == rwMutexStateUnlocked {
		// This was the last reader.
		// Try to unblock a writer.
		rw.maybeUnblockWriter()
	}
}

func (rw *RWMutex) maybeUnblockReaders() bool {
	var n uint32
	for {
		t := rw.waitingReaders.Pop()
		if t == nil {
			break
		}

		n++
		scheduleTask(t)
	}
	if n == 0 {
		return false
	}

	rw.state = n
	return true
}

func (rw *RWMutex) maybeUnblockWriter() bool {
	t := rw.waitingWriters.Pop()
	if t == nil {
		return false
	}

	rw.state = rwMutexStateWLocked
	scheduleTask(t)

	return true
}

type Locker interface {
	Lock()
	Unlock()
}

// RLocker returns a Locker interface that implements
// the Lock and Unlock methods by calling rw.RLock and rw.RUnlock.
func (rw *RWMutex) RLocker() Locker {
	return (*rlocker)(rw)
}

type rlocker RWMutex

func (r *rlocker) Lock()   { (*RWMutex)(r).RLock() }
func (r *rlocker) Unlock() { (*RWMutex)(r).RUnlock() }

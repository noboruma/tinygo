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

//go:linkname runtimePanic runtime.runtimePanic
func runtimePanic(msg string)

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
	// Reader count, with the number of readers that currently have read-locked
	// this mutex.
	// The value can be in two states: one where 0 means no readers and another
	// where -rwMutexMaxReaders means no readers. A base of 0 is normal
	// uncontended operation, a base of -rwMutexMaxReaders means a writer has
	// the lock or is trying to get the lock. In the second case, readers should
	// wait until the reader count becomes non-negative again to give the writer
	// a chance to obtain the lock.
	readers task.Futex

	// Writer futex, normally 0. If there is a writer waiting until all readers
	// have unlocked, this value is 1. It will be changed to a 2 (and get a
	// wake) when the last reader unlocks.
	writer task.Futex

	// Writer lock. Held between Lock() and Unlock().
	writerLock Mutex
}

const rwMutexMaxReaders = 1 << 30

func (rw *RWMutex) Lock() {
	// Exclusive lock for writers.
	rw.writerLock.Lock()

	// Flag that we need to be awakened after the last read-lock unlocks.
	rw.writer.Store(1)

	// Signal to readers that they can't lock this mutex anymore.
	n := uint32(rwMutexMaxReaders)
	waiting := rw.readers.Add(-n)
	if int32(waiting) == -rwMutexMaxReaders {
		// All readers were already unlocked, so we don't need to wait for them.
		rw.writer.Store(0)
		return
	}

	// There is at least one reader.
	// Wait until all readers are unlocked. The last reader to unlock will set
	// rw.writer to 2 and awaken us.
	for rw.writer.Load() == 1 {
		rw.writer.Wait(1)
	}
	rw.writer.Store(0)
}

func (rw *RWMutex) Unlock() {
	// Signal that new readers can lock this mutex.
	waiting := rw.readers.Add(rwMutexMaxReaders)
	if waiting != 0 {
		// Awaken all waiting readers.
		rw.readers.WakeAll()
	}

	// Done with this lock (next writer can try to get a lock).
	rw.writerLock.Unlock()
}

func (rw *RWMutex) RLock() {
	// Add us as a reader.
	newVal := rw.readers.Add(1)

	// Wait until the RWMutex is available for readers.
	for int32(newVal) <= 0 {
		rw.readers.Wait(newVal)
		newVal = rw.readers.Load()
	}
}

func (rw *RWMutex) RUnlock() {
	// Remove us as a reader.
	one := uint32(1)
	readers := int32(rw.readers.Add(-one))

	// Check whether RUnlock was called too often.
	if readers == -1 || readers == (-rwMutexMaxReaders)-1 {
		runtimePanic("sync: RUnlock of unlocked RWMutex")
	}

	if readers == -rwMutexMaxReaders {
		// This was the last read lock. Check whether we need to wake up a write
		// lock.
		if rw.writer.CompareAndSwap(1, 2) {
			rw.writer.Wake()
		}
	}
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

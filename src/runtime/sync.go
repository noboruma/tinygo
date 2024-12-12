package runtime

import (
	"internal/futex"
	"sync/atomic"
)

// This file contains stub implementations for internal/poll.

//go:linkname semacquire internal/poll.runtime_Semacquire
func semacquire(sema *uint32) {
	var semaBlock futex.Futex
	semaBlock.Store(*sema)
	for {
		val := atomic.LoadUint32(sema)
		if val == 0 {
			semaBlock.Wait(val)
			continue
		}
		if atomic.CompareAndSwapUint32(sema, val, val-1) {
			break
		}
	}

}

//go:linkname semrelease internal/poll.runtime_Semrelease
func semrelease(sema *uint32) {
	var semaBlock futex.Futex
	semaBlock.Store(*sema)

	atomic.AddUint32(sema, 1)

	semaBlock.Wake()
}

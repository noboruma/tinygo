//go:build netpoll

package runtime

import (
	"unsafe"
)

type pollDesc struct {
	fd        int32
	unlockfd  int32
	rDeadline timespec
	wDeadline timespec
}

const (
	pollNoError        = 0 // no error
	pollErrClosing     = 1 // descriptor is closed
	pollErrTimeout     = 2 // I/O timeout
	pollErrNotPollable = 3 // general error polling descriptor

	pollModeRead  = 'r'
	pollModeWrite = 'w'

	pollIN  = 0x01
	pollOUT = 0x04
)

func after(t1, t2 timespec) bool {
	if t1.sec > t2.sec {
		return true
	} else if t1.sec == t2.sec {
		return t1.nsec > t2.nsec
	}
	return false
}

//go:linkname poll_runtime_pollReset internal/poll.runtime_pollReset
func poll_runtime_pollReset(pd *pollDesc, mode int) int {
	println("reset", pd)

	sec, nano, _ := now()
	res := timespec{
		tv_sec:  sec,
		tv_nsec: nano,
	}

	switch mode {
	case pollModeRead:
		if after(res, pd.rDeadline) {
			return pollErrTimeout
		}
	case pollModeWrite:
		if after(res, pd.wDeadline) {
			return pollErrTimeout
		}
	case pollModeWrite + pollModeRead:
		if after(res, pd.rDeadline) {
			return pollErrTimeout
		}
		if after(res, pd.wDeadline) {
			return pollErrTimeout
		}
	}
	return pollNoError
}

//go:linkname poll_runtime_pollWait internal/poll.runtime_pollWait
func poll_runtime_pollWait(pd *pollDesc, mode int) int {
	println("wait", pd)

	var res int32
	switch mode {
	case pollModeRead:
		tv := addToTimespec(pd.rDeadline)
		fds := [...]pollfd{
			pollfd{
				fd:      pd.fd,
				events:  pollIN,
				revents: 0,
			},
			//pollfd{
			//	fd:      pd.unlockfd,
			//	events:  pollIN,
			//	revents: 0,
			//},
		}
		res = ppoll(unsafe.Pointer(&fds), uint(len(fds)), unsafe.Pointer(&tv), nil)
	case pollModeWrite:
		var tv timespec
		addToTimespec(&tv, pd.wDeadline)
		fds := [...]pollfd{
			pollfd{
				fd:      pd.fd,
				events:  pollOUT,
				revents: 0,
			},
			//pollfd{
			//	fd:      pd.unlockfd,
			//	events:  pollOUT,
			//	revents: 0,
			//},
		}
		res = ppoll(unsafe.Pointer(&fds), uint(len(fds)), unsafe.Pointer(&tv), nil)
	default:
		println("should not happen")
	}
	if res == 0 {
		println("returns 0")
		return pollErrTimeout
	} else if res < 0 {
		return pollErrNotPollable
	}
	return pollNoError
}

//go:linkname poll_runtime_pollSetDeadline internal/poll.runtime_pollSetDeadline
func poll_runtime_pollSetDeadline(pd *pollDesc, d int64, mode int) {

	sec, nano, _ := now()
	res := timespec{
		tv_sec:  sec,
		tv_nsec: nano,
	}

	addToTimespec(&res, d)

	switch mode {
	case pollModeRead:
		pd.rDeadline = res
	case pollModeWrite:
		pd.wDeadline = res
	case pollModeRead + pollModeWrite:
		pd.rDeadline = res
		pd.wDeadline = res
	}
}

//go:linkname poll_runtime_pollOpen internal/poll.runtime_pollOpen
func poll_runtime_pollOpen(fd uintptr) (*pollDesc, int) {
	unlockfd := eventfd(0, 0)
	if unlockfd == -1 {
		println("FAILED")
		return nil, pollErrNotPollable
	}
	res := &pollDesc{
		fd:        int32(fd),
		unlockfd:  unlockfd,
		rDeadline: 0,
		wDeadline: 0,
	}
	println("open", res)
	return res, pollNoError
}

//go:linkname poll_runtime_pollServerInit internal/poll.runtime_pollServerInit
func poll_runtime_pollServerInit() {
	// This implementation is per thread, it does no rely on shared structure
}

//go:linkname poll_runtime_pollClose internal/poll.runtime_pollClose
func poll_runtime_pollClose(ctx uintptr) {
	println("close", ctx)
	pd := (*pollDesc)(unsafe.Pointer(ctx))

	if pd.fd != 0 {
		syscall_close(pd.fd)
		pd.fd = 0
	}

	if pd.unlockfd != 0 {
		syscall_close(pd.unlockfd)
		pd.unlockfd = 0
	}
}

//go:linkname poll_runtime_pollUnblock internal/poll.runtime_pollUnblock
func poll_runtime_pollUnblock(ctx uintptr) {
	println("unblock", ctx)
	pd := (*pollDesc)(unsafe.Pointer(ctx))
	if pd.unlockfd != 0 {
		syscall_close(pd.unlockfd)
		pd.unlockfd = eventfd(0, 0)
	}
}

func addToTimespec(tv *timespec, nanoSec int64) {
	tv.tv_nsec += nanoSec

	if tv.tv_nsec >= 1000000000 {
		tv.tv_sec += tv.tv_nsec / 1000000000 // Add whole seconds
		tv.tv_nsec = tv.tv_nsec % 1000000000 // Keep the remainder as nanoseconds
	}
}

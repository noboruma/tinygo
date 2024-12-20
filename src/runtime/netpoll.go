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
	if t1.tv_sec > t2.tv_sec {
		return true
	} else if t1.tv_sec == t2.tv_sec {
		return t1.tv_nsec > t2.tv_nsec
	}
	return false
}

//go:linkname poll_runtime_pollReset internal/poll.runtime_pollReset
func poll_runtime_pollReset(pd *pollDesc, mode int) int {
	println("reset", pd)

	switch mode {
	case pollModeRead:
	case pollModeWrite:
	case pollModeWrite + pollModeRead:
	}
	return pollNoError
}

//go:linkname poll_runtime_pollWait internal/poll.runtime_pollWait
func poll_runtime_pollWait(pd *pollDesc, mode int) int {

	sec, nano, _ := now()
	now := timespec{
		tv_sec:  sec,
		tv_nsec: int64(nano),
	}

	var res int32
	switch mode {
	case pollModeRead:
		println("rwait", pd.fd, pd.rDeadline.tv_sec-now.tv_sec)
		if after(now, pd.rDeadline) {
			return pollErrTimeout
		}
		fds := []pollfd{
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
		tv := timespec{
			tv_sec: pd.rDeadline.tv_sec - now.tv_sec,
		}
		res = ppoll(unsafe.Pointer(&fds), uint(len(fds)), unsafe.Pointer(&tv), nil)
		KeepAlive(fds)
		KeepAlive(tv)
	case pollModeWrite:
		println("wwait", pd.fd, pd.wDeadline.tv_sec-now.tv_sec)
		if after(now, pd.wDeadline) {
			return pollErrTimeout
		}
		fds := []pollfd{
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
		tv := timespec{
			tv_sec: pd.wDeadline.tv_sec - now.tv_sec,
		}
		res = ppoll(unsafe.Pointer(&fds), uint(len(fds)), unsafe.Pointer(&tv), nil)
		KeepAlive(fds)
		KeepAlive(tv)
	default:
		println("should not happen")
	}
	if res == 0 {
		println("returns 0")
		return pollErrTimeout
	} else if res < 0 {
		return pollErrNotPollable
	}
	println("ready")
	return pollNoError
}

//go:linkname poll_runtime_pollSetDeadline internal/poll.runtime_pollSetDeadline
func poll_runtime_pollSetDeadline(pd *pollDesc, d int64, mode int) {

	deadline := timespec{
		tv_sec:  0,
		tv_nsec: 0,
	}

	addToTimespec(&deadline, d)

	switch mode {
	case pollModeRead:
		pd.rDeadline = deadline
	case pollModeWrite:
		pd.wDeadline = deadline
	case pollModeRead + pollModeWrite:
		pd.rDeadline = deadline
		pd.wDeadline = deadline
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
		fd:       int32(fd),
		unlockfd: unlockfd,
		rDeadline: timespec{
			tv_sec:  1<<31 - 1,
			tv_nsec: 1<<31 - 1,
		},
		wDeadline: timespec{
			tv_sec:  1<<31 - 1,
			tv_nsec: 1<<31 - 1,
		},
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

	if pd.unlockfd != 0 {
		syscall_close(pd.unlockfd)
		pd.unlockfd = eventfd(0, 0)
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
		tv.tv_sec += tv.tv_nsec / 1000000000
		tv.tv_nsec = tv.tv_nsec % 1000000000
	}
}

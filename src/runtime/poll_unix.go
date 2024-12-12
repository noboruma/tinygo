//go:build linux

package runtime

import "unsafe"

type pollDesc struct {
	rfd       pollfd
	wfd       pollfd
	rDeadline int64
	wDeadline int64
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

//go:linkname poll_runtime_pollReset internal/poll.runtime_pollReset
func poll_runtime_pollReset(pd *pollDesc, mode int) int {
	switch mode {
	case pollModeRead:
		pd.rfd.events = pollIN
		pd.rfd.revents = 0
	case pollModeWrite:
		pd.wfd.events = pollOUT
		pd.wfd.revents = 0
	default:
		println("todo reset")
	}
	println("reset")
	return pollNoError
}

//go:linkname poll_runtime_pollWait internal/poll.runtime_pollWait
func poll_runtime_pollWait(pd *pollDesc, mode int) int {
	switch mode {
	case pollModeRead:
		println("wait read")
		tv := timespec{
			tv_sec:  pd.rDeadline,
			tv_nsec: 0,
		}
		res := ppoll(unsafe.Pointer(&pd.rfd), 1, unsafe.Pointer(&tv), nil)
		println("go read!", res)
	case pollModeWrite:
		println("wait write")
		tv := timespec{
			tv_sec:  pd.wDeadline,
			tv_nsec: 0,
		}
		res := ppoll(unsafe.Pointer(&pd.wfd), 1, unsafe.Pointer(&tv), nil)
		println("go write!", res)
	default:
		println("todo wait")
	}
	return pollNoError
}

//go:linkname poll_runtime_pollSetDeadline internal/poll.runtime_pollSetDeadline
func poll_runtime_pollSetDeadline(pd *pollDesc, d int64, mode int) {
	switch mode {
	case pollModeRead:
		pd.rDeadline = d
	case pollModeWrite:
		pd.wDeadline = d
	}
}

//go:linkname poll_runtime_pollOpen internal/poll.runtime_pollOpen
func poll_runtime_pollOpen(fd uintptr) (*pollDesc, int) {
	res := &pollDesc{
		rfd: pollfd{
			fd:      int32(fd),
			events:  pollIN,
			revents: 0,
		},
		wfd: pollfd{
			fd:      int32(fd),
			events:  pollOUT,
			revents: 0,
		},
		rDeadline: 30,
		wDeadline: 30,
	}
	println("open", res)
	return res, pollNoError
}

//go:linkname poll_runtime_pollServerInit internal/poll.runtime_pollServerInit
func poll_runtime_pollServerInit() {
	// This implementation is per thread
}

//go:linkname poll_runtime_pollClose internal/poll.runtime_pollClose
func poll_runtime_pollClose(ctx uintptr) {
	println("close", ctx)
	// This implementation is per thread
}

//go:linkname poll_runtime_pollUnblock internal/poll.runtime_pollUnblock
func poll_runtime_pollUnblock(ctx uintptr) {
	println("unblock", ctx)
}

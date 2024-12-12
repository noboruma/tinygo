package runtime

const GOARCH = "arm64"

// The bitness of the CPU (e.g. 8, 32, 64).
const TargetBits = 64

const deferExtraRegs = 0

const callInstSize = 4 // "bl someFunction" is 4 bytes

const (
	linux_MAP_ANONYMOUS = 0x20
	linux_SIGBUS        = 7
	linux_SIGILL        = 4
	linux_SIGSEGV       = 11
)

// Align on word boundary.
func align(ptr uintptr) uintptr {
	return (ptr + 15) &^ 15
}

func getCurrentStackPointer() uintptr {
	return uintptr(stacksave())
}

//export low_isar0
func low_isar0() uint64

//go:linkname getisar0 vendor/golang.org/x/sys/cpu.getisar0
func getisar0() uint64 {
	return low_isar0()
}

//export low_isar1
func low_isar1() uint64

//go:linkname getisar1 vendor/golang.org/x/sys/cpu.getisar1
func getisar1() uint64 {
	return low_isar1()
}

//export low_pfr0
func low_pfr0() uint64

//go:linkname getpfr0 vendor/golang.org/x/sys/cpu.getpfr0
func getpfr0() uint64 {
	return low_pfr0()
}

//export low_zfr0
func low_zfr0() uint64

//go:linkname getzfr0 vendor/golang.org/x/sys/cpu.getzfr0
func getzfr0() uint64 {
	return low_zfr0()
}

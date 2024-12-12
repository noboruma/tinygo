//go:build i386 || amd64

package runtime

import "unsafe"

//export low_cpuid
func low_cpuid(a, b, c, d unsafe.Pointer)

//go:linkname cpuid vendor/golang.org/x/sys/cpu.cpuid
func cpuid(eax uint32) (ebx, ecx, edx uint32) {
	var (
		_ebx, _ecx, _edx uint32
	)
	low_cpuid(unsafe.Pointer(&eax),
		unsafe.Pointer(&_ebx),
		unsafe.Pointer(&_ecx),
		unsafe.Pointer(&_edx))

	return _ebx, _ecx, _edx
}

//export low_cpuid
func low_xgetbv(a, b, c, d unsafe.Pointer)

//go:linkname xgetbv vendor/golang.org/x/sys/cpu.xgetbv
func xgetbv(eax uint32) (ebx, ecx, edx uint32) {
	var (
		_ebx, _ecx, _edx uint32
	)
	low_cpuid(unsafe.Pointer(&eax),
		unsafe.Pointer(&_ebx),
		unsafe.Pointer(&_ecx),
		unsafe.Pointer(&_edx))

	return _ebx, _ecx, _edx
}

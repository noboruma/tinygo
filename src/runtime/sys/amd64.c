#include <stdint.h>
void low_cpuid(uint32_t *eax, uint32_t *ebx, uint32_t *ecx, uint32_t *edx) {
    __asm__ volatile(
        "cpuid"
        : "=a" (*eax), "=b" (*ebx), "=c" (*ecx), "=d" (*edx)
        : "a" (*eax)
    );
}

unsigned long long low_xgetbv(unsigned int index) {
    unsigned int low, high;
    __asm__ (
        "xgetbv"
        : "=a" (low), "=d" (high)
        : "c" (index)
    );
    return ((unsigned long long)high << 32) | low;
}

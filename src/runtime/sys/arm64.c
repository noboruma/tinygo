#include <stdint.h>

unsigned long low_isar0() {
	unsigned long result = 0;
	asm volatile("mrs %0, ID_AA64ISAR0_EL1" : "=r" (result));
	return result;
}

unsigned long low_isar1() {
	unsigned long result = 0;
	asm volatile("mrs %0, ID_AA64ISAR1_EL1" : "=r" (result));
	return result;
}

unsigned long low_pfr0() {
	unsigned long result = 0;
	asm volatile("mrs %0, ID_AA64PFR0_EL1" : "=r" (result));
	return result;
}

unsigned long low_zfr0() {
	unsigned long result = 0;
	asm volatile("mrs %0, ID_AA64ZFR0_EL1" : "=r" (result));
	return result;
}

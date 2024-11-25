//go:build tkey && !qemu

package runtime

// ticksToNanoseconds converts ticks (at 18MHz) to nanoseconds.
func ticksToNanoseconds(ticks timeUnit) int64 {
	return int64(ticks) * 1800
}

// nanosecondsToTicks converts nanoseconds to ticks (at 18MHz).
func nanosecondsToTicks(ns int64) timeUnit {
	return timeUnit(ns / 1800)
}

func exit(code int) {
	abort()
}

func abort() {
	// lock up forever
	for {
		// TODO: something here?
	}
}

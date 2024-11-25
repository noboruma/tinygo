//go:build tkey

// This file implements target-specific things for the TKey.

package runtime

import (
	"machine"
	"runtime/volatile"
)

type timeUnit int64

//export main
func main() {
	preinit()
	initPeripherals()
	run()
	exit(0)
}

// initPeripherals configures peripherals the way the runtime expects them.
func initPeripherals() {
	machine.InitSerial()
}

func putchar(c byte) {
	machine.Serial.WriteByte(c)
}

func getchar() byte {
	for machine.Serial.Buffered() == 0 {
		Gosched()
	}
	v, _ := machine.Serial.ReadByte()
	return v
}

func buffered() int {
	return machine.Serial.Buffered()
}

var timestamp volatile.Register32

// ticks returns the current value of the timer in ticks.
func ticks() timeUnit {
	return timeUnit(timestamp.Get())
}

// sleepTicks sleeps for at least the duration d.
func sleepTicks(d timeUnit) {
	target := uint64(ticks() + d)

	for {
		if uint64(ticks()) >= target {
			break
		}
		timestamp.Set(timestamp.Get() + 1)
	}
}

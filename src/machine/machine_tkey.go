//go:build tkey

package machine

import (
	"device/tkey"
)

const deviceName = "TKey"

// GPIO pins modes are only here to match the Pin interface.
// The actual configuration is fixed in the hardware.
const (
	PinOutput PinMode = iota
	PinInput
	PinInputPullup
	PinInputPulldown
)

const (
	LED_BLUE  = Pin(tkey.TK1_MMIO_TK1_LED_B_BIT)
	LED_GREEN = Pin(tkey.TK1_MMIO_TK1_LED_G_BIT)
	LED_RED   = Pin(tkey.TK1_MMIO_TK1_LED_R_BIT)

	LED = LED_GREEN

	TKEY_TOUCH = Pin(3) // 3 is unused, but we need a value here to match the Pin interface.
	BUTTON     = TKEY_TOUCH
)

var buttonConfig PinConfig

// No config needed for TKey, just to match the Pin interface.
func (p Pin) Configure(config PinConfig) {
	switch p {
	case BUTTON:
		buttonConfig = config

		// Clear any pending touch events.
		tkey.TOUCH.STATUS.Set(0)
	}
}

// Set GPIO pin to high or low.
func (p Pin) Set(high bool) {
	switch p {
	case LED_BLUE, LED_GREEN, LED_RED:
		if high {
			tkey.TK1.LED.SetBits(1 << uint(p))
		} else {
			tkey.TK1.LED.ClearBits(1 << uint(p))
		}
	}
}

func (p Pin) Get() bool {
	switch p {
	case BUTTON:
		pushed := false
		if tkey.TOUCH.STATUS.HasBits(1) {
			tkey.TOUCH.STATUS.Set(0)
			pushed = true
		}

		switch buttonConfig.Mode {
		case PinInputPullup:
			return !pushed
		case PinInput, PinInputPulldown:
			return pushed
		}
	}

	return false
}

type UART struct {
	Bus *tkey.UART_Type
}

var (
	DefaultUART = UART0
	UART0       = &_UART0
	_UART0      = UART{Bus: tkey.UART}
)

// Thw TKey UART is fixed at 62500 baud, 8N1.
func (uart *UART) Configure(config UARTConfig) {
}

func (uart *UART) SetBaudRate(br uint32) {
}

func (uart *UART) Write(data []byte) (n int, err error) {
	for _, c := range data {
		if err := uart.WriteByte(c); err != nil {
			return n, err
		}
	}
	return len(data), nil
}

func (uart *UART) WriteByte(c byte) error {
	for uart.Bus.TX_STATUS.Get() == 0 {
	}

	uart.Bus.TX_DATA.Set(uint32(c))

	return nil
}

func (uart *UART) Buffered() int {
	return int(uart.Bus.RX_BYTES.Get())
}

func (uart *UART) ReadByte() (byte, error) {
	for uart.Bus.RX_STATUS.Get() == 0 {
	}

	return byte(uart.Bus.RX_DATA.Get()), nil
}

// GetRNG returns 32 bits of cryptographically secure random data
func GetRNG() (uint32, error) {
	for tkey.TRNG.STATUS.Get() == 0 {
	}

	return uint32(tkey.TRNG.ENTROPY.Get()), nil
}

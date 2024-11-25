//go:build tkey

package machine

import (
	"device/tkey"
)

const deviceName = "TKey"

const (
	PinOutput PinMode = iota
)

var (
	LED_BLUE  = Pin(tkey.TK1_MMIO_TK1_LED_B_BIT)
	LED_GREEN = Pin(tkey.TK1_MMIO_TK1_LED_G_BIT)
	LED_RED   = Pin(tkey.TK1_MMIO_TK1_LED_R_BIT)

	LED = LED_GREEN
)

// No config needed for TKey, just to match the Pin interface.
func (p Pin) Configure(config PinConfig) {
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

type UART struct {
	Bus    *tkey.UART_Type
	Buffer *RingBuffer
}

var (
	UART0  = &_UART0
	_UART0 = UART{Bus: tkey.UART, Buffer: NewRingBuffer()}
)

func (uart *UART) Configure(config UARTConfig) {
	if config.BaudRate == 0 {
		config.BaudRate = 115200
	}

	uart.SetBaudRate(config.BaudRate)
}

func (uart *UART) SetBaudRate(br uint32) {
	uart.Bus.BIT_RATE.Set(uint16(18e6 / br))
}

func (uart *UART) writeByte(c byte) error {
	for uart.Bus.TX_STATUS.Get() == 0 {
	}

	uart.Bus.TX_DATA.Set(uint16(c))

	return nil
}

func (uart *UART) flush() {}

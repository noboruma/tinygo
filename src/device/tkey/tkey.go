//go:build tkey

// Hand written file based on https://github.com/tillitis/tkey-libs/blob/main/include/tkey/tk1_mem.h

package tkey

import (
	"runtime/volatile"
	"unsafe"
)

// Peripherals
var (
	TRNG = (*TRNG_Type)(unsafe.Pointer(TK1_MMIO_TRNG_BASE))

	TIMER = (*TIMER_Type)(unsafe.Pointer(TK1_MMIO_TIMER_BASE))

	UDS = (*UDS_Type)(unsafe.Pointer(TK1_MMIO_UDS_BASE))

	UART = (*UART_Type)(unsafe.Pointer(TK1_MMIO_UART_BASE))

	TOUCH = (*TOUCH_Type)(unsafe.Pointer(TK1_MMIO_TOUCH_BASE))

	TK1 = (*TK1_Type)(unsafe.Pointer(TK1_MMIO_TK1_BASE))
)

// Memory sections
const (
	TK1_ROM_BASE uintptr = 0x00000000

	TK1_RAM_BASE uintptr = 0x40000000

	TK1_MMIO_BASE uintptr = 0xc0000000

	TK1_MMIO_TRNG_BASE uintptr = 0xc0000000

	TK1_MMIO_TIMER_BASE uintptr = 0xc1000000

	TK1_MMIO_UDS_BASE uintptr = 0xc2000000

	TK1_MMIO_UART_BASE uintptr = 0xc3000000

	TK1_MMIO_TOUCH_BASE uintptr = 0xc4000000

	TK1_MMIO_FW_RAM_BASE uintptr = 0xd0000000

	TK1_MMIO_TK1_BASE uintptr = 0xff000000
)

// Memory section sizes
const (
	TK1_RAM_SIZE uintptr = 0x20000

	TK1_MMIO_SIZE uintptr = 0x3fffffff
)

type TRNG_Type struct {
	_       [36]byte
	STATUS  volatile.Register16
	_       [108]byte
	ENTROPY volatile.Register16
}

type TIMER_Type struct {
	_         [32]byte
	CTRL      volatile.Register16
	_         [2]byte
	STATUS    volatile.Register16
	_         [2]byte
	PRESCALER volatile.Register32
	TIMER     volatile.Register32
}

type UDS_Type struct {
	DATA [8]volatile.Register16
}

type UART_Type struct {
	_         [40]byte
	BIT_RATE  volatile.Register16
	_         [2]byte
	DATA_BITS volatile.Register16
	_         [2]byte
	STOP_BITS volatile.Register16
	_         [58]byte
	RX_STATUS volatile.Register16
	_         [2]byte
	RX_DATA   volatile.Register16
	_         [2]byte
	RX_BYTES  volatile.Register16
	_         [16]byte
	TX_STATUS volatile.Register16
	_         [2]byte
	TX_DATA   volatile.Register16
}

type TOUCH_Type struct {
	_      [36]byte
	STATUS volatile.Register16
}

type TK1_Type struct {
	NAME0         [4]volatile.Register8
	NAME1         [4]volatile.Register8
	VERSION       [4]volatile.Register8
	_             [16]byte
	SWITCH_APP    volatile.Register32
	_             [4]byte
	LED           volatile.Register32
	GPIO          volatile.Register16
	APP_ADDR      volatile.Register32
	APP_SIZE      volatile.Register32
	BLAKE2S       volatile.Register32
	_             [56]byte
	CDI_FIRST     [8]volatile.Register16
	_             [38]byte
	UDI_FIRST     [2]volatile.Register16
	_             [62]byte
	RAM_ADDR_RAND volatile.Register16
	_             [2]byte
	RAM_DATA_RAND volatile.Register16
	_             [126]byte
	CPU_MON_CTRL  volatile.Register16
	_             [2]byte
	CPU_MON_FIRST volatile.Register32
	CPU_MON_LAST  volatile.Register32
	_             [60]byte
	SYSTEM_RESET  volatile.Register16
	_             [66]byte
	SPI_EN        volatile.Register16
	_             [2]byte
	SPI_XFER      volatile.Register16
	_             [2]byte
	SPI_DATA      volatile.Register16
}

const (
	TK1_MMIO_TIMER_CTRL_START_BIT = 0
	TK1_MMIO_TIMER_CTRL_STOP_BIT  = 1
	TK1_MMIO_TIMER_CTRL_START     = 1 << TK1_MMIO_TIMER_CTRL_START_BIT
	TK1_MMIO_TIMER_CTRL_STOP      = 1 << TK1_MMIO_TIMER_CTRL_STOP_BIT

	TK1_MMIO_TK1_LED_R_BIT = 2
	TK1_MMIO_TK1_LED_G_BIT = 1
	TK1_MMIO_TK1_LED_B_BIT = 0
)

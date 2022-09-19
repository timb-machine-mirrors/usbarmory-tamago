// NXP i.MX6UL initialization
// https://github.com/usbarmory/tamago
//
// Copyright (c) WithSecure Corporation
// https://foundry.withsecure.com
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package imx6ul

import (
	_ "unsafe"

	"github.com/usbarmory/tamago/arm"
	"github.com/usbarmory/tamago/dma"
	"github.com/usbarmory/tamago/internal/reg"
	"github.com/usbarmory/tamago/soc/nxp/usb"
)

// i.MX processor families
const (
	IMX6UL  = 0x64
	IMX6ULL = 0x65
)

//go:linkname ramStackOffset runtime.ramStackOffset
var ramStackOffset uint32 = 0x100

var (
	// Processor family
	Family uint32

	// Flag native or emulated processor
	Native bool

	// SDP flags whether Serial Download Protocol over USB has been used to
	// boot this runtime. The value is always false on non-secure (e.g.
	// TrustZone Normal World) processor modes.
	SDP bool
)

// Init takes care of the lower level SoC initialization triggered early in
// runtime setup (e.g. runtime.hwinit).
func Init() {
	if ARM.Mode() != arm.SYS_MODE {
		// initialization required only when in PL1
		return
	}

	ARM.Init()
	ARM.EnableVFP()

	// required when booting in SDP mode
	ARM.EnableSMP()

	// MMU initialization is required to take advantage of data cache
	ARM.InitMMU()
	ARM.EnableCache()

	_, fam, revMajor, revMinor := SiliconVersion()
	Family = fam

	if revMajor != 0 || revMinor != 0 {
		Native = true
	}

	initTimers()
}

func init() {
	// clear power-down watchdog
	clearWDOG()

	// use internal OCRAM (iRAM) as default DMA region
	dma.Init(OCRAM_START, OCRAM_SIZE)

	// assign internal OCRAM to DCP internal key exchange
	DCP.DeriveKeyMemory = dma.Default()

	switch Model() {
	case "i.MX6UL":
		OCOTP.Banks = 16
	case "i.MX6ULL":
		OCOTP.Banks = 8
	}

	if !Native || ARM.NonSecure() {
		return
	}

	// On the i.MX6UL family the only way to detect if we are booting
	// through Serial Download Mode over USB is to check whether the USB
	// OTG1 controller was running in device mode prior to our own
	// initialization.
	if reg.Get(USB1_BASE+usb.USB_UOGx_USBMODE, usb.USBMODE_CM, 0b11) == usb.USBMODE_CM_DEVICE &&
		reg.Get(USB1_BASE+usb.USB_UOGx_USBCMD, usb.USBCMD_RS, 1) != 0 {
		SDP = true
	}
}
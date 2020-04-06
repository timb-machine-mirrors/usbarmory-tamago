// USB armory Mk II support for tamago/arm
// https://github.com/f-secure-foundry/tamago
//
// Copyright (c) F-Secure Corporation
// https://foundry.f-secure.com
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.
//
// +build tamago,arm

package usbarmory

import (
	"errors"

	"github.com/f-secure-foundry/tamago/imx6"
)

// On the USB armory Mk II
// the white LED is connected to CSI_DATA00 pad and GPIO4_IO21,
// the blue  LED is connected to CSI_DATA01 pad and GPIO4_IO22.

const (
	// GPIO4 data
	GPIO4_DR uint32 = 0x020a8000
	// GPIO4 direction
	GPIO4_GDIR uint32 = 0x020a8004

	// GPIO number
	WHITE = 21
	// mux control
	IOMUXC_SW_MUX_CTL_PAD_CSI_DATA00 uint32 = 0x020e01e4
	// pad control
	IOMUXC_SW_PAD_CTL_PAD_CSI_DATA00 uint32 = 0x020e0470

	// GPIO number
	BLUE = 22
	// mux control
	IOMUXC_SW_MUX_CTL_PAD_CSI_DATA01 uint32 = 0x020e01e8
	// pad control
	IOMUXC_SW_PAD_CTL_PAD_CSI_DATA01 uint32 = 0x020e0474
)

var white *imx6.GPIO
var blue *imx6.GPIO

func configurePad(gpio *imx6.GPIO) {
	gpio.PadCtl((1 << imx6.SW_PAD_CTL_PKE) |
		(imx6.SW_PAD_CTL_SPEED_100MHZ << imx6.SW_PAD_CTL_SPEED) |
		(imx6.SW_PAD_CTL_DSE_2_R0_6 << imx6.SW_PAD_CTL_DSE))
	gpio.Out()
}

func init() {
	var err error

	white, err = imx6.NewGPIO(WHITE,
		IOMUXC_SW_MUX_CTL_PAD_CSI_DATA00, IOMUXC_SW_PAD_CTL_PAD_CSI_DATA00,
		GPIO4_DR, GPIO4_GDIR)

	if err != nil {
		panic(err)
	}

	blue, err = imx6.NewGPIO(BLUE,
		IOMUXC_SW_MUX_CTL_PAD_CSI_DATA01, IOMUXC_SW_PAD_CTL_PAD_CSI_DATA01,
		GPIO4_DR, GPIO4_GDIR)

	if err != nil {
		panic(err)
	}

	if !imx6.Native {
		return
	}

	configurePad(white)
	configurePad(blue)
}

// LED turns on/off an LED by name.
func LED(name string, on bool) (err error) {
	var led *imx6.GPIO

	switch name {
	case "white", "White", "WHITE":
		led = white
	case "blue", "Blue", "BLUE":
		led = blue
	default:
		return errors.New("invalid LED")
	}

	if !imx6.Native {
		return
	}

	if on {
		led.Low()
	} else {
		led.High()
	}

	return
}
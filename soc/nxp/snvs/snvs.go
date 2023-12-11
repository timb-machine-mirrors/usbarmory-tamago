// NXP Secure Non-Volatile Storage (SNVS) support
// https://github.com/usbarmory/tamago
//
// Copyright (c) WithSecure Corporation
// https://foundry.withsecure.com
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

// Package snvs implements a driver for NXP Secure Non-Volatile Storage (SNVS)
// following reference specifications:
//   - IMX6ULLRM  - i.MX 6ULL Applications Processor Reference Manual          - Rev 1 2017/11
//   - IMX6ULLSRM - i.MX 6ULL Applications Processor Security Reference Manual - Rev 0 2016/09
//
// This package is only meant to be used with `GOOS=tamago GOARCH=arm` as
// supported by the TamaGo framework for bare metal Go on ARM SoCs, see
// https://github.com/usbarmory/tamago.
package snvs

import (
	"github.com/usbarmory/tamago/bits"
	"github.com/usbarmory/tamago/internal/reg"

	"sync"
	"time"
)

// SNVS registers
const (
	SNVS_HPCOMR     = 0x04
	HPCOMR_HAC_STOP = 19
	HPCOMR_HAC_LOAD = 17
	HPCOMR_HAC_EN   = 16

	SNVS_HPSVCR     = 0x10
	HPSVCR_LPSV_CFG = 30

	SNVS_HPSR           = 0x14
	HPSR_OTPMK_ZERO     = 27
	HPSR_OTPMK_SYNDROME = 16

	SNVS_HPHACIVR = 0x1c
	SNVS_HPHACR   = 0x20

	HPSR_SSM_STATE    = 8
	SSM_STATE_TRUSTED = 0b1101
	SSM_STATE_SECURE  = 0b1111

	SNVS_LPTDCR  = 0x48
	LPTDCR_VT_EN = 6
	LPTDCR_TT_EN = 5
	LPTDCR_CT_EN = 4

	SNVS_LPSR = 0x4c
	LPSR_VTD  = 6
	LPSR_TTD  = 5
	LPSR_CTD  = 4
	LPSR_PGD  = 3

	SNVS_LPPGDR = 0x64
	// Power Glitch Detector Register hardwired value
	LPPGDR_PGD_VAL = 0x41736166
)

// DryIce registers
const (
	DRYICE_DTOCR    = 0x00
	DTOCR_DRYICE_EN = 0

	DRYICE_DTRR = 0x08
	DTRR_CTD    = 2
	DTRR_VTD    = 1
	DTRR_TTD    = 0

	DRYICE_DMCR = 0x0c
	DMCR_CT_EN  = 2
	DMCR_VT_EN  = 1
	DMCR_TT_EN  = 0
)

// SNVS represents the SNVS instance.
type SNVS struct {
	sync.Mutex

	// Base register
	Base uint32
	// Clock gate register
	CCGR uint32
	// Clock gate
	CG int
	// auxiliary logic base register
	DryIce uint32

	// control registers
	hpcomr   uint32
	hpsvcr   uint32
	hpsr     uint32
	hphacivr uint32
	hphacr   uint32
	lptdcr   uint32
	lpsr     uint32
	lppgdr   uint32

	// DryIce registers
	dtocr uint32
	dtrr  uint32
	dmcr  uint32

	// active configuration
	sp SecurityPolicy
}

// SecurityPolicy represents an SNVS configuration and is used to configure
// tamper detection or return detected violations (see SetPolicy()).
type SecurityPolicy struct {
	// SRTC Clock Tampering
	Clock bool
	// Temperature Tamper
	Temperature bool
	// Voltage Tamper
	Voltage bool
	// Power Glitch Violation (used only when reporting, see Monitor())
	Power bool

	// SecurityViolation controls whether monitored conditions generate a
	// violation which transitions the SNVS state to soft fail, preventing
	// access to the OTPMK and SNVS availability (see Available()).
	SecurityViolation bool

	// HardFail controls whether a soft fail state (see SecurityViolation)
	// transitions the system to a hard reset after a predefined system
	// clock delay (see HAC).
	HardFail bool

	// HAC is used to stop and reset the initial value of the High
	// Assurance Counter, a delay in system clocks between a soft fail and
	// a hard fail, or return the current HAC value.
	HAC uint32
}

// Init initializes the SNVS controller.
func (hw *SNVS) Init() {
	if hw.Base == 0 || hw.CCGR == 0 {
		panic("invalid SNVS instance")
	}

	// enable clock
	reg.SetN(hw.CCGR, hw.CG, 0b11, 0b11)

	hw.hpcomr = hw.Base + SNVS_HPCOMR
	hw.hpsvcr = hw.Base + SNVS_HPSVCR
	hw.hpsr = hw.Base + SNVS_HPSR
	hw.hphacivr = hw.Base + SNVS_HPHACIVR
	hw.hphacr = hw.Base + SNVS_HPHACR
	hw.lptdcr = hw.Base + SNVS_LPTDCR
	hw.lpsr = hw.Base + SNVS_LPSR
	hw.lppgdr = hw.Base + SNVS_LPPGDR

	if hw.DryIce > 0 {
		hw.dtocr = hw.DryIce + DRYICE_DTOCR
		hw.dtrr = hw.DryIce + DRYICE_DTRR
		hw.dmcr = hw.DryIce + DRYICE_DMCR
	}
}

// SetPolicy configures the SNVS tamper detection and security violation
// policy, It can be used to prevent a transition from soft fail to hard fail
// if invoked within the expiration of a previously applied policy (see
// SecurityPolicy.HAC).
func (hw *SNVS) SetPolicy(sp SecurityPolicy) {
	hw.Lock()
	defer hw.Unlock()

	// stop High Assurance Counter
	reg.Set(hw.hpcomr, HPCOMR_HAC_STOP)
	reg.Clear(hw.hpcomr, HPCOMR_HAC_EN)

	// set Power Glitch Detector value and clear its record
	reg.Write(hw.lppgdr, LPPGDR_PGD_VAL)
	reg.Set(hw.lpsr, LPSR_PGD)

	// set LP security violation configuration
	reg.SetTo(hw.hpsvcr, HPSVCR_LPSV_CFG+1, sp.SecurityViolation)

	if sp.HardFail {
		reg.Clear(hw.hpcomr, HPCOMR_HAC_STOP)
		reg.Write(hw.hphacivr, sp.HAC)
		reg.Set(hw.hpcomr, HPCOMR_HAC_LOAD)
		reg.Set(hw.hpcomr, HPCOMR_HAC_EN)
	}

	if hw.DryIce > 0 {
		reg.Set(hw.dtocr, DTOCR_DRYICE_EN)
		time.Sleep(1 * time.Millisecond)

		reg.SetTo(hw.dmcr, DMCR_VT_EN, sp.Voltage)
		reg.SetTo(hw.dmcr, DMCR_TT_EN, sp.Temperature)
		reg.SetTo(hw.dmcr, DMCR_CT_EN, sp.Clock)
		time.Sleep(1 * time.Millisecond)

		// clear records
		reg.Set(hw.dtrr, DTRR_VTD)
		reg.Set(hw.dtrr, DTRR_TTD)
		reg.Set(hw.dtrr, DTRR_CTD)
	}

	// set tamper monitors
	reg.SetTo(hw.lptdcr, LPTDCR_VT_EN, sp.Voltage)
	reg.SetTo(hw.lptdcr, LPTDCR_TT_EN, sp.Temperature)
	reg.SetTo(hw.lptdcr, LPTDCR_CT_EN, sp.Clock)

	// clear records
	reg.Set(hw.lpsr, LPSR_VTD)
	reg.Set(hw.lpsr, LPSR_TTD)
	reg.Set(hw.lpsr, LPSR_CTD)

	hw.sp = sp
}

// Monitor returns the SNVS tamper detection status, configured security
// violation policy and the current High Assurance Counter value.
func (hw *SNVS) Monitor() (violations SecurityPolicy) {
	clk := reg.IsSet(hw.lpsr, LPSR_CTD)
	tmp := reg.IsSet(hw.lpsr, LPSR_TTD)
	vcc := reg.IsSet(hw.lpsr, LPSR_VTD)

	if hw.DryIce > 0 {
		clk = clk || reg.IsSet(hw.dtrr, DTRR_CTD)
		tmp = tmp || reg.IsSet(hw.dtrr, DTRR_TTD)
		vcc = vcc || reg.IsSet(hw.dtrr, DTRR_VTD)
	}

	return SecurityPolicy{
		Clock:             clk,
		Temperature:       tmp,
		Voltage:           vcc,
		Power:             reg.IsSet(hw.lpsr, LPSR_PGD),
		SecurityViolation: hw.sp.SecurityViolation,
		HardFail:          hw.sp.HardFail,
		HAC:               reg.Read(hw.hphacr),
	}
}

// Available verifies whether the Secure Non Volatile Storage (SNVS) is
// correctly programmed and in Trusted or Secure state (indicating that Secure
// Boot is enabled and no security violations have been detected).
//
// The unique OTPMK internal key is available only when Secure Boot (HAB) is
// enabled, otherwise a Non-volatile Test Key (NVTK), identical for each SoC,
// is used.
func (hw *SNVS) Available() bool {
	hpsr := reg.Read(hw.hpsr)

	// ensure that the OTPMK has been correctly programmed
	if bits.Get(&hpsr, HPSR_OTPMK_ZERO, 1) != 0 || bits.Get(&hpsr, HPSR_OTPMK_SYNDROME, 0x1ff) != 0 {
		return false
	}

	switch bits.Get(&hpsr, HPSR_SSM_STATE, 0b1111) {
	case SSM_STATE_TRUSTED, SSM_STATE_SECURE:
		return true
	default:
		return false
	}
}

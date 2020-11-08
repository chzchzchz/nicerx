package radio

import (
	"encoding/binary"
	"fmt"
	"net"
)

var dongleMagic = [...]byte{'R', 'T', 'L', '0'}

// RTLTCPSDR contains dongle information and an embedded tcp connection to the spectrum server.
type RTLTCPSDR struct {
	*net.TCPConn
	Info DongleInfo
}

// Give an address of the form "127.0.0.1:1234" connects to the spectrum
// server at the given address or returns an error. The user is responsible
// for closing this connection. If addr is nil, use "127.0.0.1:1234" or
// command line flag value.
func (sdr *RTLTCPSDR) Connect(addr *net.TCPAddr) (err error) {
	if sdr.TCPConn, err = net.DialTCP("tcp", nil, addr); err != nil {
		return fmt.Errorf("error connecting to spectrum server: %v", err)
	}
	defer func() {
		if err != nil {
			sdr.Close()
		}
	}()
	if err = binary.Read(sdr.TCPConn, binary.BigEndian, &sdr.Info); err != nil {
		return fmt.Errorf("error getting dongle information: %v", err)
	}
	if !sdr.Info.Valid() {
		return fmt.Errorf("bad magic number: %q", sdr.Info.Magic)
	}
	return nil
}

// DongleInfo is data pulled from the RTLTCPSDR on connection.
type DongleInfo struct {
	Magic     [4]byte
	Tuner     uint32
	GainCount uint32 // Useful for setting gain by index
}

// Valid checks the received magic number matches the expected byte string 'RTL0'.
func (d DongleInfo) Valid() bool {
	return d.Magic == dongleMagic
}

type command struct {
	command   uint8
	Parameter uint32
}

// Command constants defined in rtl_tcp.c
const (
	centerFreq = iota + 1
	sampleRate
	tunerGainMode
	tunerGain
	freqCorrection
	tunerIfGain
	testMode
	agcMode
	directSampling
	offsetTuning
	rtlXtalFreq
	tunerXtalFreq
	gainByIndex
)

func (sdr *RTLTCPSDR) do(cmd uint8, v uint32) error {
	return binary.Write(sdr.TCPConn, binary.BigEndian, command{cmd, v})
}

// Set the center frequency in Hz.
func (sdr *RTLTCPSDR) SetCenterFreq(freq uint32) error {
	return sdr.do(centerFreq, freq)
}

// Set the sample rate in Hz.
func (sdr *RTLTCPSDR) SetSampleRate(rate uint32) error {
	return sdr.do(sampleRate, rate)
}

// Set gain in tenths of dB. (197 => 19.7dB)
func (sdr *RTLTCPSDR) SetGain(gain uint32) error {
	return sdr.do(tunerGain, gain)
}

// Set the Tuner AGC, true to enable.
func (sdr *RTLTCPSDR) SetGainMode(state bool) error {
	if state {
		return sdr.do(tunerGainMode, 0)
	}
	return sdr.do(tunerGainMode, 1)
}

// Set gain by index, must be <= DongleInfo.GainCount
func (sdr *RTLTCPSDR) SetGainByIndex(idx uint32) error {
	if idx > sdr.Info.GainCount {
		return fmt.Errorf("invalid gain index: %d", idx)
	}
	return sdr.do(gainByIndex, idx)
}

// Set frequency correction in ppm.
func (sdr *RTLTCPSDR) SetFreqCorrection(ppm uint32) error {
	return sdr.do(freqCorrection, ppm)
}

// Set tuner intermediate frequency stage and gain.
func (sdr *RTLTCPSDR) SetTunerIfGain(stage, gain uint16) error {
	return sdr.do(tunerIfGain, (uint32(stage)<<16)|uint32(gain))
}

// Set test mode, true for enabled.
func (sdr *RTLTCPSDR) SetTestMode(state bool) error {
	if state {
		return sdr.do(testMode, 1)
	}
	return sdr.do(testMode, 0)
}

// Set RTL AGC mode, true for enabled.
func (sdr *RTLTCPSDR) SetAGCMode(state bool) error {
	if state {
		return sdr.do(agcMode, 1)
	}
	return sdr.do(agcMode, 0)
}

// Set direct sampling mode. 0 = disabled, 1 = i-branch, 2 = q-branch, 3 = direct mod.
func (sdr *RTLTCPSDR) SetDirectSampling(state uint32) error {
	return sdr.do(directSampling, state)
}

// Set offset tuning, true for enabled.
func (sdr *RTLTCPSDR) SetOffsetTuning(state bool) error {
	if state {
		return sdr.do(offsetTuning, 1)
	}
	return sdr.do(offsetTuning, 0)
}

// Set RTL xtal frequency.
func (sdr *RTLTCPSDR) SetRTLXtalFreq(freq uint32) error {
	return sdr.do(rtlXtalFreq, freq)
}

// Set tuner xtal frequency.
func (sdr *RTLTCPSDR) SetTunerXtalFreq(freq uint32) error {
	return sdr.do(tunerXtalFreq, freq)
}

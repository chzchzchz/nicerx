package radio

import (
	"context"
	"errors"
	"fmt"
)

var ErrRateOutOfRange = errors.New("sample rate out of range")
var ErrFrequencyOutOfRange = errors.New("frequency out of range")

type SDR interface {
	SetBand(b HzBand) error
	SetFreqCorrection(ppm uint32) error
	Info() SDRHWInfo
	Close() error
	Reader() *MixerIQReader
}

type SDRFormat struct {
	BitDepth   uint   `json:"bit_depth"`
	CenterHz   uint64 `json:"center_hz"`
	SampleRate uint32 `json:"sample_rate"`
}

type SDRHWInfo struct {
	Id string `json:"id"`

	MinHz         uint64 `json:"min_hz"`
	MaxHz         uint64 `json:"max_hz"`
	MinSampleRate uint32 `json:"min_sample_rate"`
	MaxSampleRate uint32 `json:"max_sample_rate"`

	SDRFormat
}

func Calibrate(s SDR) error {
	for {
		ppm, err := FindPPM(s)
		if err != nil {
			return err
		}
		fmt.Println("measured ppm", ppm)
		if ppm < 1.0 {
			break
		}
		if err := s.SetFreqCorrection(uint32(ppm)); err != nil {
			return err
		}
		ppm, err = FindPPM(s)
		if err != nil {
			return err
		}
		fmt.Println("new ppm", ppm)
		if ppm < 2.0 {
			break
		} else if err := s.SetFreqCorrection(0); err != nil {
			return err
		}
	}
	return nil
}

func NewSDR(ctx context.Context) (SDR, error) { return newRTLSDR(ctx, "0") }

func NewSDRWithSerial(ctx context.Context, ser string) (SDR, error) { return newRTLSDR(ctx, ser) }

func SDRList(ctx context.Context) ([]SDRHWInfo, error) {
	return rtlSDRList(ctx)
}

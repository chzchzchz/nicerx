package radio

import (
	"context"
	"fmt"
)

type SDR interface {
	SetBand(b HzBand) error
	SetFreqCorrection(ppm uint32) error
	Info() SDRHWInfo
	Close() error
	Reader() *MixerIQReader
}

type SDRHWInfo struct {
	Id string `json:"id"`

	BitDepth      uint   `json:"bit_depth"`
	MinHz         uint64 `json:"min_hz"`
	MaxHz         uint64 `json:"max_hz"`
	MaxSampleRate uint32 `json:"max_sample_rate"`

	CenterHz   uint32 `json:"center_hz"`
	SampleRate uint32 `json:"sample_rate"`
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

func NewSDR(ctx context.Context) (SDR, error) { return newRTLSDR(ctx) }

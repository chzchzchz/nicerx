package radio

import (
	"errors"
	"fmt"
)

type ScanConfig struct {
	CenterMHz   float64
	MinWidthMHz float64
}

var ErrBadSampleRate = errors.New("bad sample rate")

var scanSampleRate = 2048000
var scanWindowSamples = 32768

func Scan(sdr SDR, cfg ScanConfig) (ret []FreqBand) {
	hzb := HzBand{Center: uint64(cfg.CenterMHz * 1e6), Width: uint64(scanSampleRate)}
	if err := sdr.SetBand(hzb); err != nil {
		panic(err)
	}
	ret, _ = ScanIQReader(sdr.Reader(), cfg.MinWidthMHz*1e6)
	return ret
}

func ScanIQReader(iqr *MixerIQReader, minWidthHz float64) (ret []FreqBand, err error) {
	if iqr.Width != uint64(scanSampleRate) {
		return nil, ErrBadSampleRate
	}
	sp := NewSpectralPower(iqr.ToMHz(), scanWindowSamples, 50)
	if err = sp.Measure(iqr.Batch64(scanWindowSamples, 50)); err != nil {
		return nil, err
	}
	spurs := sp.Spurs()
	for _, fb := range sp.Bands() {
		if fb.Width <= minWidthHz/1e6 {
			continue
		}
		hasSpur := false
		for _, spur := range spurs {
			if spur.Overlaps(fb) {
				hasSpur = true
				fmt.Printf("========skipping spur %.3f; total: %d======\n",
					spur.Center, len(spurs))
				break
			}
		}
		if !hasSpur {
			ret = append(ret, fb)
		}
	}
	return ret, nil
}

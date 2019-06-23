package radio

import "fmt"

type ScanConfig struct {
	CenterMHz   float64
	MinWidthMHz float64
}

var scanSampleRate = 2048000
var scanWindowSamples = 32768

func Scan(sdr SDR, cfg ScanConfig) (ret []FreqBand) {
	hzb := HzBand{Center: cfg.CenterMHz * 1e6, Width: float64(scanSampleRate)}
	if err := sdr.SetBand(hzb); err != nil {
		panic(err)
	}
	return ScanIQReader(sdr.Reader(), cfg.MinWidthMHz*1e6)
}

func ScanIQReader(iqr *MixerIQReader, minWidthHz float64) (ret []FreqBand) {
	sp := NewSpectralPower(iqr.ToMHz(), scanWindowSamples, 50)
	sp.Measure(iqr.Batch64(scanWindowSamples, 50))
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
	return ret
}

package radio

import "fmt"

type ScanConfig struct {
	CenterMHz   float64
	MinWidthMHz float64
}

var scanSampleRate = 2048000
var scanWindowSamples = 32768

func Scan(sdr *SDR, cfg ScanConfig) (ret []FreqBand) {
	fb := FreqBand{Center: cfg.CenterMHz, Width: float64(scanSampleRate) / 1e6}
	if err := sdr.SetBand(fb); err != nil {
		panic(err)
		return nil
	}
	sp := NewSpectralPower(fb, scanWindowSamples, 50)
	sp.Measure(NewIQReader(sdr).Batch64(scanWindowSamples, 50))
	spurs := sp.Spurs()
	for _, fb := range sp.Bands() {
		if fb.Width <= cfg.MinWidthMHz {
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

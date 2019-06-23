package radio

import (
	"math"
)

const ppmSampleRate = 2048000
const ppmBuckets = 8192
const ppmBucketMHz = ppmSampleRate / ppmBuckets / 1.0e6
const ppmCenterMHz = 162.0
const ppmFFTs = 100

func FindPPM(sdr SDR) (float64, error) {
	b := HzBand{Center: ppmCenterMHz * 1e6, Width: ppmSampleRate}
	if err := sdr.SetBand(b); err != nil {
		return 0, err
	}
	ppmFB := FreqBand{Center: ppmCenterMHz, Width: float64(ppmSampleRate) / 1e6}
	sp := NewSpectralPower(ppmFB, ppmBuckets, ppmFFTs)
	sp.Measure(sdr.Reader().Batch64(ppmBuckets, ppmFFTs))
	topAvg, topFreq := 0.0, 0.0
	for i, v := range sp.Average()[ppmBuckets/2+2:] {
		if v > topAvg {
			topAvg = v
			topFreq = ppmCenterMHz + float64(i+2)*ppmBucketMHz
		}
	}
	targetFreq, df := 0.0, 999999.0
	noaa := []float64{162.4, 162.425, 162.450, 162.475,
		162.500, 162.525, 162.550}
	for _, f := range noaa {
		if diff := math.Abs(topFreq - f); diff < df {
			targetFreq, df = f, diff
		}
	}
	return 1e6 * df / targetFreq, nil
}

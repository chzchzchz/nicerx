package radio

import (
	"io"
	"math"
	"math/cmplx"
	"sort"

	"github.com/runningwild/go-fftw/fftw32"
)

type SpectralPower struct {
	min     []float64
	max     []float64
	avg     []float64
	med     []float64
	fftBins *fftw32.Array
	ffts    int
	band    FreqBand
}

type binBand struct {
	Begin int
	Bins  int
	DB    float64
}

func NewSpectralPower(band FreqBand, bins, ffts int) *SpectralPower {
	return &SpectralPower{
		fftBins: fftw32.NewArray(bins),
		ffts:    ffts,
		band:    band,
	}
}

func (sp *SpectralPower) Average() []float64 { return sp.avg }

func (sp *SpectralPower) NoiseFloor() float64 {
	med := make([]float64, len(sp.med))
	copy(med, sp.med)
	sort.Float64s(med)
	return med[len(med)/2]
}

func (sp *SpectralPower) Spread() float64 {
	med := make([]float64, len(sp.avg))
	copy(med, sp.avg)
	sort.Float64s(med)
	return med[len(med)/2]
}

func (sp *SpectralPower) Stddev() float64 {
	spr, sdev := sp.Spread(), 0.0
	for _, v := range sp.avg {
		sdev += (v - spr) * (v - spr)
	}
	sdev /= float64(len(sp.avg) - 1)
	return math.Sqrt(sdev)
}

func (sp *SpectralPower) Spurs() (ret []FreqBand) {
	spr, sdev := sp.Spread(), sp.Stddev()
	for i := 1; i < len(sp.avg)-1; i++ {
		left, mid, right := sp.avg[i-1]-spr, sp.avg[i]-spr, sp.avg[i+1]-spr
		if mid < 0 {
			continue
		}
		if mid-left > 2.0*sdev && mid-right > 2.0*sdev {
			ret = append(ret, sp.freq(binBand{i, 1, sp.avg[i]}))
		}
	}
	return ret
}

func (sp *SpectralPower) Bands() (ret []FreqBand) {
	spr, sdev := sp.Spread(), sp.Stddev()
	begin, end := -1, -1
	db := 0.0
	for i, avg := range sp.avg {
		if avg-spr >= 1.5*sdev {
			if begin == -1 {
				if i == 0 || sp.avg[i-1]-spr > (avg-spr)/2.0 {
					continue
				}
				begin = i
			}
			end = i
			db += avg - spr
		} else if begin != -1 {
			n := end - begin + 1
			bb := binBand{begin, (end - begin) + 1, db / float64(n)}
			ret = append(ret, sp.freq(bb))
			begin, db = -1, 0
		}
	}
	if begin != -1 {
		n := end - begin + 1
		bb := binBand{begin, (end - begin) + 1, db / float64(n)}
		ret = append(ret, sp.freq(bb))
	}
	return ret
}

func (sp *SpectralPower) binMHz() float64 {
	bins := len(sp.fftBins.Elems)
	return sp.band.Width / float64(bins)
}

func (sp *SpectralPower) BandPower(fb FreqBand, samps int) float64 {
	bins := len(sp.fftBins.Elems)
	bandBins := int(fb.Width / sp.binMHz())
	startOffMHz := fb.BeginMHz() - sp.band.Center
	startBin := int(startOffMHz/sp.binMHz() + float64(bins/2))
	avg := 0.0
	for i := 0; i < bandBins; i++ {
		avg += sp.avg[i+startBin]
	}
	return (avg / float64(bandBins)) - sp.Spread()
}

func (sp *SpectralPower) freq(bb binBand) FreqBand {
	beginMHz := float64(bb.Begin-len(sp.fftBins.Elems)/2)*sp.binMHz() + sp.band.Center
	bw := float64(bb.Bins) * sp.binMHz()
	return FreqBand{Center: beginMHz + bw/2.0, Width: bw}
}

func (sp *SpectralPower) Measure(ch <-chan []complex64) error {
	sp.min = make([]float64, len(sp.fftBins.Elems))
	sp.max = make([]float64, len(sp.fftBins.Elems))
	sp.avg = make([]float64, len(sp.fftBins.Elems))
	sp.med = make([]float64, len(sp.fftBins.Elems))
	meds := make([][]float64, len(sp.fftBins.Elems))
	medSamples := 10
	if medSamples > sp.ffts {
		medSamples = sp.ffts
	}
	for i := range meds {
		meds[i] = make([]float64, medSamples)
	}
	arr := &fftw32.Array{}
	for n := 0; n < sp.ffts; n++ {
		samps, ok := <-ch
		if !ok {
			return io.EOF
		}
		arr.Elems = samps
		sp.fftBins = fftw32.FFT(arr)
		for i, v := range sp.fftBins.Elems {
			idx := i + len(sp.fftBins.Elems)/2
			if i >= len(sp.fftBins.Elems)/2 {
				idx = i - len(sp.fftBins.Elems)/2
			}
			db := 20 * math.Log10(cmplx.Abs(complex128(v)))
			sp.avg[idx] += db / float64(sp.ffts)
			if sp.min[idx] == 0 || sp.min[idx] > db {
				sp.min[idx] = db
			}
			if sp.max[idx] == 0 || sp.max[idx] < db {
				sp.max[idx] = db
			}
			meds[idx][((len(meds[idx])-1)*n)/sp.ffts] = db
		}
	}
	for i := range sp.med {
		sort.Float64s(meds[i])
		sp.med[i] = meds[i][len(meds[i])/2]
	}
	return nil
}

package radio

import (
	"math"
)

type HzBand struct {
	Center uint64 `json:"center_hz"`
	Width  uint64 `json:"width_hz"`
}

func (hzb HzBand) ToMHz() FreqBand {
	return FreqBand{
		float64(hzb.Center) / 1e6,
		float64(hzb.Width) / 1e6,
	}
}

// TODO: replace with HzBand
type FreqBand struct {
	Center float64
	Width  float64
}

func (f FreqBand) BeginMHz() float64     { return f.Center - f.Width/2.0 }
func (f FreqBand) EndMHz() float64       { return f.Center + f.Width/2.0 }
func (f FreqBand) BandwidthKHz() float64 { return f.Width * 1e3 }
func (f FreqBand) ToHzBand() HzBand {
	return HzBand{
		Center: uint64(f.Center * 1e6),
		Width:  uint64(f.Width * 1e6),
	}
}

func NewFreqRange(loMHz, hiMHz float64) FreqBand {
	return FreqBand{Center: (hiMHz + loMHz) / 2.0, Width: hiMHz - loMHz}
}

func (fb1 *FreqBand) merge(fb2 FreqBand) {
	begin := math.Min(fb1.BeginMHz(), fb2.BeginMHz())
	end := math.Max(fb1.EndMHz(), fb2.EndMHz())
	fb1.Center = (end + begin) / 2.0
	fb1.Width = end - begin
}

func (fb1 *FreqBand) Overlaps(fb2 FreqBand) bool {
	return !(fb2.EndMHz() < fb1.BeginMHz() || fb2.BeginMHz() > fb1.EndMHz())
}

func BandMerge(fb1 []FreqBand) (ret []FreqBand) {
	anyMerge := false
	for i := range fb1 {
		merged := false
		for j := range ret {
			if fb1[i].Overlaps(ret[j]) {
				ret[j].merge(fb1[i])
				merged = true
			}
		}
		if !merged {
			ret = append(ret, fb1[i])
		} else {
			anyMerge = true
		}
	}
	if anyMerge {
		return BandMerge(ret)
	}
	return ret
}

func BandRange(fb []FreqBand) FreqBand {
	br := fb[0]
	for _, v := range fb {
		br.merge(v)
	}
	return br
}

package radio

import (
	"math"
	"sort"
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

func (hzb HzBand) Overlaps(hz2 HzBand) bool {
	return hzb.ToMHz().Overlaps(hz2.ToMHz())
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

func (fb1 FreqBand) Overlaps(fb2 FreqBand) bool {
	return !(fb2.EndMHz() < fb1.BeginMHz() || fb2.BeginMHz() > fb1.EndMHz())
}

type Bands []FreqBand

func (a Bands) Len() int           { return len(a) }
func (a Bands) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a Bands) Less(i, j int) bool { return a[i].BeginMHz() < a[j].BeginMHz() }

func BandMerge(fbs []FreqBand) (ret []FreqBand) {
	if len(fbs) == 0 {
		return nil
	}
	sort.Sort(Bands(fbs))
	ret = append(ret, fbs[0])
	for _, fb := range fbs[1:] {
		if fb.BeginMHz() > ret[len(ret)-1].EndMHz() {
			ret = append(ret, fb)
		} else {
			ret[len(ret)-1].merge(fb)
		}
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

package nicerx

import (
	"image"
	"image/color"
	"image/jpeg"
	"math/cmplx"
	"os"

	"github.com/chzchzchz/nicerx/radio"

	"github.com/runningwild/go-fftw/fftw32"
)

// black, green, yellow, white
var colorScale = []color.NRGBA{
	{0, 0, 0, 255},
	{0, 255, 0, 255},
	{255, 255, 0, 255},
	{255, 255, 255, 255},
}

func interpolate(t float64, a, b uint8) uint8 { return uint8(float64(a)*(1-t) + float64(b)*t) }

func fftBin2Color(v float64) color.NRGBA {
	idx := float64(len(colorScale)-1) * v
	if int(idx)+1 >= len(colorScale) {
		panic("bad idx")
	}
	t := idx - float64(int(idx))
	prev, next := colorScale[int(idx)], colorScale[int(idx)+1]
	return color.NRGBA{
		interpolate(t, prev.R, next.R),
		interpolate(t, prev.G, next.G),
		interpolate(t, prev.B, next.B),
		255,
	}
}

func WriteSpectrogramFile(infn, outfn string, bins int) error {
	inf, err := os.Open(infn)
	if err != nil {
		return err
	}
	defer inf.Close()
	fi, err := inf.Stat()
	if err != nil {
		return err
	}
	lines := int(fi.Size()) / 2 / bins

	r := image.Rectangle{Min: image.Point{0, 0}, Max: image.Point{bins, lines}}
	img := image.NewNRGBA(r)

	arr := &fftw32.Array{}
	iqr := radio.NewIQReader(inf)
	y := 0
	for samps := range iqr.Batch64(bins, 0) {
		arr.Elems = samps
		fft := make([]float64, len(samps))
		for i, v := range fftw32.FFT(arr).Elems {
			fft[i] = cmplx.Abs(complex128(v))
		}
		min, max := fft[0], fft[0]
		for _, v := range fft {
			if v < min {
				min = v
			}
			if v > max {
				max = v
			}
		}
		// scale to [0, 1)
		scale := 1.0 / ((max - min) + 0.001)
		// Order so lowest and highest frequencies are at the beginning
		// and end, respectively.
		fftOrdered := append(fft[bins/2:], fft[:bins/2]...)
		for x, v := range fftOrdered {
			val := scale * (v - min)
			val = val * val
			img.SetNRGBA(x, y, fftBin2Color(val))
		}
		y++
	}

	outf, err := os.OpenFile(outfn, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer outf.Close()
	if err := jpeg.Encode(outf, img, nil); err != nil {
		return err
	}

	return nil
}

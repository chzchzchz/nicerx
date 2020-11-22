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

func FFTBin2Color(v float64) color.NRGBA {
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

type spectrogram struct {
	plan    *fftw32.Plan
	in, out *fftw32.Array
	inc     <-chan []complex64
	outc    chan<- []float64
}

func (sp *spectrogram) run() {
	bins := len(sp.out.Elems)
	for samps := range sp.inc {
		copy(sp.in.Elems, samps)
		sp.plan.Execute()

		fft := make([]float64, len(samps))
		min, max := 0.0, 0.0
		for i, v := range sp.out.Elems[1:] {
			j := i + 1
			fft[j] = cmplx.Abs(complex128(v))
			if fft[j] < min {
				min = fft[j]
			}
			if fft[j] > max {
				max = fft[j]
			}
		}

		// Cheat to hide DC bias bucket by averaging adjacent buckets.
		fft[0] = (fft[1] + fft[len(fft)-1]) / 2.0

		// Scale to [0, 1), also hiding DC bias.
		scale := 1.0 / ((max - min) + 0.001)
		for i, v := range fft {
			fft[i] = scale * (v - min)
		}

		// Order so lowest and highest frequencies are at the beginning
		// and end, respectively.
		sp.outc <- append(fft[bins/2:], fft[:bins/2]...)
	}
}

func SpectrogramChan(inc <-chan []complex64, bins int) <-chan []float64 {
	outc := make(chan []float64, 2)
	go func() {
		defer close(outc)
		in, out := fftw32.NewArray(bins), fftw32.NewArray(bins)
		plan := fftw32.NewPlan(in, out, fftw32.Forward, fftw32.DefaultFlag)
		defer plan.Destroy()
		sp := spectrogram{
			plan: plan,
			in:   in,
			out:  out,
			inc:  inc,
			outc: outc,
		}
		sp.run()
	}()
	return outc
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

	img := image.NewNRGBA(image.Rect(0, 0, bins, lines))
	iqr := radio.NewIQReader(inf)
	y := 0
	for fft := range SpectrogramChan(iqr.Batch64(bins, 0), bins) {
		for x, v := range fft {
			img.SetNRGBA(x, y, FFTBin2Color(v*v))
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

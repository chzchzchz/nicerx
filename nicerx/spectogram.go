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
		scale := 255.0 / (max - min)
		// Order so lowest and highest frequencies are at the beginning
		// and end, respectively.
		fftOrdered := append(fft[bins/2:], fft[:bins/2]...)
		c := color.NRGBA{R: 0, G: 0, B: 0, A: 255}
		for x, v := range fftOrdered {
			c.G = uint8((v - min) * scale)
			img.SetNRGBA(x, y, c)
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

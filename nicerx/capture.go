package nicerx

import (
	"context"
	"io"
	"log"

	"github.com/chzchzchz/nicerx/dsp"
	"github.com/chzchzchz/nicerx/radio"
	"github.com/chzchzchz/nicerx/store"
)

type Capture struct {
	sdr  radio.SDR
	band radio.FreqBand
	ss   *store.SignalStore
}

const windowSize = 20
const sdrRate = 1024000
const offsetHz = 2 * 10240
const windowSamples = 8192
const maxWindowWrite = 40

func NewCapture(sdr radio.SDR,
	band radio.FreqBand,
	ss *store.SignalStore) *Capture {
	return &Capture{sdr, band, ss}
}

func (c *Capture) Band() radio.FreqBand { return c.band }

func (c *Capture) Step(ctx context.Context) error {
	hzb := radio.HzBand{Center: c.band.Center*1e6 - offsetHz, Width: float64(sdrRate)}
	if err := c.sdr.SetBand(hzb); err != nil {
		return err
	}

	// Read windows from SDR and compute FFT.
	cctx, cancel := context.WithCancel(ctx)
	defer cancel()
	sampc := c.sdr.Reader().BatchStream64(cctx, windowSamples, 0)
	fb := radio.FreqBand{
		Center: hzb.Center / 1e6,
		Width:  float64(sdrRate) / 1e6}
	sp := radio.NewSpectralPower(fb, windowSamples, windowSize)
	readWindow := func() (samps [][]complex64) {
		windowc, donec := make(chan []complex64, windowSize), make(chan struct{})
		go func() {
			defer close(donec)
			sp.Measure(windowc)
		}()
		for samp := range sampc {
			samps = append(samps, samp)
			windowc <- samp
			if len(samps) >= windowSize {
				break
			}
		}
		close(windowc)
		<-donec
		return samps
	}

	// Push signal samples to processCapture.
	var outc chan []complex64
	var lastSamps [][]complex64
	writtenWindows := -1
	mercy := 1
	writeWindow := func(oc chan []complex64, samps [][]complex64) {
		for _, samp := range samps {
			oc <- samp
		}
		writtenWindows++
	}
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		samps := readWindow()
		gain, sdev := sp.BandPower(c.band, sdrRate), sp.Stddev()
		if gain > sdev && writtenWindows < maxWindowWrite {
			if outc == nil {
				outc = make(chan []complex64, windowSize)
				defer close(outc)
				go c.processCapture(outc)
				writeWindow(outc, lastSamps)
			}
			mercy = 1
			writeWindow(outc, samps)
			log.Printf("mhz: %.3f; gain: %.3f; sdev: %.3f; #%d\n", c.band.Center, gain, sdev, writtenWindows)
		} else if outc != nil {
			writeWindow(outc, samps)
			if mercy > 0 {
				mercy--
				continue
			}
			return io.EOF
		} else {
			lastSamps = samps
		}
	}
}

func (c *Capture) processCapture(sampc <-chan []complex64) error {
	mdc := dsp.MixDown(offsetHz, sdrRate, sampc)
	decRate := 1
	outHz := sdrRate
	sigHz := int(2 * c.band.Width * 1e6)
	for (outHz%2) == 0 && (outHz/2) > sigHz {
		outHz /= 2
		decRate *= 2
	}
	lpc := dsp.Lowpass(c.band.Width*1e6, sdrRate, decRate, mdc)

	outfb := radio.FreqBand{Center: c.band.Center, Width: float64(outHz) / 1e6}
	outf, err := c.ss.OpenFile(outfb)
	if err != nil {
		return err
	}
	defer outf.Close()
	iqw := radio.NewIQWriter(outf)
	for samps := range lpc {
		if err := iqw.Write64(samps); err != nil {
			return err
		}
	}
	return WriteSpectrogramFile(outf.Name(), outf.Name()+".jpg", 256)
}

func (c *Capture) Name() string { return "capture" }

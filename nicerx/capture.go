package nicerx

import (
	"context"
	"io"
	"log"
	"os"

	"github.com/chzchzchz/nicerx/dsp"
	"github.com/chzchzchz/nicerx/radio"
	"github.com/chzchzchz/nicerx/store"
)

type Capture struct {
	sdr  *radio.SDR
	band radio.FreqBand
	ss   *store.SignalStore
}

const windowSize = 20
const sdrRate = 1024000
const offsetHz = 2 * 10240
const windowSamples = 8192
const maxWindowWrite = 40

func NewCapture(sdr *radio.SDR,
	band radio.FreqBand,
	ss *store.SignalStore) *Capture {
	return &Capture{sdr, band, ss}
}

func (c *Capture) Band() radio.FreqBand { return c.band }

func (c *Capture) Step(ctx context.Context) error {
	centerHz := uint32(c.band.Center*1e6) - offsetHz
	fb := radio.FreqBand{
		Center: float64(centerHz)/1e6,
		Width: float64(sdrRate) / 1e6}
	if err := c.sdr.SetBand(fb); err != nil {
		return err
	}
	cctx, cancel := context.WithCancel(ctx)
	defer cancel()
	sampc := radio.NewIQReader(c.sdr.SDR).BatchStream64(cctx, windowSamples, 0)
	sp := radio.NewSpectralPower(fb, windowSamples, windowSize)
	var iqw *radio.IQWriter
	var f *os.File
	var samps, lastSamps [][]complex64
	writtenWindows := 0
	finishStream := func() error {
		if samps != nil {
			writeWindow(iqw, samps)
		}
		writtenWindows = 0
		fn := f.Name()
		f.Close()
		return c.processTmpCapture(fn)
	}
	for ctx.Err() == nil {
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
		gain, sdev := sp.BandPower(c.band, sdrRate), sp.Stddev()
		if gain > sdev && writtenWindows < maxWindowWrite {
			log.Printf("mhz: %.3f; gain: %.3f; sdev: %.3f; #%d\n", c.band.Center, gain, sdev, writtenWindows)
			if iqw == nil {
				ff, err := c.ss.OpenFile(c.sdr.Band())
				if err != nil {
					return err
				}
				f = ff
				iqw = radio.NewIQWriter(f)
				writeWindow(iqw, lastSamps)
			}
			writeWindow(iqw, samps)
			writtenWindows++
		} else if iqw != nil {
			return finishStream()
		}
		lastSamps, samps = samps, nil
	}
	if f != nil {
		finishStream()
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return io.EOF
}

func writeWindow(iqw *radio.IQWriter, samps [][]complex64) error {
	for _, v := range samps {
		if err := iqw.Write64(v); err != nil {
			return err
		}
	}
	return nil
}

func (c *Capture) processTmpCapture(fn string) error {
	f, err := os.Open(fn)
	if err != nil {
		return err
	}
	defer f.Close()
	iqr := radio.NewIQReader(f)
	mdc := dsp.MixDown(offsetHz, sdrRate, iqr.Batch64(windowSamples, 0))
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

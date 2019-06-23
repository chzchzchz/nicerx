package nicerx

import (
	"context"
	"fmt"
	"io"

	"github.com/chzchzchz/nicerx/radio"
	"github.com/chzchzchz/nicerx/store"
)

type Scanner struct {
	sdr   radio.SDR
	bands *store.BandStore

	scanBand    radio.FreqBand
	currentBand radio.FreqBand
	bandwidth   float64
}

func NewScanner(sdr radio.SDR, b *store.BandStore) *Scanner {
	s := &Scanner{
		sdr:         sdr,
		bands:       b,
		scanBand:    radio.NewFreqRange(432.0, 1300.0),
		currentBand: radio.FreqBand{Center: 432.0, Width: 1.5},
		bandwidth:   1.5,
	}
	s.bands.Load("band.db")
	return s
}

func (s *Scanner) Band() radio.FreqBand { return s.currentBand }

func (s *Scanner) Step(ctx context.Context) error {
	if s.currentBand.BeginMHz() > s.scanBand.EndMHz() {
		return io.EOF
	}
	var fbands []radio.FreqBand
	fmt.Printf("===scan %.2f MHz==\n", s.currentBand.Center)
	for j := 0; j < 20; j++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		bands := radio.Scan(
			s.sdr,
			radio.ScanConfig{
				CenterMHz:   s.currentBand.Center,
				MinWidthMHz: 0.002})
		fmt.Printf("bands[%d]: %d\r", j, len(bands))
		fbands = append(fbands, bands...)
	}
	fbands = radio.BandMerge(fbands)
	fmt.Println("\nbands: ", len(fbands))
	s.bands.Add(fbands)
	s.bands.Save("band.db")
	s.currentBand = radio.FreqBand{
		Center: s.currentBand.Center + s.bandwidth,
		Width:  s.bandwidth,
	}
	return nil
}

func (s *Scanner) Name() string { return "scan" }

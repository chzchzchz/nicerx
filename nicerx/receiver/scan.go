package receiver

import (
	"context"

	"github.com/chzchzchz/nicerx/radio"
)

func NewScan() RxFunc { return openScan }

func openScan(ctx context.Context, iqr *radio.MixerIQReader) *RxStream {
	ch := make(chan interface{}, 1)
	rs := &RxStream{ch: ch}
	go func() {
		var err error
		defer func() {
			rs.err = err
			close(ch)
		}()
		for {
			var fbands []radio.FreqBand
			for i := 0; i < 20; i++ {
				bands, err := radio.ScanIQReader(iqr, 2000)
				if err != nil {
					return
				}
				fbands = append(fbands, bands...)
			}
			fbands = radio.BandMerge(fbands)
			ret := make([]radio.HzBand, 0, len(fbands))
			for _, f := range fbands {
				ret = append(ret, f.ToHzBand())
			}
			select {
			case ch <- ret:
			case <-ctx.Done():
				err = ctx.Err()
				return
			}
		}
	}()
	return rs
}

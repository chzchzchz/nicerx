package server

import (
	"context"
	"log"

	"github.com/chzchzchz/nicerx/dsp"
	"github.com/chzchzchz/nicerx/radio"
	"github.com/chzchzchz/nicerx/sdrproxy"
)

type SignalChannel <-chan []complex64

type Signal struct {
	req  sdrproxy.RxRequest
	resp sdrproxy.RxResponse

	serv   *Server
	sigc   <-chan []complex64
	cancel context.CancelFunc
	readyc <-chan struct{}
}

func newSignalChannel(ctx context.Context, req radio.HzBand, iqr *radio.MixerIQReader) (SignalChannel, error) {
	if !req.Overlaps(iqr.HzBand) {
		return nil, sdrproxy.ErrOutOfRange
	}

	// Setup band by choosing rate and filters to get band via SDR bands.
	processSignal := func(ch <-chan []complex64) <-chan []complex64 { return ch }
	if iqr.Width != req.Width || iqr.Center != req.Center {
		log.Print("upsample to ", iqr.Width, " and downsample to ", req.Width)
		log.Print("xlate to ", req.Center, " from input center ", iqr.Center)
		processSignal = func(ch <-chan []complex64) <-chan []complex64 {
			/* Oversampled; down mix translate, lowpass, downsample */
			mixc := ch
			if iqr.Center != req.Center {
				mixc = dsp.MixDownCtx(ctx, float64(req.Center)-float64(iqr.Center), int(iqr.Width), ch)
			}

			dec := 1
			for iqr.Width > uint64(dec)*4*req.Width {
				dec *= 2
			}
			lpc := dsp.LowpassCtx(ctx, float64(req.Width), int(iqr.Width), dec, mixc)
			resampleRatio := float32(req.Width) / (float32(iqr.Width) / float32(dec))
			return dsp.ResampleComplex64Ctx(ctx, resampleRatio, lpc)
		}
	}
	/* TODO: xlate filter to avoid DC bias */
	return processSignal(iqr.BatchStream64(ctx, int(iqr.Width), 0)), nil
}

func (s *Signal) Response() sdrproxy.RxResponse { return s.resp }

func (s *Signal) Chan() SignalChannel {
	return s.sigc
}

func (s *Signal) stop() error {
	s.cancel()
	for range <-s.sigc {
	}
	return nil
}

func (s *Signal) Close() error {
	err := s.stop()
	s.serv.removeSignal(s.req.Name)
	return err
}

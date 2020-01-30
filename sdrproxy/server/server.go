package server

import (
	"context"
	"log"
	"sync"

	"github.com/chzchzchz/nicerx/dsp"
	"github.com/chzchzchz/nicerx/radio"
	"github.com/chzchzchz/nicerx/sdrproxy"
)

type Signal struct {
	req  sdrproxy.RxRequest
	resp sdrproxy.RxResponse

	serv   *Server
	sdr    radio.SDR
	sigc   <-chan []complex64
	cancel context.CancelFunc
}

type Server struct {
	signals map[string]*Signal
	rwmu    sync.RWMutex
}

func NewServer() *Server {
	return &Server{
		signals: make(map[string]*Signal),
	}
}

func (s *Server) OpenSignal(ctx context.Context, req sdrproxy.RxRequest) (sig *Signal, err error) {
	cctx, cancel := context.WithCancel(ctx)

	sig = &Signal{req: req, serv: s, cancel: cancel}
	s.rwmu.Lock()
	if _, ok := s.signals[req.Name]; ok {
		err = sdrproxy.ErrSignalExists
	} else {
		s.signals[req.Name] = sig
	}
	s.rwmu.Unlock()
	if err != nil {
		return nil, err
	}

	if sig.sdr, err = radio.NewSDRWithSerial(cctx, req.Radio); err != nil {
		s.removeSignal(req.Name)
		return nil, err
	}
	// Setup band by choosing rate and filters to get band via SDR bands.
	info, sdrBand := sig.sdr.Info(), req.HzBand
	processSignal := func(ch <-chan []complex64) <-chan []complex64 { return ch }
	sdrBand.Width = uint64(getSampleRate(uint32(sdrBand.Width)))
	if sdrBand.Width != req.HzBand.Width {
		log.Print("upsample to ", sdrBand.Width, " and downsample to ", req.HzBand.Width)
		processSignal = func(ch <-chan []complex64) <-chan []complex64 {
			/* Oversample, lowpass, downsample */
			lpc := dsp.Lowpass(float64(req.HzBand.Width), int(sdrBand.Width), 1, ch)
			resampleRatio := float32(req.HzBand.Width) / float32(sdrBand.Width)
			return dsp.ResampleComplex64(resampleRatio, lpc)
		}
	}
	if err = sig.sdr.SetBand(sdrBand); err != nil {
		cancel()
		sig.sdr.Close()
		s.removeSignal(req.Name)
		return nil, err
	}

	/* TODO: xlate filter to avoid DC bias */
	sig.sigc = processSignal(sig.sdr.Reader().BatchStream64(cctx, int(sdrBand.Width), 0))

	info = sig.sdr.Info()
	dataFormat := radio.SDRFormat{
		BitDepth:   info.BitDepth,
		CenterHz:   req.HzBand.Center,
		SampleRate: uint32(req.HzBand.Width),
	}
	sig.resp = sdrproxy.RxResponse{Format: dataFormat, Radio: info}
	return sig, err
}

func (s *Server) removeSignal(name string) {
	s.rwmu.Lock()
	delete(s.signals, name)
	s.rwmu.Unlock()
}

func (s *Signal) Response() sdrproxy.RxResponse { return s.resp }

func (s *Signal) Chan() <-chan []complex64 { return s.sigc }

func (s *Signal) stop() error {
	s.cancel()
	for range <-s.sigc {
	}
	return s.sdr.Close()
}

func (s *Signal) Close() error {
	err := s.stop()
	s.serv.removeSignal(s.req.Name)
	return err
}

func (s *Server) Signals() (ret []sdrproxy.RxSignal) {
	s.rwmu.RLock()
	defer s.rwmu.RUnlock()
	for _, sig := range s.signals {
		rxsig := sdrproxy.RxSignal{Request: sig.req, Response: sig.resp}
		ret = append(ret, rxsig)
	}
	return ret
}

// TODO: move into radio code probably
func getSampleRate(wantRate uint32) uint32 {
	rates := []uint32{240000, 960000, 1152000, 1600000, 1800000, 1920000, 2400000}
	for _, v := range rates {
		if wantRate < v {
			return v
		}
	}
	return 3200000
}

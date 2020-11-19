package server

import (
	"context"
	"io"
	"sync"

	"github.com/chzchzchz/nicerx/radio"
	"github.com/chzchzchz/nicerx/sdrproxy"
)

type serverSDR struct {
	radio.SDR
	readyc <-chan struct{}
}

type Server struct {
	// signals holds all signals attached.
	signals map[string]*Signal

	// sdrs holds all open SDRs.
	sdrs map[string]*serverSDR

	// muxers holds all SDR muxer readers
	// TODO TODO TODO

	rwmu sync.RWMutex
}

func NewServer() *Server {
	return &Server{
		sdrs:    make(map[string]*serverSDR),
		signals: make(map[string]*Signal),
	}
}

func (s *Server) OpenSignal(ctx context.Context, req sdrproxy.RxRequest) (sig *Signal, err error) {
	cctx, cancel := context.WithCancel(ctx)
	s.rwmu.Lock()
	sig, ok := s.signals[req.Name]
	if !ok {
		readyc := make(chan struct{})
		defer close(readyc)
		sig = &Signal{req: req, serv: s, cancel: cancel, readyc: readyc}
		s.signals[req.Name] = sig
	}
	s.rwmu.Unlock()

	if ok {
		if req.HzBand != sig.req.HzBand || req.Radio != sig.req.Radio {
			return nil, sdrproxy.ErrSignalExists
		}
		select {
		case <-sig.readyc:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return sig, nil
	}

	r, sdr, err := s.openMuxReader(cctx, req)
	if err != nil {
		s.removeSignal(req.Name)
		return nil, err
	}

	if sig.sigc, err = newSignalChannel(cctx, req.HzBand, r); err != nil {
		s.removeSignal(req.Name)
		return nil, err
	}
	dataFormat := radio.SDRFormat{
		BitDepth:   8, //info.BitDepth,
		CenterHz:   req.HzBand.Center,
		SampleRate: uint32(req.HzBand.Width),
	}
	sig.resp = sdrproxy.RxResponse{Format: dataFormat, Radio: sdr.Info()}
	return sig, nil
}

func (s *Server) openMuxReader(ctx context.Context, req sdrproxy.RxRequest) (*radio.MixerIQReader, radio.SDR, error) {
	sdr, err := s.openSDR(ctx, req)
	if err != nil {
		s.removeSignal(req.Name)
		return nil, nil, err
	}
	return sdr.Reader(), sdr, nil
}

func (s *Server) openSDR(ctx context.Context, req sdrproxy.RxRequest) (radio.SDR, error) {
	s.rwmu.Lock()
	curSDR, ok := s.sdrs[req.Radio]
	if !ok {
		readyc := make(chan struct{})
		curSDR = &serverSDR{readyc: readyc}
		defer close(readyc)
		s.sdrs[req.Radio] = curSDR
	}
	s.rwmu.Unlock()

	if ok {
		// Wait for SDR to be ready.
		select {
		case <-curSDR.readyc:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		if curSDR.SDR == nil {
			return nil, io.EOF
		}
		return curSDR.SDR, nil
	}

	sdr, err := radio.NewSDRWithSerial(ctx, req.Radio)
	if err != nil {
		s.closeSDR(req.Radio)
		return nil, err
	}
	curSDR.SDR = sdr

	// Adjust band based on hints to cover wider range without retuning.
	sdrBand := req.HzBand
	if req.HintTuneHz != 0 {
		w := uint64(2048000)
		if req.HintTuneWidthHz != 0 {
			w = req.HintTuneWidthHz
		}
		sdrBand = radio.HzBand{Center: req.HintTuneHz, Width: w}
	} else {
		sdrBand.Width = uint64(getSampleRate(uint32(sdrBand.Width)))
	}

	if err := sdr.SetBand(sdrBand); err != nil {
		s.closeSDR(req.Radio)
		return nil, err
	}

	return sdr, nil
}

func (s *Server) Close() {
	s.rwmu.Lock()
	defer s.rwmu.Unlock()
	for _, sdr := range s.sdrs {
		sdr.Close()
	}
	s.sdrs = make(map[string]*serverSDR)
}

func (s *Server) removeSignal(name string) {
	s.rwmu.Lock()
	sig := s.signals[name]
	delete(s.signals, name)
	s.rwmu.Unlock()
	if sig != nil {
		s.closeSDR(sig.req.Radio)
	}
}

func (s *Server) closeSDR(name string) {
	s.rwmu.Lock()
	defer s.rwmu.Unlock()
	for _, sig := range s.signals {
		if sig.req.Radio == name {
			return
		}
	}
	// No signals reference SDR; may close.
	if sdr := s.sdrs[name]; sdr != nil {
		if sdr.SDR != nil {
			sdr.Close()
		}
		delete(s.sdrs, name)
	}
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
	rates := []uint32{240000, 960000, 1024000, 1152000, 1600000, 1800000, 1920000, 2048000, 2400000}
	for _, v := range rates {
		if wantRate <= v {
			return v
		}
	}
	return 3200000
}

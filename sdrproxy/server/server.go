package server

import (
	"context"
	"sync"

	"github.com/chzchzchz/nicerx/radio"
	"github.com/chzchzchz/nicerx/sdrproxy"
)

type Server struct {
	// signals holds all signals attached.
	signals map[string]*Signal

	// sdrs holds all open SDRs.
	sdrs map[string]radio.SDR

	// muxers holds all SDR muxer readers
	// TODO TODO TODO

	rwmu sync.RWMutex
}

func NewServer() *Server {
	return &Server{
		sdrs:    make(map[string]radio.SDR),
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

	r, sdr, err := s.openMuxReader(req)
	if err != nil {
		return nil, err
	}

	sig.sigc = newSignalChannel(cctx, req.HzBand, r)
	dataFormat := radio.SDRFormat{
		BitDepth:   8, //info.BitDepth,
		CenterHz:   req.HzBand.Center,
		SampleRate: uint32(req.HzBand.Width),
	}
	sig.resp = sdrproxy.RxResponse{Format: dataFormat, Radio: sdr.Info()}
	return sig, nil
}

func (s *Server) openMuxReader(req sdrproxy.RxRequest) (*radio.MixerIQReader, radio.SDR, error) {
	sdr, err := s.openSDR(req)
	if err != nil {
		s.removeSignal(req.Name)
		return nil, nil, err
	}
	return sdr.Reader(), sdr, nil
}

func (s *Server) openSDR(req sdrproxy.RxRequest) (radio.SDR, error) {
	sdr, err := radio.NewSDRWithSerial(context.TODO(), req.Radio)
	if err != nil {
		return nil, err
	}
	s.rwmu.Lock()
	defer s.rwmu.Unlock()
	// Don't create new SDR if one was created in the meantime.
	if curSDR, ok := s.sdrs[req.Radio]; ok {
		defer sdr.Close()
		return curSDR, nil
	} else {
		s.sdrs[req.Radio] = sdr
	}

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
	if err = sdr.SetBand(sdrBand); err != nil {
		sdr.Close()
		delete(s.sdrs, req.Radio)
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
	s.sdrs = make(map[string]radio.SDR)
	return
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
	s.sdrs[name].Close()
	delete(s.sdrs, name)
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

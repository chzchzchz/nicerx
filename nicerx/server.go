package nicerx

import (
	"context"
	"io"
	"sync"

	"github.com/chzchzchz/nicerx/nicerx/receiver"
	"github.com/chzchzchz/nicerx/radio"
	"github.com/chzchzchz/nicerx/store"
)

type Server struct {
	SDR     radio.SDR
	Bands   *store.BandStore
	Tasks   *TaskQueue
	Signals *store.SignalStore

	rxers map[string]*receiver.Rxer

	mu      sync.RWMutex
	tctx    context.Context
	tcancel context.CancelFunc
}

func NewServer(sdr radio.SDR) (*Server, error) {
	ss, err := store.NewSignalStore("bands")
	if err != nil {
		return nil, err
	}
	s := &Server{
		SDR:     sdr,
		Bands:   store.NewBandStore(),
		Tasks:   NewTaskQueue(),
		Signals: ss,
		rxers:   make(map[string]*receiver.Rxer),
	}
	s.Bands.Load("bands.db")
	return s, nil
}

func (s *Server) Serve(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

func (s *Server) Rxers() []receiver.RxConfigBase {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ret := make([]receiver.RxConfigBase, 0, len(s.rxers))
	for _, v := range s.rxers {
		ret = append(ret, v.RxConfigBase)
	}
	return ret
}

func (s *Server) AddRxer(r *receiver.Rxer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rxers[r.UserName] = r
}

func (s *Server) DelRxer(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.rxers, name)
}

func (s *Server) OpenRxer(ctx context.Context, name string) (*receiver.RxStream, error) {
	s.mu.RLock()
	s.mu.RUnlock()
	r := s.rxers[name]
	if r == nil {
		return nil, io.EOF
	}
	return r.Open(ctx, s.SDR.Reader()), nil
}

func (s *Server) Capture(centerMHz float64) {
	fbs := s.Bands.Range(radio.FreqBand{Center: centerMHz, Width: 100 / 1e6})
	if len(fbs) == 0 {
		return
	}
	tid := s.Tasks.Add(NewCapture(s.SDR, fbs[0], s.Signals))
	s.Tasks.Prioritize(tid, 2)
}

type SignalBand struct {
	store.BandRecord
	HasSignal  bool
	HasCapture bool
}

func (s *Server) SignalBands() map[float64]SignalBand {
	bands := s.Bands.Bands()
	captures := s.Tasks.Freqs("capture")
	ret := make(map[float64]SignalBand, len(bands))
	for _, fb := range bands {
		_, hasCapture := captures[fb.Center]
		ret[fb.Center] = SignalBand{
			BandRecord: fb,
			HasSignal:  s.Signals.HasBand(fb.FreqBand),
			HasCapture: hasCapture,
		}
	}
	return ret
}

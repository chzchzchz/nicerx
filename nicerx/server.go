package nicerx

import (
	"context"
	"sync"

	"github.com/chzchzchz/nicerx/radio"
	"github.com/chzchzchz/nicerx/store"
)

type Server struct {
	SDR     radio.SDR
	Bands   *store.BandStore
	Tasks   *TaskQueue
	Signals *store.SignalStore

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
	}
	s.Bands.Load("bands.db")
	s.resetCtx()
	return s, nil
}

func (s *Server) Serve(ctx context.Context) error {
	// s.Tasks.Add(NewScanner(s.SDR, s.Bands))
	for {
		if err := s.Tasks.Run(s.tctx); err != nil && err != s.tctx.Err() {
			return err
		}
		if s.tctx.Err() != nil {
			s.resetCtx()
		}
	}
	return nil
}

func (s *Server) Pause(tid TaskId) {
	s.Tasks.Pause(tid)
	s.resetTask()
}

func (s *Server) Resume(tid TaskId) {
	s.Tasks.Resume(tid)
	s.resetTask()
}

func (s *Server) Stop(tid TaskId) {
	s.Tasks.Stop(tid)
	s.resetTask()
}

func (s *Server) resetTask() {
	s.mu.RLock()
	s.tcancel()
	s.mu.RUnlock()
}

func (s *Server) resetCtx() {
	tctx, tcancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.tctx, s.tcancel = tctx, tcancel
	s.mu.Unlock()
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

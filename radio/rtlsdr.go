package radio

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/bemasher/rtltcp"
	"github.com/kr/pty"
)

var minFreqHz = uint32(25000000)
var maxFreqHz = uint32(1750000000)
var minRate = uint32(225000)
var maxRate = uint32(3200000)

type rtlSDR struct {
	*rtltcp.SDR
	cmd  *exec.Cmd
	fpty *os.File
	// device serial number or device index
	serialNumber string

	lastCenter        uint32
	lastSampleRate    uint32
	lastPPM           uint32
	lastCalibrateTime time.Time

	iqr *MixerIQReader
	mu  sync.RWMutex
}

func newRTLSDR(ctx context.Context, ser string) (*rtlSDR, error) {
	// TODO: support different ports
	cmd := exec.CommandContext(ctx, "rtl_tcp", "-a", "127.0.0.1", "-p", "12345", "-d", ser, "-s", "240000")
	fpty, err := pty.Start(cmd)
	if err != nil {
		return nil, err
	}
	go io.Copy(os.Stdout, fpty)
	// TODO: would like to wait for 'listening...' but need tty to line-buffer
	select {
	case <-time.After(2 * time.Second):
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return &rtlSDR{fpty: fpty, cmd: cmd, serialNumber: ser}, nil
}

func (s *rtlSDR) SetFreqCorrection(ppm uint32) error {
	if err := s.initSDR(); err != nil {
		return err
	}
	s.lastPPM, s.lastCalibrateTime = ppm, time.Now()
	return s.SDR.SetFreqCorrection(ppm)
}

func (s *rtlSDR) SetBand(b HzBand) error {
	if b.Center < uint64(minFreqHz) || b.Center > uint64(maxFreqHz) {
		return ErrFrequencyOutOfRange
	}
	if !isValidRate(uint32(b.Width)) {
		return ErrRateOutOfRange
	}
	if err := s.initSDR(); err != nil {
		return err
	}

	// TODO: don't necessarily recalibrate; slow if not needed / ppm known.
	if time.Since(s.lastCalibrateTime) > 5*time.Minute {
		s.lastCalibrateTime = time.Now()
		if s.lastCalibrateTime.IsZero() {
			if err := s.SDR.SetAGCMode(true); err != nil {
				return err
			}
		}
		if err := Calibrate(s); err != nil {
			return err
		}
	}

	newFreq, newRate := uint32(b.Center), uint32(b.Width)
	if err := s.SetCenterFreq(newFreq); err != nil {
		return err
	}
	if err := s.SetSampleRate(newRate); err != nil {
		return err
	}

	// Reset connection so following reads get the new tuned band.
	return s.resetConn()
}

func (s *rtlSDR) Info() SDRHWInfo {
	return SDRHWInfo{
		Id: s.serialNumber,
		SDRFormat: SDRFormat{
			BitDepth:   8,
			CenterHz:   uint64(s.lastCenter),
			SampleRate: s.lastSampleRate,
		},
		MinHz:         uint64(minFreqHz),
		MaxHz:         uint64(maxFreqHz),
		MinSampleRate: minRate,
		MaxSampleRate: maxRate,
	}
}

func (s *rtlSDR) Close() error {
	s.stop()
	s.fpty.Close()
	return s.cmd.Wait()
}

func (s *rtlSDR) band() HzBand {
	return HzBand{uint64(s.lastCenter), uint64(s.lastSampleRate)}
}

type eofReader struct{}

func (e *eofReader) Read(p []byte) (int, error) { return 0, io.EOF }

func (s *rtlSDR) Reader() *MixerIQReader {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.SDR == nil {
		return NewMixerIQReader(&eofReader{}, s.band())
	} else if s.iqr == nil {
		s.iqr = NewMixerIQReader(s.SDR, s.band())
	}
	return s.iqr
}

func (s *rtlSDR) stop() error {
	if s.SDR == nil {
		return nil
	}
	err := s.SDR.Close()
	s.SDR, s.iqr = nil, nil
	return err
}

func (s *rtlSDR) resetConn() (err error) {
	s.stop()
	s.SDR, err = connect(context.TODO())
	return err
}

func isValidRate(rate uint32) bool {
	return !((rate <= 225000) || (rate > 3200000) ||
		((rate > 300000) && (rate <= 900000)))
}

func (s *rtlSDR) SetSampleRate(rate uint32) (err error) {
	if !isValidRate(rate) {
		return ErrRateOutOfRange
	}
	if s.lastSampleRate == rate {
		return nil
	}
	if err := s.initSDR(); err != nil {
		return err
	}
	if err := s.SDR.SetSampleRate(rate); err != nil {
		return err
	}
	s.lastSampleRate = rate
	return nil
}

func (s *rtlSDR) SetCenterFreq(cent uint32) error {
	if err := s.initSDR(); err != nil {
		return err
	}
	return s.setCenterFreq(cent)
}

func (s *rtlSDR) setCenterFreq(cent uint32) error {
	if cent < minFreqHz || cent > maxFreqHz {
		return ErrFrequencyOutOfRange
	}
	if s.lastCenter != cent {
		if err := s.SDR.SetCenterFreq(cent); err != nil {
			return err
		}
		s.lastCenter = cent
	}
	return nil
}

func connect(ctx context.Context) (*rtltcp.SDR, error) {
	var sdr *rtltcp.SDR
	addr, err := net.ResolveTCPAddr("tcp4", "127.0.0.1:12345")
	if err != nil {
		return nil, err
	}
	for i := 0; i < 10; i++ {
		sdr = &rtltcp.SDR{}
		if err = sdr.Connect(addr); err != nil {
			fmt.Println(err)
			sdr = nil
		} else {
			return sdr, nil
		}
		time.Sleep(100 * time.Millisecond)
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}
	return nil, err
}

func (s *rtlSDR) initSDR() (err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.SDR == nil {
		err = s.resetConn()
	}
	return err
}

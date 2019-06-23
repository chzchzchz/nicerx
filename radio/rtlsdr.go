package radio

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"time"

	"github.com/bemasher/rtltcp"
	"github.com/kr/pty"
)

var minFreqHz = uint32(25000000)
var maxFreqHz = uint32(1750000000)

type rtlSDR struct {
	*rtltcp.SDR
	cmd  *exec.Cmd
	fpty *os.File

	lastCenter        uint32
	lastSampleRate    uint32
	lastPPM           uint32
	lastCalibrateTime time.Time
}

func newRTLSDR(ctx context.Context) (SDR, error) {
	cmd := exec.CommandContext(ctx, "rtl_tcp", "-a", "127.0.0.1", "-p", "12345")
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
	return &rtlSDR{fpty: fpty, cmd: cmd}, nil
}

func (s *rtlSDR) SetFreqCorrection(ppm uint32) error {
	s.lastPPM = ppm
	return s.SDR.SetFreqCorrection(ppm)
}

func (s *rtlSDR) SetBand(b HzBand) error {
	if s.SDR == nil {
		if err := s.resetConn(); err != nil {
			return err
		}
	}
	if time.Since(s.lastCalibrateTime) > 5*time.Minute {
		if s.lastCalibrateTime.IsZero() {
			if err := s.SDR.SetAGCMode(true); err != nil {
				return err
			}
		}
		s.lastCalibrateTime = time.Now()
		if err := Calibrate(s); err != nil {
			return err
		}
	}
	newFreq, newRate := uint32(b.Center), uint32(b.Width)
	if newFreq == s.lastCenter && newRate == s.lastSampleRate {
		return nil
	}
	if err := s.SetSampleRate(newRate); err != nil {
		return err
	}
	return s.SetCenterFreq(newFreq)
}

func (s *rtlSDR) Info() SDRHWInfo {
	return SDRHWInfo{
		Id:            "Id",
		BitDepth:      8,
		MinHz:         uint64(minFreqHz),
		MaxHz:         uint64(maxFreqHz),
		MaxSampleRate: 2048000,
		CenterHz:      s.lastCenter,
		SampleRate:    s.lastSampleRate,
	}
}

func (s *rtlSDR) Close() error {
	s.stop()
	s.fpty.Close()
	return s.cmd.Wait()
}

func (s *rtlSDR) band() HzBand {
	return HzBand{
		Center: float64(s.lastCenter),
		Width:  float64(s.lastSampleRate),
	}
}

type eofReader struct{}

func (e *eofReader) Read(p []byte) (int, error) { return 0, io.EOF }

func (s *rtlSDR) Reader() *MixerIQReader {
	if s.SDR == nil {
		return NewMixerIQReader(&eofReader{}, s.band())
	}
	return NewMixerIQReader(s.SDR, s.band())
}

func (s *rtlSDR) stop() error {
	if s.SDR == nil {
		return nil
	}
	err := s.SDR.Close()
	s.SDR = nil
	return err
}

func (s *rtlSDR) resetConn() (err error) {
	s.stop()
	s.SDR, err = connect(context.TODO())
	return err
}

func (s *rtlSDR) SetSampleRate(rate uint32) (err error) {
	if err := s.setSampleRate(rate); err != nil {
		return err
	}
	return s.resetConn()
}

func (s *rtlSDR) setSampleRate(rate uint32) error {
	if s.lastSampleRate != rate {
		if err := s.SDR.SetSampleRate(rate); err != nil {
			return err
		}
		s.lastSampleRate = rate
	}
	return nil
}

func (s *rtlSDR) SetCenterFreq(cent uint32) error {
	if err := s.setCenterFreq(cent); err != nil {
		return err
	}
	return s.resetConn()
}

func (s *rtlSDR) setCenterFreq(cent uint32) error {
	if cent < minFreqHz || cent > maxFreqHz {
		return fmt.Errorf("out of range")
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

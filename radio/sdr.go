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

type SDR struct {
	*rtltcp.SDR
	cmd  *exec.Cmd
	fpty *os.File

	lastCenter        uint32
	lastSampleRate    uint32
	lastPPM           uint32
	lastCalibrateTime time.Time
}

func (s SDR) Band() FreqBand     { return FreqBand{Center: s.CenterMHz(), Width: s.MSPS()} }
func (s SDR) CenterMHz() float64 { return float64(s.lastCenter) / 1e6 }
func (s SDR) MSPS() float64      { return float64(s.lastSampleRate) / 1e6 }
func (s SDR) PPM() uint32        { return s.lastPPM }

func (s *SDR) Stop() error {
	if s.SDR == nil {
		return nil
	}
	err := s.SDR.Close()
	s.SDR = nil
	return err
}

func (s *SDR) SetBand(b FreqBand) error {
	if s.SDR == nil {
		if err := s.resetConn(); err != nil {
			return err
		}
	}
	if s.lastPPM == 0 || time.Since(s.lastCalibrateTime) > 5*time.Minute {
		fmt.Println("CALIBRATING!!")
		if s.lastCalibrateTime.IsZero() {
			if err := s.SDR.SetAGCMode(true); err != nil {
				return err
			}
		}
		if err := s.Calibrate(); err != nil {
			return err
		}
	}
	newFreq, newRate := uint32(b.Center*1e6), uint32(b.Width*1e6)
	if newFreq == s.lastCenter && newRate == s.lastSampleRate {
		return nil
	}
	if err := s.SetSampleRate(newRate); err != nil {
		return err
	}
	return s.SetCenterFreq(newFreq)
}

func (s *SDR) SetFreqCorrection(ppm uint32) error {
	s.lastPPM = ppm
	return s.SDR.SetFreqCorrection(ppm)
}

func (s *SDR) resetConn() (err error) {
	s.Stop()
	s.SDR, err = connect(context.TODO())
	return err
}

func (s *SDR) SetSampleRate(rate uint32) (err error) {
	if err := s.setSampleRate(rate); err != nil {
		return err
	}
	return s.resetConn()
}

func (s *SDR) setSampleRate(rate uint32) error {
	if s.lastSampleRate != rate {
		if err := s.SDR.SetSampleRate(rate); err != nil {
			return err
		}
		s.lastSampleRate = rate
	}
	return nil
}

func (s *SDR) SetCenterFreq(cent uint32) error {
	if err := s.setCenterFreq(cent); err != nil {
		return err
	}
	return s.resetConn()
}

func (s *SDR) setCenterFreq(cent uint32) error {
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

func (s *SDR) Calibrate() error {
	for {
		ppm, err := FindPPM(s)
		if err != nil {
			return err
		}
		fmt.Println("measured ppm", ppm)
		if ppm < 1.0 {
			break
		}
		if err := s.SetFreqCorrection(uint32(ppm + 0.5)); err != nil {
			return err
		}
		ppm, err = FindPPM(s)
		if err != nil {
			return err
		}
		fmt.Println("new ppm", ppm)
		if ppm < 2.0 {
			break
		} else if err := s.SetFreqCorrection(0); err != nil {
			return err
		}
	}
	s.lastCalibrateTime = time.Now()
	return nil
}

func NewSDR(ctx context.Context) (*SDR, error) {
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
	return &SDR{fpty: fpty, cmd: cmd}, nil
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

func (s *SDR) Close() error {
	s.Stop()
	s.fpty.Close()
	return s.cmd.Wait()
}

package radio

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/kr/pty"
)

var minFreqHz = uint32(25000000)
var maxFreqHz = uint32(1750000000)
var minRate = uint32(225000)
var maxRate = uint32(3200000)

type rtlSDR struct {
	sdr    *RTLTCPSDR
	cmd    *exec.Cmd
	fpty   *os.File
	ctx    context.Context
	cancel context.CancelFunc

	// device serial number or device index
	serialNumber string

	lastCenter        uint32
	lastSampleRate    uint32
	lastPPM           uint32
	lastCalibrateTime time.Time

	iqr *MixerIQReader
	mu  sync.RWMutex

	addr string
}

// TODO: port pool
const portBase = 12345

var port = 0

func newRTLSDR(ctx context.Context, ser string) (*rtlSDR, error) {
	port++
	p := fmt.Sprint((port % 64) + portBase)
	cctx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(cctx, "rtl_tcp", "-a", "127.0.0.1", "-p", p, "-d", ser, "-s", "240000")
	fpty, err := pty.Start(cmd)
	if err != nil {
		return nil, err
	}
	go io.Copy(os.Stdout, fpty)
	// TODO: would like to wait for 'listening...' but need tty to line-buffer
	select {
	case <-time.After(time.Second):
	case <-cctx.Done():
		return nil, cctx.Err()
	}
	return &rtlSDR{
		fpty:         fpty,
		ctx:          cctx,
		cancel:       cancel,
		cmd:          cmd,
		serialNumber: ser,
		addr:         "127.0.0.1:" + p}, nil
}

func (s *rtlSDR) SetFreqCorrection(ppm uint32) error {
	if err := s.initSDR(); err != nil {
		return err
	}
	s.lastPPM, s.lastCalibrateTime = ppm, time.Now()
	return s.sdr.SetFreqCorrection(ppm)
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
			if err := s.sdr.SetAGCMode(true); err != nil {
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
	s.cancel()
	s.fpty.Close()
	err := s.cmd.Wait()
	return err
}

func (s *rtlSDR) band() HzBand {
	return HzBand{uint64(s.lastCenter), uint64(s.lastSampleRate)}
}

type eofReader struct{}

func (e *eofReader) Read(p []byte) (int, error) { return 0, io.EOF }

func (s *rtlSDR) Reader() *MixerIQReader {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.sdr == nil {
		return NewMixerIQReader(&eofReader{}, s.band())
	} else if s.iqr == nil {
		s.iqr = NewMixerIQReader(s.sdr, s.band())
	}
	return s.iqr
}

func (s *rtlSDR) stop() error {
	if s.sdr == nil {
		return nil
	}
	err := s.sdr.Close()
	s.sdr, s.iqr = nil, nil
	return err
}

func (s *rtlSDR) resetConn() (err error) {
	s.stop()
	s.sdr, err = connect(s.ctx, s.addr)
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
	if err := s.sdr.SetSampleRate(rate); err != nil {
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
		if err := s.sdr.SetCenterFreq(cent); err != nil {
			return err
		}
		s.lastCenter = cent
	}
	return nil
}

func connect(ctx context.Context, addr string) (*RTLTCPSDR, error) {
	var sdr *RTLTCPSDR
	tcpAddr, err := net.ResolveTCPAddr("tcp4", addr)
	if err != nil {
		return nil, err
	}
	for i := 0; i < 10; i++ {
		sdr = &RTLTCPSDR{}
		if err = sdr.Connect(tcpAddr); err != nil {
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
	if s.sdr == nil {
		err = s.resetConn()
	}
	return err
}

func rtlIdxs(ctx context.Context) (idxs []int, err error) {
	cmd := exec.CommandContext(ctx, "rtl_eeprom")
	p, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	defer p.Close()
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	defer cmd.Wait()
	r := bufio.NewReader(p)
	for {
		l, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		if !strings.Contains(l, ":") {
			break
		}
		ss := strings.Split(l, ":")
		if strings.Contains(ss[0], "Found") {
			continue
		}
		v := 0
		if _, err := fmt.Sscanf(ss[0], "%d", &v); err != nil {
			return nil, err
		}
		idxs = append(idxs, v)
	}
	return idxs, err
}

func rtlSDRList(ctx context.Context) (ret []SDRHWInfo, err error) {
	idxs, err := rtlIdxs(ctx)
	if err != nil {
		return nil, err
	}
	// rtl_tcp will print the sn when listing all devices but not rtl_eeprom ?!
	getSerial := func(idx int) (string, error) {
		cmd := exec.CommandContext(ctx, "rtl_eeprom", "-d", fmt.Sprint(idx))
		p, err := cmd.StderrPipe()
		if err != nil {
			return "", err
		}
		defer p.Close()
		if err := cmd.Start(); err != nil {
			return "", err
		}
		defer cmd.Wait()
		r := bufio.NewReader(p)
		for {
			l, err := r.ReadString('\n')
			if err != nil {
				return "", err
			}
			if !strings.Contains(l, "Serial number:") {
				continue
			}
			ss := strings.Split(l, ":")
			if len(ss) != 2 {
				continue
			}
			return strings.TrimSpace(ss[1]), nil
		}
		return "", io.EOF
	}
	for _, v := range idxs {
		s, err := getSerial(v)
		if err == io.EOF {
			log.Println("failed enumerating", v)
			continue
		}
		if err != nil {
			return nil, err
		}
		info := SDRHWInfo{
			Id:            s,
			MinHz:         uint64(minFreqHz),
			MaxHz:         uint64(maxFreqHz),
			MinSampleRate: minRate,
			MaxSampleRate: maxRate,
			SDRFormat: SDRFormat{
				BitDepth:   8,
				CenterHz:   0,
				SampleRate: 0,
			},
		}
		ret = append(ret, info)
	}
	return ret, nil
}

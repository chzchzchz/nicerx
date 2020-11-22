package nicerx

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"strings"

	"github.com/chzchzchz/nicerx/radio"
	"github.com/chzchzchz/nicerx/radio/wav"
	"github.com/chzchzchz/nicerx/sdrproxy"
	"github.com/chzchzchz/nicerx/sdrproxy/client"
)

func OpenOutputS16(path string, hzb radio.HzBand) (io.Writer, func(), error) {
	w, closer, err := openOutput(path)
	if err != nil {
		return nil, nil, err
	}
	if strings.HasSuffix(path, ".wav") {
		ww, err := wav.NewWriter(w, int(hzb.Width), 16, 1)
		if err != nil {
			return nil, nil, err
		}
		newCloser := func() {
			ww.Close()
			closer()
		}
		return ww, newCloser, nil
	}
	return w, closer, err
}

func OpenIQW(path string, hzb radio.HzBand) (*radio.IQWriter, func(), error) {
	w, closer, err := openOutput(path)
	if err != nil {
		return nil, nil, err
	}
	if strings.HasSuffix(path, ".wav") {
		ww, err := wav.NewWriter(w, int(hzb.Width), 8, 2)
		if err != nil {
			return nil, nil, err
		}
		wavCloser := func() {
			ww.Close()
			closer()
		}
		return radio.NewIQWriter(w), wavCloser, nil
	}
	return radio.NewIQWriter(w), closer, nil
}

func openOutput(path string) (io.Writer, func(), error) {
	if path == "-" || path == "-.wav" || path == "-.iq8" {
		return os.Stdout, func() {}, nil
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0644)
	if err != nil {
		return nil, nil, err
	}
	return f, func() { f.Close() }, nil
}

func OpenIQR(path string, hzb radio.HzBand) (*radio.MixerIQReader, func(), error) {
	if u, err := url.Parse(path); err == nil {
		if u.Scheme == "sdr" {
			return openIQRURL(*u, hzb)
		}
	}
	f, closer, err := openInput(path)
	if err != nil {
		return nil, nil, err
	}
	if strings.HasSuffix(path, ".wav") {
		r, err := wav.NewReader(f)
		if err != nil {
			return nil, nil, err
		}
		hzb = radio.HzBand{Center: 0, Width: uint64(r.SampleRate())}
		return radio.NewMixerIQReader(r, hzb), closer, nil
	}
	return radio.NewMixerIQReader(f, hzb), closer, nil
}

func openInput(path string) (io.Reader, func(), error) {
	if path == "-" || path == "-.wav" || path == "-.iq8" {
		return os.Stdin, func() {}, nil
	}
	fin, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	return fin, func() { fin.Close() }, nil
}

func openIQRURL(u url.URL, b radio.HzBand) (*radio.MixerIQReader, func(), error) {
	if u.Path == "" {
		// sdr://device/
		u.Path, u.Host = u.Host, ""
	}
	if u.Host == "" {
		u.Host = "localhost:12000"
	}
	if u.User != nil {
		// sdr://stream@host/
		u.Path = u.User.Username()
	}

	sdrDevice := u.Path
	if sdrDevice == "" {
		return nil, nil, fmt.Errorf("no sdr device defined in url %s", u.String())
	}
	u.Path, u.Scheme = "", "http"
	c := client.New(u)
	log.Printf("opening %s and connected to %s", sdrDevice, u.String())
	cctx, cancel := context.WithCancel(context.Background())
	closer := func() {
		cancel()
		c.Close()
	}

	// Determine current radio tuning and use that.
	if b.Center == 0 && u.User == nil {
		sigs, err := c.Signals(cctx)
		if err != nil {
			return nil, nil, err
		}
		for _, sig := range sigs {
			if sig.Response.Radio.Id == sdrDevice {
				log.Printf("got radio %+v", sig.Response.Radio)
				req := sdrproxy.RxRequest{
					HzBand: sig.Response.Radio.HzBand(),
					Name:   sdrDevice,
					Radio:  sdrDevice,
				}
				iqr, err := c.OpenIQReader(cctx, req)
				if err != nil {
					return nil, nil, err
				}
				return iqr.ToMixer(req.HzBand), closer, nil
			}
		}
		return nil, nil, fmt.Errorf("could not find sdr")
	}

	name := fmt.Sprintf("%s-%d", sdrDevice, b.Center)
	req := sdrproxy.RxRequest{
		HzBand: b,
		Name:   name,
		Radio:  sdrDevice,
	}
	iqr, err := c.OpenIQReader(cctx, req)
	if err != nil {
		return nil, nil, err
	}
	return iqr.ToMixer(req.HzBand), closer, nil
}

package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"

	"github.com/chzchzchz/nicerx/radio"
	"github.com/chzchzchz/nicerx/sdrproxy"
	"github.com/chzchzchz/nicerx/sdrproxy/client"
)

func openInput(inf string) (io.Reader, func(), error) {
	if inf == "-" {
		return os.Stdin, func() {}, nil
	}
	fin, err := os.Open(inf)
	if err != nil {
		return nil, nil, err
	}
	return fin, func() { fin.Close() }, nil
}

func openIQR(path string, hzb radio.HzBand) (*radio.MixerIQReader, func(), error) {
	if u, err := url.Parse(path); err == nil {
		return openIQRURL(*u, hzb)
	}
	f, closer, err := openInput(path)
	if err != nil {
		return nil, nil, err
	}
	return radio.NewMixerIQReader(f, hzb), closer, nil
}

func openIQRURL(u url.URL, b radio.HzBand) (*radio.MixerIQReader, func(), error) {
	if u.Scheme != "sdr" {
		return nil, nil, fmt.Errorf("expected sdr://host:port/device")
	}
	if u.Path == "" {
		u.Path = u.Host
		u.Host = ""
	}
	if u.Host == "" {
		u.Host = "localhost:12000"
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
	if b.Center == 0 {
		sigs, err := c.Signals(cctx)
		if err != nil {
			return nil, nil, err
		}
		for _, sig := range sigs {
			if sig.Response.Radio.Id == sdrDevice {
				log.Printf("got radio %+v", sig.Response.Radio)
				req := sdrproxy.RxRequest{
					HzBand: sig.Response.Radio.HzBand(),
					Name:   "spectrogram-" + sdrDevice,
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

	name := fmt.Sprintf("spectrogram-%s-%d", sdrDevice, b.Center)
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

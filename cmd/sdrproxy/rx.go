package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/chzchzchz/nicerx/dsp"
	"github.com/chzchzchz/nicerx/radio"
)

type rxHandler struct {
	serv *Server
}

func newRXHandler(s *Server) http.Handler { return &rxHandler{s} }

type RxRequest struct {
	radio.HzBand
	// Name is an optional name. I don't use it for anything.
	Name string `json:"name"`
	// Radio is the unique identifier for some radio on the system.
	Radio string `json:"radio"`
}

func newRxRequest(r *http.Request) (*RxRequest, error) {
	b, err := ioutil.ReadAll(r.Body)
	defer r.Body.Close()
	if err != nil {
		return nil, err
	}
	var msg RxRequest
	if err := json.Unmarshal(b, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

type RxResponse struct {
	Format radio.SDRFormat `json:"format"`
	Radio  radio.SDRHWInfo `json:"radio"`
}

// TODO: move into radio code probably
func getSampleRate(wantRate uint32) uint32 {
	rates := []uint32{240000, 960000, 1152000, 1600000, 1800000, 1920000, 2400000}
	for _, v := range rates {
		if wantRate < v {
			return v
		}
	}
	return 3200000
}

func (rxh *rxHandler) handlePost(w http.ResponseWriter, r *http.Request) error {
	req, err := newRxRequest(r)
	if err != nil {
		return err
	}

	sdr, err := radio.NewSDRWithSerial(r.Context(), req.Radio)
	if err != nil {
		return err
	}

	defer sdr.Close()

	// Setup band by choosing rate and filters to get band via SDR bands.
	info, sdrBand := sdr.Info(), req.HzBand
	processSignal := func(ch <-chan []complex64) <-chan []complex64 { return ch }
	sdrBand.Width = uint64(getSampleRate(uint32(sdrBand.Width)))
	if sdrBand.Width != req.HzBand.Width {
		log.Print("upsample to ", sdrBand.Width, " and downsample to ", req.HzBand.Width)
		processSignal = func(ch <-chan []complex64) <-chan []complex64 {
			/* Oversample, lowpass, downsample */
			lpc := dsp.Lowpass(float64(req.HzBand.Width), int(sdrBand.Width), 1, ch)
			resampleRatio := float32(req.HzBand.Width) / float32(sdrBand.Width)
			return dsp.ResampleComplex64(resampleRatio, lpc)
		}
	}
	if err = sdr.SetBand(sdrBand); err != nil {
		return err
	}

	info = sdr.Info()
	dataFormat := radio.SDRFormat{
		BitDepth:   info.BitDepth,
		CenterHz:   req.HzBand.Center,
		SampleRate: uint32(req.HzBand.Width),
	}
	resp := RxResponse{Format: dataFormat, Radio: info}
	respBytes, err := json.Marshal(resp)
	if err != nil {
		return err
	}

	w.Header().Set("Signal", string(respBytes))
	w.Header().Set("Content-Type", "application/octet-stream")

	/* TODO: xlate filter to avoid DC bias */
	sigc := processSignal(sdr.Reader().BatchStream64(r.Context(), int(sdrBand.Width), 0))

	// Stream data.
	iqw := radio.NewIQWriter(w)
	for sig := range sigc {
		if err = iqw.Write64(sig); err != nil {
			log.Printf("sigc error: %v", err)
			break
		}
	}
	return nil
}

func (rxh *rxHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		if err := rxh.handlePost(w, r); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	// JSON listing active RXers on GET.
	// case http.MethodGet:
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

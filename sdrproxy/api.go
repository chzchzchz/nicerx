package sdrproxy

import (
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"

	"github.com/chzchzchz/nicerx/radio"
)

var ErrSignalExists = errors.New("signal by that name exists")
var ErrOutOfRange = errors.New("signal out of range for tuning")

type RxRequest struct {
	radio.HzBand
	// Name is an optional "pretty" name to refer to this channel.
	Name string `json:"name"`
	// Radio is the unique identifier for some radio on the system.
	Radio string `json:"radio"`
	// HintTuneHz is the frequency for tuning the SDR, if possible.
	HintTuneHz uint64 `json:"hint_tune_hz"`
	// HintTuneBw is the samples for the SDR, if possible.
	HintTuneWidthHz uint64 `json:"hint_width_hz"`
}

type RxResponse struct {
	Format radio.SDRFormat `json:"format"`
	Radio  radio.SDRHWInfo `json:"radio"`
}

type RxSignal struct {
	Request  RxRequest
	Response RxResponse
}

func NewRxRequest(rc io.ReadCloser) (*RxRequest, error) {
	b, err := ioutil.ReadAll(rc)
	defer rc.Close()
	if err != nil {
		return nil, err
	}
	var msg RxRequest
	if err := json.Unmarshal(b, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

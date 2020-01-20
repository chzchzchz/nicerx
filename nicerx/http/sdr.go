package http

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/chzchzchz/nicerx/nicerx"
	"github.com/chzchzchz/nicerx/radio"
)

type SDRTune struct {
	Id string `json:"id"`
	radio.HzBand
}

type sdrHandler struct {
	s *nicerx.Server
}

func (s *sdrHandler) handleTune(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	b, err := ioutil.ReadAll(r.Body)
	defer r.Body.Close()
	defer func() {
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}()
	if err != nil {
		return
	}
	var msg SDRTune
	if err = json.Unmarshal(b, &msg); err != nil {
		return
	}
	if err = s.s.SDR.SetBand(msg.HzBand); err != nil {
		return
	}
}

func (s *sdrHandler) handleRaw(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
	w.Header().Set("Content-Type", "binary/octet-stream")
	ctx, cancel := context.WithCancel(context.TODO())
	defer cancel()
	iqw := radio.NewIQWriter(w)
	for samps := range s.s.SDR.Reader().BatchStream64(ctx, 2048, 0) {
		if err := iqw.Write64(samps); err != nil {
			return
		}
	}
}

func (s *sdrHandler) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	js, err := json.Marshal([]radio.SDRHWInfo{s.s.SDR.Info()})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Write(js)
}

func newSDRHandler(s *nicerx.Server) http.Handler {
	sh := sdrHandler{s}
	mux := http.NewServeMux()
	mux.HandleFunc("/tune", sh.handleTune)
	mux.HandleFunc("/raw", sh.handleRaw)
	mux.HandleFunc("/", sh.handleIndex)
	return mux
}

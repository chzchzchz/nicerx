package http

import (
	"encoding/json"
	"net/http"

	"github.com/chzchzchz/nicerx/radio"
	"github.com/chzchzchz/nicerx/sdrproxy/server"
)

type sdrHandler struct {
	serv *server.Server
}

func newSDRHandler(s *server.Server) http.Handler { return &sdrHandler{s} }

func (sh *sdrHandler) handleGet(w http.ResponseWriter, r *http.Request) error {
	sdrs, err := radio.SDRList(r.Context())
	if err != nil {
		return err
	}
	respBytes, err := json.Marshal(sdrs)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(respBytes)
	return err
}

func (sh *sdrHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var err error
	switch r.Method {
	case http.MethodGet:
		err = sh.handleGet(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

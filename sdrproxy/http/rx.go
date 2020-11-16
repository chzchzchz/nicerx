package http

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/chzchzchz/nicerx/radio"
	"github.com/chzchzchz/nicerx/sdrproxy"
	"github.com/chzchzchz/nicerx/sdrproxy/server"
)

type rxHandler struct {
	serv *server.Server
}

func newRXHandler(s *server.Server) http.Handler { return &rxHandler{s} }

func (rxh *rxHandler) handlePost(w http.ResponseWriter, r *http.Request) error {
	req, err := sdrproxy.NewRxRequest(r.Body)
	if err != nil {
		return err
	}

	s, err := rxh.serv.OpenSignal(r.Context(), *req)
	if err != nil {
		return err
	}
	defer s.Close()

	respBytes, err := json.Marshal(s.Response())
	if err != nil {
		return err
	}
	w.Header().Set("Signal", string(respBytes))
	w.Header().Set("Content-Type", "application/octet-stream")

	bw := req.HzBand.Width
	fname := fmt.Sprintf("%v:[%v,%v].iq8", req.HzBand.Center, req.HzBand.Center-bw/2, req.HzBand.Center+bw/2)
	w.Header().Set("Content-Disposition", `inline; filename="`+fname+`"`)

	// Stream out data.
	iqw := radio.NewIQWriter(w)
	log.Printf("[%s] opened stream %+v", r.RemoteAddr, s.Response())
	for sig := range s.Chan() {
		if err = iqw.Write64(sig); err != nil {
			log.Printf("sigc error: %v", err)
			break
		}
	}
	return nil
}

func (rxh *rxHandler) handleGet(w http.ResponseWriter, r *http.Request) error {
	respBytes, err := json.Marshal(rxh.serv.Signals())
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	_, err = w.Write(respBytes)
	return err
}

func (rxh *rxHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var err error

	log.Printf("[%s] request %s %s %+v\n\n", r.RemoteAddr, r.Method, r.URL.String(), r)
	switch r.Method {
	case http.MethodPost:
		err = rxh.handlePost(w, r)
	case http.MethodGet:
		err = rxh.handleGet(w, r)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
	if err != nil {
		log.Printf("[%s] failed %s %s", r.RemoteAddr, err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

}

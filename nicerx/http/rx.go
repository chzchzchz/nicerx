package http

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/chzchzchz/nicerx/nicerx"
	"github.com/chzchzchz/nicerx/nicerx/receiver"
)

type rxHandler struct {
	s *nicerx.Server
}

func streamJSON(rxs *receiver.RxStream) error {
	/*
	enc := json.NewEncoder(w)
	for v := range rs.Chan() {
		if err := enc.Encode(v); err != nil {
			return err
		}
		if _, err := w.Write([]byte{'\n'}); err != nil {
			return err
		}
		w.(http.Flusher).Flush()
	}
	return rs.Err()
	*/
	panic("STUB")
}

func (rxh *rxHandler) handleId(id string, w http.ResponseWriter, r *http.Request) {
	name := r.URL.Path[1:]
	switch r.Method {
	case http.MethodDelete:
		rxh.s.DelRxer(name)
	case http.MethodGet:
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		rs, err := rxh.s.OpenRxer(ctx, name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		err = streamJSON(rs)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func rxerFromRequest(r *http.Request) (*receiver.Rxer, error) {
	b, err := ioutil.ReadAll(r.Body)
	defer r.Body.Close()
	if err != nil {
		return nil, err
	}
	var msg receiver.RxConfigBase
	if err := json.Unmarshal(b, &msg); err != nil {
		return nil, err
	}
	switch msg.TypeName {
	case "scan":
		return &receiver.Rxer{
			RxConfigBase: msg,
			Open:         receiver.NewScan()}, nil
	case "capture":
		return &receiver.Rxer{
			RxConfigBase: msg,
			Open:         receiver.NewCapture(msg.HzBand),
		}, nil
	default:
		return nil, fmt.Errorf("bad type")
	}
}

func (rxh *rxHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		rxh.handleId(r.URL.Path, w, r)
		return
	}
	switch r.Method {
	case http.MethodPost:
		rxer, err := rxerFromRequest(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		rxh.s.AddRxer(rxer)
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		if js, err := json.Marshal(rxh.s.Rxers()); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		} else {
			w.Write(js)
		}
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func newRXHandler(s *nicerx.Server) http.Handler { return &rxHandler{s} }

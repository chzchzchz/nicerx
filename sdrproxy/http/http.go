package http

import (
	"net/http"

	"github.com/chzchzchz/nicerx/sdrproxy/server"
)

func ServeHttp(s *server.Server, serv string) error {
	mux := http.NewServeMux()
	mux.Handle("/api/rx/", http.StripPrefix("/api/rx", newRXHandler(s)))
	// mux.Handle("/api/sdr/", ...) // add/remove/list sdr status
	// mux.Handle("/", newIndexHandler(s))
	return http.ListenAndServe(serv, mux)
}

package http

import (
	"net/http"

	"github.com/chzchzchz/nicerx/sdrproxy/server"
)

func ServeHttp(s *server.Server, serv string) error {
	mux := http.NewServeMux()
	// Add/remove/list rx streams.
	mux.Handle("/api/rx/", http.StripPrefix("/api/rx", newRXHandler(s)))
	// Add/remove/list sdr status.
	mux.Handle("/api/sdr/", http.StripPrefix("/api/sdr", newSDRHandler(s)))
	// mux.Handle("/", newIndexHandler(s))
	return http.ListenAndServe(serv, mux)
}

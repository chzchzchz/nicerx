package http

import (
	"net/http"

	"github.com/chzchzchz/nicerx/nicerx"
)

func ServeHttp(s *nicerx.Server, serv string) error {
	mux := http.NewServeMux()
	mux.Handle("/api/sdr/", http.StripPrefix("/api/sdr", newSDRHandler(s)))
	mux.Handle("/", newIndexHandler(s))
	return http.ListenAndServe(serv, mux)
}

package main

import (
	"flag"
	"log"
	"net/http"
)

func ServeHttp(s *Server, serv string) error {
	mux := http.NewServeMux()
	mux.Handle("/api/rx/", http.StripPrefix("/api/rx", newRXHandler(s)))
	// mux.Handle("/api/sdr/", ...) // add/remove/list sdr status
	// mux.Handle("/", newIndexHandler(s))
	return http.ListenAndServe(serv, mux)
}

var bindServ = flag.String("bind", "localhost:12000", "address to bind server")

func main() {
	flag.Parse()
	log.Printf("listening on %s", *bindServ)
	if err := ServeHttp(NewServer(), *bindServ); err != nil {
		panic(err)
	}
}

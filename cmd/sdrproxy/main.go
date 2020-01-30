package main

import (
	"flag"
	"log"

	"github.com/chzchzchz/nicerx/sdrproxy/http"
	"github.com/chzchzchz/nicerx/sdrproxy/server"
)

var bindServ = flag.String("bind", "localhost:12000", "address to bind server")

func main() {
	flag.Parse()

	log.Printf("listening on %s", *bindServ)
	s := server.NewServer()
	if err := http.ServeHttp(s, *bindServ); err != nil {
		panic(err)
	}
}

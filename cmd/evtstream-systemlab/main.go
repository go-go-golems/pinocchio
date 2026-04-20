package main

import (
	"flag"
	"log"
	"net/http"
)

func main() {
	addr := flag.String("addr", ":8091", "listen address")
	flag.Parse()

	app, err := newSystemlabServer()
	if err != nil {
		log.Fatalf("build systemlab server: %v", err)
	}
	log.Printf("evtstream-systemlab listening on %s", *addr)
	if err := http.ListenAndServe(*addr, app.routes()); err != nil {
		log.Fatalf("listen and serve: %v", err)
	}
}

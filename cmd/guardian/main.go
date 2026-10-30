package main

import (
	"log"
	"net/http"
	"os"

	"github.com/correspondenceadg-cmyk/Xochi/server"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	secret := os.Getenv("TICK_SECRET")
	if secret == "" {
		log.Println("WARNING: TICK_SECRET unset; /v1/tick will reject all requests")
	}
	s := server.New(server.NewMemStore(), server.LogNotifier{}, secret)
	log.Printf("listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, s.Handler()))
}

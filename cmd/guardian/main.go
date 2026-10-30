package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/correspondenceadg-cmyk/Xochi/server"
	"github.com/jackc/pgx/v5/pgxpool"
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

	var st server.Store
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL != "" {
		pool, err := pgxpool.New(context.Background(), dbURL)
		if err != nil {
			log.Fatalf("pgxpool.New: %v", err)
		}
		st = server.NewPgStore(pool)
		log.Println("using postgres store")
	} else {
		st = server.NewMemStore()
		log.Println("using memory store")
	}

	s := server.New(st, server.LogNotifier{}, secret)
	log.Printf("listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, s.Handler()))
}

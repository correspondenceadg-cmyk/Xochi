package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

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

	var n server.Notifier
	brevoKey := os.Getenv("BREVO_API_KEY")
	brevoSender := os.Getenv("BREVO_SENDER_EMAIL")
	if brevoKey != "" {
		n = server.BrevoNotifier{APIKey: brevoKey, Sender: brevoSender}
		log.Println("using Brevo notifier")
	} else {
		n = server.LogNotifier{}
		log.Println("using Log notifier")
	}

	s := server.New(st, n, secret)
	
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      s.Handler(),
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	log.Printf("listening on :%s", port)
	log.Fatal(srv.ListenAndServe())
}

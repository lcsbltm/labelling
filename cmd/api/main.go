package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"

	"labelling/internal/api"
	"labelling/internal/postgres"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	docs := flag.Bool("docs", false, "serve local Swagger UI at /docs")
	flag.Parse()

	err := godotenv.Load(".env")
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Fatal("could not load .env; check file permissions and syntax")
	}

	err = run(*addr, os.Getenv("DATABASE_URL"), *docs)
	if err != nil {
		log.Fatal(err)
	}
}

func run(addr, databaseURL string, docs bool) error {
	if databaseURL == "" {
		return errors.New("DATABASE_URL is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	db, err := postgres.Open(ctx, databaseURL)
	cancel()
	if err != nil {
		return err
	}
	defer db.Close()

	router := api.NewRouter(db)
	if docs {
		api.RegisterDocs(router)
	}

	server := &http.Server{
		Addr:              addr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Printf("API listening on %s", addr)
	return server.ListenAndServe()
}

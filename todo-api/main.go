package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/asdlc/todo-api/internal/handlers"
	"github.com/asdlc/todo-api/internal/middleware"
	"github.com/asdlc/todo-api/internal/models"
	"github.com/asdlc/todo-api/internal/store"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	defaultDataFile = "/data/todos.json"
	defaultPort     = "9090"
)

func main() {
	dataFile := os.Getenv("TODO_DATA_FILE")
	if dataFile == "" {
		dataFile = defaultDataFile
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultPort
	}

	st, err := store.New(dataFile)
	if err != nil {
		log.Fatalf("failed to initialize store: %v", err)
	}
	seedSamples(st)

	h := handlers.New(st)
	mux := http.NewServeMux()
	h.Register(mux)
	mux.Handle("/metrics", promhttp.Handler())

	handler := middleware.Recover(middleware.Logging(mux))

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf(`{"level":"info","msg":"todo-api listening","port":%q}`, port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Println(`{"level":"info","msg":"shutting down"}`)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf(`{"level":"error","msg":"server shutdown error","err":%q}`, err.Error())
	}
}

// seedSamples inserts a few demo todos the first time the store starts empty.
// Deterministic IDs keep local testing reproducible.
func seedSamples(st *store.Store) {
	if len(st.List()) > 0 {
		return
	}
	samples := []models.Todo{
		{ID: "3f1c1b5e-4b3f-4b21-9f35-1e06d6ad7b11", Title: "Buy milk", OwnerID: "demo-user"},
		{ID: "a8b7c6d5-e4f3-4210-b2a3-12345678abcd", Title: "Walk the dog", OwnerID: "demo-user"},
		{ID: "11111111-2222-4333-8444-555555555555", Title: "Read a book", OwnerID: "other-user"},
	}
	for _, t := range samples {
		_ = st.Add(t)
	}
}

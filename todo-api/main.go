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
	"github.com/asdlc/todo-api/internal/store"
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
	log.Printf("todo store initialized at %s", dataFile)

	mux := http.NewServeMux()
	h := handlers.New(st)
	h.Register(mux)

	handler := middleware.Recover(middleware.Logging(mux))

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("todo-api listening on :%s", port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	log.Println("shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("server shutdown error: %v", err)
	}
	log.Println("server stopped")
}

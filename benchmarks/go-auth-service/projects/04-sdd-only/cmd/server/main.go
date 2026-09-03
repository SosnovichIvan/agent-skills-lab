package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"authservice/internal/config"
	"authservice/internal/repository"
	"authservice/internal/service"
	"authservice/internal/token"
	"authservice/internal/transport"
)

func main() {
	cfg, err := config.FromEnvironment()
	if err != nil {
		log.Fatalf("configuration error: %v", err)
	}
	tokens, err := token.NewManager(cfg.Secret, cfg.Issuer, cfg.TokenTTL)
	if err != nil {
		log.Fatalf("token configuration error: %v", err)
	}
	auth := service.New(repository.NewMemory(), tokens)
	server := &http.Server{Addr: cfg.Address, Handler: transport.New(auth, tokens).Handler(), ReadTimeout: cfg.ReadTimeout, WriteTimeout: cfg.WriteTimeout, IdleTimeout: cfg.IdleTimeout, ReadHeaderTimeout: cfg.ReadTimeout}

	serverErrors := make(chan error, 1)
	go func() {
		log.Printf("auth service listening on %s", cfg.Address)
		serverErrors <- server.ListenAndServe()
	}()

	shutdownContext, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	case <-shutdownContext.Done():
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Printf("graceful shutdown failed: %v", err)
			_ = server.Close()
		}
	}
}

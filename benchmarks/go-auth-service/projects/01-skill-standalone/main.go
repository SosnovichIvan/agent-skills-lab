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
)

func main() {
	cfg, e := loadConfig()
	if e != nil {
		log.Fatal(e)
	}
	svc := &authService{newUserRepository(), tokenManager{cfg.Secret, cfg.Issuer, cfg.TokenTTL}}
	srv := &http.Server{Addr: cfg.Addr, Handler: (&api{svc}).routes(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 15}
	stop, stopSignal := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignal()
	go func() {
		log.Printf("authentication service listening on %s", cfg.Addr)
		if e := srv.ListenAndServe(); e != nil && !errors.Is(e, http.ErrServerClosed) {
			log.Printf("server error: %v", e)
			stopSignal()
		}
	}()
	<-stop.Done()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if e := srv.Shutdown(ctx); e != nil {
		log.Printf("graceful shutdown failed: %v", e)
	}
}

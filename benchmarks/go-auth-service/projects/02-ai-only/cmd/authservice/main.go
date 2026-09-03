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

	"authservice/internal/repository"
	"authservice/internal/service"
	"authservice/internal/token"
	"authservice/internal/transport"
)

type config struct {
	Addr   string
	Secret string
	Issuer string
	TTL    time.Duration
}

func loadConfig() (config, error) {
	c := config{Addr: os.Getenv("AUTH_ADDR"), Secret: os.Getenv("AUTH_SECRET"), Issuer: os.Getenv("AUTH_ISSUER")}
	if c.Addr == "" {
		c.Addr = ":8080"
	}
	if len(c.Secret) < 32 {
		return config{}, errors.New("AUTH_SECRET must be at least 32 bytes")
	}
	if c.Issuer == "" {
		return config{}, errors.New("AUTH_ISSUER is required")
	}
	ttlText := os.Getenv("AUTH_TOKEN_TTL")
	if ttlText == "" {
		ttlText = "15m"
	}
	var err error
	c.TTL, err = time.ParseDuration(ttlText)
	if err != nil || c.TTL <= 0 || c.TTL > 24*time.Hour {
		return config{}, errors.New("AUTH_TOKEN_TTL must be a duration between 1ns and 24h")
	}
	return c, nil
}

func main() {
	c, err := loadConfig()
	if err != nil {
		log.Fatal(err)
	}

	users := repository.NewMemory()
	issuer := token.NewIssuer(c.Secret, c.Issuer, c.TTL)
	app := service.New(users, issuer)
	server := &http.Server{
		Addr:              c.Addr,
		Handler:           transport.New(app, issuer),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	go func() {
		log.Printf("auth service listening on %s", c.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown: %v", err)
	}
}

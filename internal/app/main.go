package app

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"tinyURL/internal/config"
	"tinyURL/internal/db"
	hrefrepo "tinyURL/internal/models/hrefRepo"
	userrepo "tinyURL/internal/models/userRepo"
	"tinyURL/internal/services/auth"
	"tinyURL/internal/services/registration"
	"tinyURL/internal/services/shorting"
	transport "tinyURL/internal/transport/http"
)

func Start() error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}

	// Контекст живёт до Ctrl+C или SIGTERM — по нему гасим сервер.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := db.Init(ctx, cfg.DBURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	users := userrepo.NewUserRepo(pool)
	hrefs := hrefrepo.NewHrefRepo(pool)

	authService, err := auth.New(
		users,
		cfg.JWTSecret,
		auth.WithTokenTTL(cfg.TokenTTL),
	)
	if err != nil {
		return err
	}

	registrationService := registration.New(pool, users, hrefs, authService)

	host, err := cfg.Host()
	if err != nil {
		return err
	}

	shortingService := shorting.New(hrefs, host)

	authMiddleware := transport.NewAuth(authService)

	mux := http.NewServeMux()
	transport.NewAuthHandler(authService, registrationService, authMiddleware).Routes(mux)
	transport.NewSlotHandler(shortingService, authMiddleware, cfg.BaseURL).Routes(mux)

	server := &http.Server{
		Addr:              ":" + cfg.ServerPort,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)

	go func() {
		log.Printf("listening on %s", server.Addr)

		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Print("shutting down")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return server.Shutdown(shutdownCtx)
}

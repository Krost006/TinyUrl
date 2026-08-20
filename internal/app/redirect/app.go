// Package redirect собирает и запускает сервис редиректов — второй бинарник
// проекта, рассчитанный на отдельный контейнер.
package redirect

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
	clickrepo "tinyURL/internal/models/clickRepo"
	hrefrepo "tinyURL/internal/models/hrefRepo"
	redirectsvc "tinyURL/internal/services/redirect"
	transport "tinyURL/internal/transport/redirect"
)

func Start() error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := db.Init(ctx, cfg.DBURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	// Обратите внимание, чего здесь нет: userrepo, auth, registration,
	// shorting. Пока список импортов такой, сервис действительно отделим.
	service := redirectsvc.New(
		hrefrepo.NewHrefRepo(pool),
		clickrepo.NewClickRepo(pool),
	)

	mux := http.NewServeMux()
	transport.NewHandler(service).Routes(mux)

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
		log.Printf("redirect listening on %s", server.Addr)

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

package server

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/eu-as/sync-speech/internal/config"
	"github.com/eu-as/sync-speech/internal/ratelimit"
	"github.com/eu-as/sync-speech/internal/room"
)

type App struct {
	rooms     *room.Manager
	ws        *Server
	connLimit *ratelimit.Limiter
	msgLimit  *ratelimit.Limiter
}

func NewApp() *App {
	rooms := room.NewManager()
	return &App{
		rooms:     rooms,
		ws:        New(rooms),
		connLimit: ratelimit.New(10, 5),    // 10/min, burst 5
		msgLimit:  ratelimit.New(100, 20),  // 100/min, burst 20
	}
}

func (a *App) Run() error {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", a.handleHealthz)
	mux.HandleFunc("/s/ws", a.handleWS)
	mux.Handle("/s/", http.StripPrefix("/s/", http.FileServer(http.Dir(config.WebDir))))

	handler := a.recoverMiddleware(a.logMiddleware(a.rateLimitMiddleware(mux)))

	srv := &http.Server{
		Addr:    ":8080",
		Handler: handler,
	}

	go func() {
		slog.Info("sync-pad server starting", "port", config.Port)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	<-sig

	slog.Info("shutting down...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return srv.Shutdown(ctx)
}

func (a *App) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (a *App) handleWS(w http.ResponseWriter, r *http.Request) {
	a.ws.HandleWS(w, r)
}

func (a *App) recoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic recovered", "error", rec, "path", r.URL.Path)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (a *App) logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"ip", ratelimit.ExtractIP(r),
			"duration", time.Since(start),
		)
	})
}

func (a *App) rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := ratelimit.ExtractIP(r)
		if r.URL.Path == "/s/ws" {
			if !a.connLimit.Allow(ip) {
				slog.Warn("rate limit hit (conn)", "ip", ip)
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}
		}
		// Message rate limiting is done per-connection in the WS handler
		next.ServeHTTP(w, r)
	})
}

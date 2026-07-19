package server

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/NoelJudeNoel/sync-pad/internal/config"
	"github.com/NoelJudeNoel/sync-pad/internal/ratelimit"
	"github.com/NoelJudeNoel/sync-pad/internal/room"
	"golang.org/x/time/rate"
)

type App struct {
	cfg       config.Config
	rooms     *room.Manager
	ws        *Server
	connLimit *ratelimit.Limiter
	msgLimit  *ratelimit.Limiter
}

func NewApp(cfg config.Config) *App {
	rooms := room.NewManager(cfg.RoomTTL, cfg.CleanupInterval)
	return &App{
		cfg:       cfg,
		rooms:     rooms,
		ws:        New(rooms, cfg),
		connLimit: ratelimit.New(rate.Limit(cfg.RateLimitConns), cfg.RateLimitConns/2),
		msgLimit:  ratelimit.New(rate.Limit(cfg.RateLimitMessages), cfg.RateLimitMessages/5),
	}
}

func (a *App) Run() error {
	mux := http.NewServeMux()

	base := a.cfg.BasePath
	wsPath := base + "/ws"
	healthPath := "/healthz"

	mux.HandleFunc(healthPath, a.handleHealthz)
	mux.HandleFunc(wsPath, a.handleWS)

	// Static files served under the base path.
	fs := http.StripPrefix(base+"/", http.FileServer(http.Dir(a.cfg.WebDir)))
	mux.Handle(base+"/", fs)

	handler := a.recoverMiddleware(a.logMiddleware(a.rateLimitMiddleware(mux)))

	srv := &http.Server{
		Addr:    a.cfg.Port,
		Handler: handler,
	}

	go func() {
		slog.Info("sync-pad server starting",
			"addr", a.cfg.Port,
			"ws", wsPath,
			"base", base,
			"webdir", a.cfg.WebDir,
		)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
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
	wsPath := a.cfg.BasePath + "/ws"
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := ratelimit.ExtractIP(r)
		if r.URL.Path == wsPath {
			if !a.connLimit.Allow(ip) {
				slog.Warn("rate limit hit (conn)", "ip", ip)
				http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

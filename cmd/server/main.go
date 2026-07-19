package main

import (
	"log/slog"
	"os"

	"github.com/NoelJudeNoel/sync-pad/internal/config"
	"github.com/NoelJudeNoel/sync-pad/internal/server"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg := config.Load()
	slog.Info("loaded config",
		"port", cfg.Port,
		"base", cfg.BasePath,
		"webdir", cfg.WebDir,
		"origins", cfg.AllowedOrigins,
	)

	app := server.NewApp(cfg)
	if err := app.Run(); err != nil {
		slog.Error("server exited", "error", err)
		os.Exit(1)
	}
}

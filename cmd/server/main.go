package main

import (
	"log/slog"
	"os"

	"github.com/eu-as/sync-speech/internal/server"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	app := server.NewApp()
	if err := app.Run(); err != nil {
		slog.Error("server exited", "error", err)
		os.Exit(1)
	}
}

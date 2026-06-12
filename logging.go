package main

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

var appLogger = slog.New(slog.NewTextHandler(os.Stderr, nil))

func configureLogger(format string) error {
	format = strings.TrimSpace(strings.ToLower(format))

	var handler slog.Handler
	switch format {
	case "text":
		handler = slog.NewTextHandler(os.Stderr, nil)
	case "json":
		handler = slog.NewJSONHandler(os.Stderr, nil)
	default:
		return fmt.Errorf("unsupported log format %q", format)
	}

	appLogger = slog.New(handler)
	slog.SetDefault(appLogger)
	return nil
}

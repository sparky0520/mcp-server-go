package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/sparky0520/mcp-server-go/server"
	"github.com/sparky0520/mcp-server-go/tools"
)

func run(ctx context.Context, logger *slog.Logger) error {
	mcpServer := server.New(os.Stdin, os.Stdout, logger)
	tools.RegisterDefaultTools(mcpServer)

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- mcpServer.Run(ctx)
	}()

	select {
	case err := <-serverErrors:
		return fmt.Errorf("MCP server error: %w", err)
	case sig := <-shutdown:
		logger.Info("shutdown signal received", slog.String("signal", sig.String()))
		cancel()
		return nil
	}
}

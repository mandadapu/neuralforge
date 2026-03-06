package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/mandadapu/neuralforge/internal/app"
	"github.com/mandadapu/neuralforge/internal/config"
	"github.com/spf13/cobra"
)

var version = "0.1.0-dev"

func main() {
	root := &cobra.Command{
		Use:   "neuralforge",
		Short: "Autonomous software factory",
	}

	root.AddCommand(serveCmd())
	root.AddCommand(versionCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func serveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the webhook server and worker pool",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.LoadFromEnv()
			if err := cfg.Validate(); err != nil {
				return fmt.Errorf("configuration error: %w", err)
			}

			a, err := app.New(cfg)
			if err != nil {
				return err
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				<-sigCh
				slog.Info("shutting down")
				cancel()
				a.Shutdown(context.Background())
			}()

			return a.Start(ctx)
		},
	}
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("neuralforge %s\n", version)
		},
	}
}

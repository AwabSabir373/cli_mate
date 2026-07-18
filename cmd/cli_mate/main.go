package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	tea "charm.land/bubbletea/v2"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"cli_mate/internal/config"
	"cli_mate/internal/cron"
	"cli_mate/internal/storage"
	"cli_mate/internal/ui"
	"cli_mate/pkg/logger"
)

func main() {
	if err := newRootCommand().ExecuteContext(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	var cfgPath string
	var profileName string
	var workspace string
	var daemonMode bool

	cmd := &cobra.Command{
		Use:   "cli_mate",
		Short: "Terminal-first AI coding agent",
		Long:  "cli_mate is a terminal-first AI coding agent with provider, tool, and storage boundaries designed for production growth.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// If workspace flag provided or env var set, change working directory
			if workspace == "" {
				if envWs := os.Getenv("CLI_MATE_WORKSPACE"); envWs != "" {
					workspace = envWs
				}
			}
			if workspace != "" {
				if err := os.Chdir(workspace); err != nil {
					return fmt.Errorf("could not change to workspace %q: %w", workspace, err)
				}
			}
			cfg, err := config.Load(cfgPath, profileName)
			if err != nil {
				return err
			}

			log := logger.New(cfg.Log.Level)
			log.Info().Str("profile", cfg.ActiveProfile).Msg("starting cli_mate")

			store, err := storage.OpenSQLite(ctx, cfg.Storage.Path)
			if err != nil {
				log.Warn().Err(err).Msg("session storage unavailable")
			} else {
				defer store.Close()
			}

			if daemonMode {
				return runDaemon(ctx, cfg, log)
			}

			ui.BuildVersion = version
			app := ui.NewApp(cfg, store)
			program := tea.NewProgram(app)
			app.SetProgram(program)
			_, err = program.Run()
			return err
		},
	}

	cmd.PersistentFlags().StringVar(&cfgPath, "config", "", "path to config file")
	cmd.PersistentFlags().StringVarP(&profileName, "profile", "p", "default", "profile name")
	cmd.PersistentFlags().StringVar(&workspace, "workspace", "", "path to workspace root (cwd) - overrides current working directory")
	cmd.PersistentFlags().BoolVar(&daemonMode, "daemon", false, "run as a headless cron scheduler daemon")

	cmd.AddCommand(newVersionCommand())
	cmd.AddCommand(newRunCommand())
	cmd.AddCommand(newSessionsCommand())
	cmd.AddCommand(newCronCommand())
	cmd.AddCommand(newMCPServerCommand())

	return cmd
}

func runDaemon(ctx context.Context, cfg *config.Config, log zerolog.Logger) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("home dir: %w", err)
	}
	cronRoot := filepath.Join(home, ".config", "cli_mate", "cron")
	store := cron.NewStore(cronRoot)

	ws, _ := os.Getwd()
	scheduler := cron.NewScheduler(store, cfg, ws, log)

	ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Info().Msg("daemon mode started, waiting for signals")
	scheduler.Start(ctx)
	log.Info().Msg("daemon mode stopped")
	return nil
}

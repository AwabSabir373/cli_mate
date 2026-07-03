package main

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"cli_mate/internal/config"
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

	cmd := &cobra.Command{
		Use:   "cli_mate",
		Short: "Terminal-first AI coding agent",
		Long:  "cli_mate is a terminal-first AI coding agent with provider, tool, and storage boundaries designed for production growth.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
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

			app := ui.NewApp(cfg, store)
			program := tea.NewProgram(app)
			app.SetProgram(program)
			_, err = program.Run()
			return err
		},
	}

	cmd.PersistentFlags().StringVar(&cfgPath, "config", "", "path to config file")
	cmd.PersistentFlags().StringVarP(&profileName, "profile", "p", "default", "profile name")

	cmd.AddCommand(newVersionCommand())
	cmd.AddCommand(newRunCommand())
	cmd.AddCommand(newSessionsCommand())
	cmd.AddCommand(newCronCommand())

	return cmd
}

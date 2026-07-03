package main

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"cli_mate/internal/config"
	"cli_mate/internal/storage"
)

func newSessionsCommand() *cobra.Command {
	var cfgPath string
	var profileName string

	cmd := &cobra.Command{
		Use:   "sessions",
		Short: "Manage chat sessions",
		Long:  "List, view, and manage past chat sessions.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgPath, profileName)
			if err != nil {
				return err
			}

			ctx := context.Background()
			store, err := storage.OpenSQLite(ctx, cfg.Storage.Path)
			if err != nil {
				return fmt.Errorf("open storage: %w", err)
			}
			defer store.Close()

			sessions, err := store.ListSessions(ctx)
			if err != nil {
				return fmt.Errorf("list sessions: %w", err)
			}

			if len(sessions) == 0 {
				fmt.Println("No sessions found.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tTITLE\tCREATED\tUPDATED")
			for _, s := range sessions {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
					s.ID,
					truncate(s.Title, 40),
					s.CreatedAt.Format("2006-01-02 15:04"),
					s.UpdatedAt.Format("2006-01-02 15:04"),
				)
			}
			return w.Flush()
		},
	}

	deleteCmd := &cobra.Command{
		Use:   "delete [id]",
		Short: "Delete a session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgPath, profileName)
			if err != nil {
				return err
			}

			ctx := context.Background()
			store, err := storage.OpenSQLite(ctx, cfg.Storage.Path)
			if err != nil {
				return fmt.Errorf("open storage: %w", err)
			}
			defer store.Close()

			if err := store.DeleteSession(ctx, args[0]); err != nil {
				return fmt.Errorf("delete session: %w", err)
			}
			fmt.Printf("Session %s deleted.\n", args[0])
			return nil
		},
	}

	cmd.AddCommand(deleteCmd)

	cmd.PersistentFlags().StringVar(&cfgPath, "config", "", "path to config file")
	cmd.PersistentFlags().StringVarP(&profileName, "profile", "p", "default", "profile name")

	return cmd
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

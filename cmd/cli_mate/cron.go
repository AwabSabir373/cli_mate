package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"cli_mate/internal/cron"
)

func newCronCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cron",
		Short: "Manage scheduled agent jobs",
		Long:  "Create, list, and manage scheduled agent jobs that run automatically.",
	}

	cmd.AddCommand(newCronAddCommand())
	cmd.AddCommand(newCronListCommand())
	cmd.AddCommand(newCronRemoveCommand())
	cmd.AddCommand(newCronRunCommand())

	return cmd
}

func cronStore() *cron.Store {
	home, _ := os.UserHomeDir()
	root := filepath.Join(home, ".config", "cli_mate", "cron")
	return cron.NewStore(root)
}

func newCronAddCommand() *cobra.Command {
	var expr string
	var model string

	cmd := &cobra.Command{
		Use:   "add [prompt]",
		Short: "Add a new scheduled job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store := cronStore()
			job := cron.Job{
				Expr:   expr,
				Prompt: args[0],
				Model:  model,
			}
			if err := store.Add(job); err != nil {
				return err
			}
			fmt.Printf("Job added: %s (schedule: %s)\n", job.ID, job.Expr)
			return nil
		},
	}

	cmd.Flags().StringVarP(&expr, "schedule", "s", "@daily", "cron expression (e.g., @hourly, @daily, */5 * * * *)")
	cmd.Flags().StringVarP(&model, "model", "m", "", "model override for this job")

	return cmd
}

func newCronListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all scheduled jobs",
		RunE: func(cmd *cobra.Command, args []string) error {
			store := cronStore()
			jobs, err := store.List()
			if err != nil {
				return err
			}
			if len(jobs) == 0 {
				fmt.Println("No scheduled jobs.")
				return nil
			}
			for _, job := range jobs {
				fmt.Println(cron.FormatJob(job))
			}
			return nil
		},
	}
}

func newCronRemoveCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "remove [job-id]",
		Short: "Remove a scheduled job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store := cronStore()
			if err := store.Remove(args[0]); err != nil {
				return err
			}
			fmt.Printf("Job %s removed.\n", args[0])
			return nil
		},
	}
}

func newCronRunCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "run [job-id]",
		Short: "Run a job immediately (without waiting for schedule)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			store := cronStore()
			job, err := store.Get(args[0])
			if err != nil {
				return err
			}
			fmt.Printf("Running job %s: %s\n", job.ID, job.Prompt)
			// In a real implementation, this would invoke the agent loop
			// For now, just print the prompt
			fmt.Println("Note: Full agent execution not yet wired. Job prompt:", job.Prompt)
			return nil
		},
	}
}

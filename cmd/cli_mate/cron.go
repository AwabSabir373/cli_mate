package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"cli_mate/internal/agent"
	"cli_mate/internal/config"
	"cli_mate/internal/cron"
	"cli_mate/internal/providers/registry"
	"cli_mate/internal/tools"
	"cli_mate/pkg/httpclient"
	"cli_mate/pkg/tokenizer"
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
	var cfgPath string
	var profileName string

	cmd := &cobra.Command{
		Use:   "run [job-id]",
		Short: "Run a job immediately (without waiting for schedule)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			store := cronStore()
			job, err := store.Get(args[0])
			if err != nil {
				return err
			}

			cfg, err := config.Load(cfgPath, profileName)
			if err != nil {
				return err
			}

			profile, err := cfg.Active()
			if err != nil {
				return err
			}
			model := profile.Model
			if job.Model != "" {
				model = job.Model
				profile.Model = model
			}

			if err := registry.Validate(profile); err != nil {
				return err
			}

			httpClient := httpclient.New(cfg.HTTP.Timeout, cfg.HTTP.Retries)
			prov, err := registry.New(profile, httpClient)
			if err != nil {
				return err
			}

			workspace := job.Cwd
			if workspace == "" {
				workspace, _ = os.Getwd()
			}
			instructions, _ := agent.LoadInstructions(ctx, workspace)

			toolset := []tools.Tool{
				tools.NewFileReadTool(workspace),
				tools.NewFileEditTool(workspace),
				tools.NewFileWriteTool(workspace),
				tools.NewShellTool(workspace, 45*time.Second),
				tools.NewGlobTool(workspace),
				tools.NewGrepTool(workspace),
				tools.NewFileListTool(workspace),
				tools.NewReadSubtreeTool(workspace),
			}

			counter := tokenizer.New(profile.Model)
			runner := agent.NewCodingRunner(prov, instructions, toolset, workspace)
			runner.EnableSpecialists()

			fmt.Printf("Running job %s: %s\n", job.ID, job.Prompt)
			result, runErr := runner.Run(ctx, agent.RunOptions{
				Model:         profile.Model,
				Prompt:        job.Prompt,
				MaxTokens:     profile.MaxTokens,
				ReserveTokens: profile.ReserveTokens,
				Temperature:   profile.Temperature,
				Counter:       counter,
				DisableTools:  agent.IsConversationalPrompt(job.Prompt),
			})

			// Update job metadata after execution
			job.FireCount++
			job.NextRunAt = cron.NextRunTime(job.Expr, time.Now())
			if updateErr := store.Update(*job); updateErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to update job: %v\n", updateErr)
			}

			if runErr != nil {
				return fmt.Errorf("job %s failed: %w", job.ID, runErr)
			}
			fmt.Println(result.Answer)
			return nil
		},
	}

	cmd.Flags().StringVar(&cfgPath, "config", "", "path to config file")
	cmd.Flags().StringVarP(&profileName, "profile", "p", "default", "profile name")

	return cmd
}

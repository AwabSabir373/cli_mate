package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"cli_mate/internal/agent"
	"cli_mate/internal/config"
	"cli_mate/internal/providers/registry"
	"cli_mate/internal/streamjson"
	"cli_mate/internal/tools"
	"cli_mate/pkg/httpclient"
	"cli_mate/pkg/tokenizer"
)

func newRunCommand() *cobra.Command {
	var cfgPath string
	var profileName string
	var model string
	var provider string
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "run [prompt]",
		Short: "Run a one-shot prompt and print the result",
		Long:  "Send a single prompt to the AI and print the response. Supports stdin piping.",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgPath, profileName)
			if err != nil {
				return err
			}

			// Build prompt from args + stdin
			prompt := strings.Join(args, " ")
			if prompt == "" {
				// Try reading from stdin
				if stat, _ := os.Stdin.Stat(); stat != nil && (stat.Mode()&os.ModeCharDevice) == 0 {
					data, err := io.ReadAll(os.Stdin)
					if err != nil {
						return fmt.Errorf("read stdin: %w", err)
					}
					prompt = strings.TrimSpace(string(data))
				}
			}
			if prompt == "" {
				return fmt.Errorf("no prompt provided. Usage: cli_mate run \"your prompt\" or pipe stdin")
			}

			// Apply flag overrides
			profile, err := cfg.Active()
			if err != nil {
				return err
			}
			if provider != "" {
				profile.Provider = provider
			}
			if model != "" {
				profile.Model = model
			}

			// Validate
			if err := registry.Validate(profile); err != nil {
				return err
			}

			// Connect
			httpClient := httpclient.New(cfg.HTTP.Timeout, cfg.HTTP.Retries)
			prov, err := registry.New(profile, httpClient)
			if err != nil {
				return err
			}

			// Load workspace
			workspace, _ := os.Getwd()
			instructions, _ := agent.LoadInstructions(context.Background(), workspace)

			// Build tools
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

			// Run
			counter := tokenizer.New(profile.Model)
			runner := agent.NewCodingRunner(prov, instructions, toolset, workspace)
			runner.EnableSpecialists()
			result, err := runner.Run(context.Background(), agent.RunOptions{
				Model:         profile.Model,
				Prompt:        prompt,
				MaxTokens:     profile.MaxTokens,
				ReserveTokens: profile.ReserveTokens,
				Temperature:   profile.Temperature,
				Counter:       counter,
				DisableTools:  agent.IsConversationalPrompt(prompt),
			})
			if err != nil {
				return err
			}

			// Output
			switch outputFormat {
			case "json":
				fmt.Printf(`{"answer":%q,"steps":%d,"messages":%d}`+"\n",
					result.Answer, len(result.Steps), len(result.Messages))
			case "stream-json":
				runID := uuid.New().String()
				enc := streamjson.NewEncoder(os.Stdout)
				enc.RunStart(runID, "", workspace, profile.Provider, profile.Model)
				for _, step := range result.Steps {
					enc.Text(runID, step.Text)
				}
				enc.Final(runID, result.Answer)
				enc.RunEnd(runID)
			default:
				if len(result.Steps) > 0 {
					for _, step := range result.Steps {
						fmt.Fprintf(os.Stderr, "[%s] %s\n", step.Kind, step.Text)
					}
					fmt.Fprintln(os.Stderr)
				}
				fmt.Println(result.Answer)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&cfgPath, "config", "", "path to config file")
	cmd.Flags().StringVarP(&profileName, "profile", "p", "default", "profile name")
	cmd.Flags().StringVar(&model, "model", "", "override model")
	cmd.Flags().StringVar(&provider, "provider", "", "override provider")
	cmd.Flags().StringVar(&outputFormat, "output-format", "", "output format: json")

	return cmd
}

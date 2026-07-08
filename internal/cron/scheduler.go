package cron

import (
	"context"
	"os"
	"time"

	"github.com/rs/zerolog"

	"cli_mate/internal/agent"
	"cli_mate/internal/config"
	"cli_mate/internal/providers/registry"
	"cli_mate/internal/tools"
	"cli_mate/pkg/httpclient"
	"cli_mate/pkg/tokenizer"
)

const (
	defaultTickInterval = 30 * time.Second
	maxConcurrentJobs   = 3
)

// Scheduler runs a background daemon loop that checks for due cron jobs
// and executes them via the agent coding runner.
type Scheduler struct {
	store     *Store
	cfg       *config.Config
	workspace string
	log       zerolog.Logger

	ticker  *time.Ticker
	stopCh  chan struct{}
	sem     chan struct{} // bounded concurrency semaphore
	running bool
}

// NewScheduler creates a new Scheduler.
func NewScheduler(store *Store, cfg *config.Config, workspace string, log zerolog.Logger) *Scheduler {
	return &Scheduler{
		store:     store,
		cfg:       cfg,
		workspace: workspace,
		log:       log,
		stopCh:    make(chan struct{}),
		sem:       make(chan struct{}, maxConcurrentJobs),
	}
}

// Start begins the background scheduling loop in a new goroutine.
// It blocks until ctx is cancelled. Call from a goroutine.
func (s *Scheduler) Start(ctx context.Context) {
	s.ticker = time.NewTicker(defaultTickInterval)
	s.running = true
	defer func() {
		s.ticker.Stop()
		s.running = false
	}()

	s.log.Info().Dur("interval", defaultTickInterval).Msg("cron scheduler started")

	// Run an immediate check on start
	s.tick(ctx)

	for {
		select {
		case <-s.ticker.C:
			s.tick(ctx)
		case <-ctx.Done():
			s.log.Info().Err(ctx.Err()).Msg("cron scheduler stopping")
			return
		case <-s.stopCh:
			s.log.Info().Msg("cron scheduler stopped")
			return
		}
	}
}

// Stop signals the scheduler loop to exit.
func (s *Scheduler) Stop() {
	if s.running {
		close(s.stopCh)
	}
}

func (s *Scheduler) tick(ctx context.Context) {
	jobs, err := s.store.DueJobs(time.Now())
	if err != nil {
		s.log.Error().Err(err).Msg("failed to list due jobs")
		return
	}
	if len(jobs) == 0 {
		return
	}

	s.log.Info().Int("count", len(jobs)).Msg("due jobs found")

	for i := range jobs {
		select {
		case <-ctx.Done():
			return
		case s.sem <- struct{}{}:
		}

		job := jobs[i]
		go func(job Job) {
			defer func() { <-s.sem }()
			s.executeJob(ctx, job)
		}(job)
	}
}

func (s *Scheduler) executeJob(ctx context.Context, job Job) {
	log := s.log.With().Str("job_id", job.ID).Str("prompt", truncate(job.Prompt, 80)).Logger()
	log.Info().Msg("executing cron job")

	// Resolve the profile and model for this job.
	profile, err := s.cfg.Active()
	if err != nil {
		log.Error().Err(err).Msg("failed to get active profile")
		return
	}
	model := profile.Model
	if job.Model != "" {
		model = job.Model
		profile.Model = model
	}

	if err := registry.Validate(profile); err != nil {
		log.Error().Err(err).Msg("provider validation failed")
		return
	}

	httpClient := httpclient.New(s.cfg.HTTP.Timeout, s.cfg.HTTP.Retries)
	prov, err := registry.New(profile, httpClient)
	if err != nil {
		log.Error().Err(err).Msg("failed to create provider")
		return
	}

	workspace := job.Cwd
	if workspace == "" {
		workspace = s.workspace
	}
	if workspace == "" {
		var err error
		workspace, err = os.Getwd()
		if err != nil {
			log.Error().Err(err).Msg("failed to get working directory")
			return
		}
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

	execCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	result, runErr := runner.Run(execCtx, agent.RunOptions{
		Model:         profile.Model,
		Prompt:        job.Prompt,
		MaxTokens:     profile.MaxTokens,
		ReserveTokens: profile.ReserveTokens,
		Temperature:   profile.Temperature,
		Counter:       counter,
		DisableTools:  agent.IsConversationalPrompt(job.Prompt),
	})
	cancel()

	// Update job metadata
	job.FireCount++
	job.NextRunAt = NextRunTime(job.Expr, time.Now())
	if err := s.store.Update(job); err != nil {
		log.Error().Err(err).Msg("failed to update job after execution")
	}

	if runErr != nil {
		log.Error().Err(runErr).Str("answer", truncate(result.Answer, 200)).Msg("cron job failed")
	} else {
		log.Info().Str("answer", truncate(result.Answer, 200)).Int("steps", len(result.Steps)).Msg("cron job completed")
	}
}

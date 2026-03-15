package cron

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"wildgecu/homer"

	"github.com/go-co-op/gocron/v2"
)

// Scheduler manages cron jobs using gocron.
type Scheduler struct {
	scheduler gocron.Scheduler
	crons     homer.Homer
	execCfg   *ExecutorConfig
	logger    *slog.Logger
	mu        sync.Mutex
}

// NewScheduler creates a new cron scheduler.
func NewScheduler(crons homer.Homer, execCfg *ExecutorConfig, logger *slog.Logger) (*Scheduler, error) {
	s, err := gocron.NewScheduler(gocron.WithLocation(time.UTC))
	if err != nil {
		return nil, fmt.Errorf("cron: create scheduler: %w", err)
	}
	return &Scheduler{
		scheduler: s,
		crons:     crons,
		execCfg:   execCfg,
		logger:    logger,
	}, nil
}

// LoadAndStart loads all cron jobs from homer and starts the scheduler.
func (s *Scheduler) LoadAndStart(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	jobs, errs := LoadAll(s.crons)
	for _, err := range errs {
		s.logger.Warn("cron load error", "error", err)
	}

	for _, job := range jobs {
		if err := s.addJob(ctx, job); err != nil {
			s.logger.Error("failed to add cron job", "name", job.Name, "error", err)
		}
	}

	s.scheduler.Start()
	s.logger.Info("cron scheduler started", "jobs", len(jobs))
	return nil
}

func (s *Scheduler) addJob(ctx context.Context, job *CronJob) error {
	// Capture job for closure
	j := job
	_, err := s.scheduler.NewJob(
		gocron.CronJob(j.Schedule, false),
		gocron.NewTask(func() {
			Execute(ctx, s.execCfg, j)
		}),
		gocron.WithName(j.Name),
		gocron.WithTags(j.Schedule),
	)
	return err
}

// Reload removes all jobs and re-loads from homer.
func (s *Scheduler) Reload(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove all existing jobs
	for _, j := range s.scheduler.Jobs() {
		if err := s.scheduler.RemoveJob(j.ID()); err != nil {
			s.logger.Warn("failed to remove job", "id", j.ID(), "error", err)
		}
	}

	jobs, errs := LoadAll(s.crons)
	for _, err := range errs {
		s.logger.Warn("cron reload error", "error", err)
	}

	for _, job := range jobs {
		if err := s.addJob(ctx, job); err != nil {
			s.logger.Error("failed to add cron job on reload", "name", job.Name, "error", err)
		}
	}

	s.logger.Info("cron scheduler reloaded", "jobs", len(jobs))
	return nil
}

// JobInfo holds runtime information about a scheduled job.
type JobInfo struct {
	Name     string `json:"name"`
	Schedule string `json:"schedule"`
	NextRun  string `json:"next_run"`
	LastRun  string `json:"last_run,omitempty"`
}

// ListJobs returns runtime info for all scheduled jobs.
func (s *Scheduler) ListJobs() []JobInfo {
	s.mu.Lock()
	defer s.mu.Unlock()

	infos := make([]JobInfo, 0, len(s.scheduler.Jobs()))
	for _, j := range s.scheduler.Jobs() {
		info := JobInfo{
			Name: j.Name(),
		}
		if next, err := j.NextRun(); err == nil {
			info.NextRun = next.Format(time.RFC3339)
		}
		if last, err := j.LastRun(); err == nil && !last.IsZero() {
			info.LastRun = last.Format(time.RFC3339)
		}
		// Extract schedule string from tags if available
		if tags := j.Tags(); len(tags) > 0 {
			info.Schedule = tags[0]
		}
		infos = append(infos, info)
	}
	return infos
}

// JobCount returns the number of registered jobs.
func (s *Scheduler) JobCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.scheduler.Jobs())
}

// Stop shuts down the scheduler.
func (s *Scheduler) Stop() error {
	return s.scheduler.Shutdown()
}

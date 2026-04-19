package cron

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/go-co-op/gocron/v2"
)

// JobStatus classifies a job's runtime state in the scheduler.
type JobStatus string

const (
	StatusRunning   JobStatus = "running"
	StatusSuspended JobStatus = "suspended"
	StatusError     JobStatus = "error"
)

// Scheduler manages cron jobs using gocron.
type Scheduler struct {
	scheduler gocron.Scheduler
	crons     string // path to crons directory
	execCfg   *ExecutorConfig
	logger    *slog.Logger
	mu        sync.Mutex
	entries   []*jobEntry
}

type jobEntry struct {
	name     string
	schedule string
	prompt   string
	status   JobStatus
	errMsg   string
	gJob     gocron.Job
}

// NewScheduler creates a new cron scheduler.
func NewScheduler(crons string, execCfg *ExecutorConfig, logger *slog.Logger) (*Scheduler, error) {
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

// LoadAndStart loads all cron jobs from home and starts the scheduler.
func (s *Scheduler) LoadAndStart(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	running := s.rebuildEntries(ctx)
	s.scheduler.Start()
	s.logger.Info("cron scheduler started", "jobs", running)
	return nil
}

// Reload removes all jobs and re-loads from home.
func (s *Scheduler) Reload(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	running := s.rebuildEntries(ctx)
	s.logger.Info("cron scheduler reloaded", "jobs", running)
	return nil
}

// rebuildEntries drops every registered gocron job and re-classifies the
// markdown directory. Must be called with s.mu held. Returns the count of
// jobs that ended up registered with gocron.
func (s *Scheduler) rebuildEntries(ctx context.Context) int {
	for _, j := range s.scheduler.Jobs() {
		if err := s.scheduler.RemoveJob(j.ID()); err != nil {
			s.logger.Warn("failed to remove job", "id", j.ID(), "error", err)
		}
	}
	s.entries = nil

	running := 0
	for _, r := range LoadAllResults(s.crons) {
		e := &jobEntry{name: r.Name}
		switch {
		case r.Err != nil:
			e.status = StatusError
			e.errMsg = r.Err.Error()
			s.logger.Warn("cron load error", "name", e.name, "error", e.errMsg)
		case r.Job.Suspended:
			e.schedule = r.Job.Schedule
			e.prompt = r.Job.Prompt
			e.status = StatusSuspended
			s.logger.Info("cron job suspended", "name", e.name)
		default:
			e.schedule = r.Job.Schedule
			e.prompt = r.Job.Prompt
			gJob, err := s.addJob(ctx, r.Job)
			if err != nil {
				e.status = StatusError
				e.errMsg = err.Error()
				s.logger.Error("failed to register cron job", "name", e.name, "error", err)
			} else {
				e.status = StatusRunning
				e.gJob = gJob
				running++
			}
		}
		s.entries = append(s.entries, e)
	}
	return running
}

func (s *Scheduler) addJob(ctx context.Context, job *CronJob) (gocron.Job, error) {
	j := job
	gJob, err := s.scheduler.NewJob(
		gocron.CronJob(j.Schedule, false),
		gocron.NewTask(func() {
			Execute(ctx, s.execCfg, j)
		}),
		gocron.WithName(j.Name),
		gocron.WithTags(j.Schedule),
	)
	return gJob, err
}

// JobInfo holds runtime information about a known job (running, suspended, or
// error). Fields are omitted from JSON when empty to keep the wire format lean.
type JobInfo struct {
	Name     string    `json:"name"`
	Schedule string    `json:"schedule,omitempty"`
	Status   JobStatus `json:"status"`
	Prompt   string    `json:"prompt,omitempty"`
	NextRun  string    `json:"next_run,omitempty"`
	LastRun  string    `json:"last_run,omitempty"`
	Error    string    `json:"error,omitempty"`
}

// ListJobs returns the full job state: running, suspended, and error jobs.
func (s *Scheduler) ListJobs() []JobInfo {
	s.mu.Lock()
	defer s.mu.Unlock()

	infos := make([]JobInfo, 0, len(s.entries))
	for _, e := range s.entries {
		info := JobInfo{
			Name:     e.name,
			Schedule: e.schedule,
			Status:   e.status,
			Prompt:   e.prompt,
			Error:    e.errMsg,
		}
		if e.gJob != nil {
			if next, err := e.gJob.NextRun(); err == nil && !next.IsZero() {
				info.NextRun = next.Format(time.RFC3339)
			}
			if last, err := e.gJob.LastRun(); err == nil && !last.IsZero() {
				info.LastRun = last.Format(time.RFC3339)
			}
		}
		infos = append(infos, info)
	}
	return infos
}

// JobCount returns the number of jobs currently registered with gocron
// (i.e. running jobs). Suspended and error jobs are excluded.
func (s *Scheduler) JobCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.scheduler.Jobs())
}

// Stop shuts down the scheduler.
func (s *Scheduler) Stop() error {
	return s.scheduler.Shutdown()
}

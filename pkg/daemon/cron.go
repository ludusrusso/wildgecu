package daemon

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ludusrusso/wildgecu/pkg/cron"
)

// cronSchedulerReloader is the subset of the scheduler the suspend/resume
// handlers depend on. Kept small so tests can stub it.
type cronSchedulerReloader interface {
	Reload(ctx context.Context) error
}

// cronListSource is the subset of the scheduler the cron-list handler needs.
type cronListSource interface {
	ListJobs() []cron.JobInfo
}

// cronListHandler returns an RPC handler that surfaces the scheduler's full
// job state (running, suspended, error) to the CLI.
func cronListHandler(source cronListSource) CommandHandler {
	return func(r *Request) (*Response, error) {
		return &Response{OK: true, Payload: source.ListJobs()}, nil
	}
}

// cronSuspendHandler returns an RPC handler that rewrites the `suspended` field
// of a cron markdown file to true and triggers a scheduler reload.
func cronSuspendHandler(ctx context.Context, cronsDir string, scheduler cronSchedulerReloader) CommandHandler {
	return func(r *Request) (*Response, error) {
		return setSuspended(ctx, cronsDir, scheduler, r, true)
	}
}

// cronResumeHandler returns an RPC handler that rewrites the `suspended` field
// to false and triggers a scheduler reload.
func cronResumeHandler(ctx context.Context, cronsDir string, scheduler cronSchedulerReloader) CommandHandler {
	return func(r *Request) (*Response, error) {
		return setSuspended(ctx, cronsDir, scheduler, r, false)
	}
}

func setSuspended(ctx context.Context, cronsDir string, scheduler cronSchedulerReloader, r *Request, target bool) (*Response, error) {
	name, _ := r.Args["name"].(string)
	if name == "" {
		return &Response{OK: false, Error: "missing name argument"}, nil
	}

	path := filepath.Join(cronsDir, cron.Filename(name))
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &Response{OK: false, Error: fmt.Sprintf("no job named %q", name)}, nil
	}
	if err != nil {
		return &Response{OK: false, Error: fmt.Sprintf("read %s: %v", name, err)}, nil
	}

	job, err := cron.Parse(data)
	if err != nil {
		return &Response{OK: false, Error: fmt.Sprintf("job %q has config errors, fix the file first: %v", name, err)}, nil
	}

	if job.Suspended == target {
		if target {
			return &Response{OK: true, Payload: fmt.Sprintf("%s already suspended, no change", name)}, nil
		}
		return &Response{OK: true, Payload: fmt.Sprintf("%s already running, no change", name)}, nil
	}

	if err := cron.SetFrontmatterField(path, "suspended", target); err != nil {
		return &Response{OK: false, Error: fmt.Sprintf("update %s: %v", name, err)}, nil
	}

	if scheduler != nil {
		if err := scheduler.Reload(ctx); err != nil {
			return &Response{OK: false, Error: fmt.Sprintf("reload: %v", err)}, nil
		}
	}

	if target {
		return &Response{OK: true, Payload: fmt.Sprintf("suspended %q", name)}, nil
	}
	return &Response{OK: true, Payload: fmt.Sprintf("resumed %q", name)}, nil
}

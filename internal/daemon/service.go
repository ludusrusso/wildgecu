package daemon

import (
	"context"
	"log/slog"
	"os"
	"sync"

	"github.com/kardianos/service"
)

type agentService struct {
	cfg    Config
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func (s *agentService) Start(_ service.Service) error {
	ctx, cancel := context.WithCancel(context.Background())
	s.cancel = cancel
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		if err := Run(ctx, s.cfg); err != nil {
			slog.Error("daemon run error", "error", err)
		}
	}()
	return nil
}

func (s *agentService) Stop(_ service.Service) error {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
	return nil
}

func newServiceConfig() *service.Config {
	return &service.Config{
		Name:        "gonesis",
		DisplayName: "Gonesis Agent",
		Description: "Open-source AI agent daemon",
		Executable:  os.Args[0],
	}
}

// InstallService installs the daemon as a system service.
func InstallService(cfg Config) error {
	svc, err := service.New(&agentService{cfg: cfg}, newServiceConfig())
	if err != nil {
		return err
	}
	return svc.Install()
}

// UninstallService removes the system service.
func UninstallService() error {
	svc, err := service.New(&agentService{}, newServiceConfig())
	if err != nil {
		return err
	}
	return svc.Uninstall()
}

// RunAsService runs the daemon under the system service manager.
func RunAsService(cfg Config) error {
	svc, err := service.New(&agentService{cfg: cfg}, newServiceConfig())
	if err != nil {
		return err
	}
	return svc.Run()
}

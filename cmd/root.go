package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"wildgecu/pkg/home"
	"wildgecu/x/config"
	"wildgecu/x/container"

	"github.com/spf13/cobra"
)

// Version, Commit, and Date are set via -ldflags at build time.
var Version = "dev"
var Commit = "none"
var Date = "unknown"

var debugFlag bool
var homeFlag string
var modelFlag string

// appConfig holds the parsed config, set during initConfig.
var appConfig *config.Config

var rootCmd = &cobra.Command{
	Use:   "wildgecu",
	Short: "Wildgecu - an AI-powered coding agent",
	RunE:  runChat,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&homeFlag, "home", "", "override home directory (default: ~/.wildgecu)")
	rootCmd.PersistentFlags().StringVar(&modelFlag, "model", "", "override default model (alias or provider/model)")
	rootCmd.Flags().BoolVar(&debugFlag, "debug", false, "enable debug logging to ~/.wildgecu/debug/<timestamp>.md")
}

// newHome creates a *home.Home rooted at the global home directory.
func newHome() (*home.Home, error) {
	dir, err := config.GlobalHome()
	if err != nil {
		return nil, err
	}
	return home.New(dir)
}

// resolveHomePath normalizes a path, expanding a leading tilde and making it absolute.
func resolveHomePath(path string) (string, error) {
	if strings.HasPrefix(path, "~/") || path == "~" {
		userHome, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("expand ~: %w", err)
		}
		path = filepath.Join(userHome, path[1:])
	}
	return filepath.Abs(path)
}

// newContainer creates a container.Container from the app config.
func newContainer() *container.Container {
	return container.New(appConfig, container.DefaultFactory)
}

func initConfig() {
	if homeFlag != "" {
		resolved, err := resolveHomePath(homeFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: invalid --home path: %v\n", err)
			os.Exit(1)
		}
		homeFlag = resolved
		config.SetGlobalHome(resolved)
	}

	homeDir, err := config.GlobalHome()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: cannot resolve home directory: %v\n", err)
		os.Exit(1)
	}

	if err = config.LoadDotEnv(homeDir); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	}

	cfgPath := filepath.Join(homeDir, "wildgecu.yaml")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if modelFlag != "" {
		if err := cfg.ValidateModelRef(modelFlag); err != nil {
			fmt.Fprintf(os.Stderr, "Error: --model: %v\n", err)
			os.Exit(1)
		}
		cfg.DefaultModel = modelFlag
	}

	appConfig = cfg
}

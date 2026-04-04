package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"wildgecu/pkg/home"
	"wildgecu/x/config"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Version is the build version, settable via -ldflags.
var Version = "dev"

var cfgFile string
var debugFlag bool
var homeFlag string

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
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ./wildgecu.yaml)")
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

	viper.SetDefault("model", "gemini-3-flash-preview")
	viper.SetDefault("gemini_api_key", "")
	viper.SetDefault("google_search", false)
	viper.SetDefault("base_folder", "")

	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigName("wildgecu")
		viper.SetConfigType("yaml")

		if homeDir, err := config.GlobalHome(); err == nil {
			viper.AddConfigPath(homeDir)
		}
	}

	_ = viper.BindEnv("gemini_api_key", "GEMINI_API_KEY")
	viper.AutomaticEnv()

	_ = viper.ReadInConfig()
}


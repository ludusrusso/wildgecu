package cmd

import (
	"fmt"
	"os"

	"gonesis/x/config"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Version is the build version, settable via -ldflags.
var Version = "dev"

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "gonesis",
	Short: "Gonesis - an AI-powered coding agent",
	RunE:  runChat,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ./gonesis.yaml)")
}

func initConfig() {
	viper.SetDefault("model", "gemini-3-flash-preview")
	viper.SetDefault("gemini_api_key", "")
	viper.SetDefault("base_folder", "")

	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.SetConfigName("gonesis")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")

		if home, err := config.GlobalHome(); err == nil {
			viper.AddConfigPath(home)
		}
	}

	viper.BindEnv("gemini_api_key", "GEMINI_API_KEY")
	viper.AutomaticEnv()

	viper.ReadInConfig()
}

func ensureConfigFile() error {
	path, created, err := config.EnsureConfigFile(viper.ConfigFileUsed())
	if err != nil {
		return err
	}
	if created {
		fmt.Printf("Created default config at %s\n", path)
	}
	return nil
}

package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/positronico/kkonf/internal/ui"
	"github.com/positronico/kkonf/internal/version"
	"github.com/spf13/cobra"
)

var (
	configFile  string
	noColor     bool
	showVersion bool
)

var rootCmd = &cobra.Command{
	Use:   "kkonf",
	Short: "kubectl config manager with user consolidation",
	Long: `kkonf is an interactive CLI tool for managing kubectl configuration files.
It provides CRUD operations for clusters, users, and contexts, with a special
focus on consolidating duplicate users to simplify configuration management.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if showVersion {
			fmt.Println(version.Get().Detailed())
			return nil
		}

		if configFile == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to get home directory: %w", err)
			}
			configFile = filepath.Join(home, ".kube", "config")
		}

		if _, err := os.Stat(configFile); os.IsNotExist(err) {
			return fmt.Errorf("config file not found: %s", configFile)
		}

		menu := ui.NewMainMenu(configFile, !noColor)
		return menu.Run()
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.Flags().StringVarP(&configFile, "file", "f", "", "path to kubeconfig file (default: ~/.kube/config)")
	rootCmd.Flags().BoolVar(&noColor, "no-color", false, "disable colored output")
	rootCmd.Flags().BoolVarP(&showVersion, "version", "v", false, "show version information")
}
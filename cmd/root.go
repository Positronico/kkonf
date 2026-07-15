package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/positronico/kkonf/v2/internal/config"
	"github.com/positronico/kkonf/v2/internal/models"
	"github.com/positronico/kkonf/v2/internal/tui"
	"github.com/positronico/kkonf/v2/internal/version"
	"github.com/spf13/cobra"
)

var (
	configFile  string
	showVersion bool
)

var rootCmd = &cobra.Command{
	Use:   "kkonf",
	Short: "kubectl config manager with user consolidation",
	Long: `kkonf is an interactive TUI for managing kubectl configuration files.
It provides CRUD operations for clusters, users, and contexts, with a special
focus on consolidating duplicate users to simplify configuration management.

Run without arguments for the interactive UI, or use the subcommands
(ctx, ns, rename, consolidate, export, backup) for scripting.`,
	// main.go prints the returned error once; without these, cobra would
	// print it a second time plus the full usage block on runtime failures.
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if showVersion {
			fmt.Println(version.Get().Detailed())
			return nil
		}

		configPath, err := resolveConfigPath()
		if err != nil {
			return err
		}
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			fmt.Printf("Config file %s does not exist. Create it? [y/N] ", configPath)
			reader := bufio.NewReader(os.Stdin)
			answer, _ := reader.ReadString('\n')
			if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(answer)), "y") {
				return fmt.Errorf("config file not found: %s", configPath)
			}
			if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
				return fmt.Errorf("failed to create config directory: %w", err)
			}
			if err := os.WriteFile(configPath, []byte("apiVersion: v1\nkind: Config\n"), 0o600); err != nil {
				return fmt.Errorf("failed to create config file: %w", err)
			}
			fmt.Printf("Created empty config at %s\n", configPath)
		}

		return tui.Run(configPath)
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configFile, "file", "f", "", "path to kubeconfig file (default: ~/.kube/config)")
	rootCmd.Flags().BoolVarP(&showVersion, "version", "v", false, "show version information")
}

func resolveConfigPath() (string, error) {
	if configFile != "" {
		return configFile, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory (pass -f explicitly): %w", err)
	}
	return filepath.Join(home, ".kube", "config"), nil
}

// loadKubeconfig loads the resolved config file for non-interactive commands.
func loadKubeconfig() (string, *models.Config, error) {
	path, err := resolveConfigPath()
	if err != nil {
		return "", nil, err
	}
	cfg, err := config.NewLoader(path).Load()
	if err != nil {
		return path, nil, err
	}
	return path, cfg, nil
}

func saveKubeconfig(path string, cfg *models.Config) error {
	return config.NewWriter(path).Save(cfg)
}

package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	exportOutput string
	exportForce  bool
)

var exportCmd = &cobra.Command{
	Use:   "export [context ...]",
	Short: "Export contexts (with their clusters and users) to a new file",
	Long: `Exports the named contexts plus the clusters and users they reference.
Without arguments, exports the entire configuration.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		_, cfg, err := loadKubeconfig()
		if err != nil {
			return err
		}
		if _, err := os.Stat(exportOutput); err == nil && !exportForce {
			return fmt.Errorf("output file %s already exists (use --force to overwrite)", exportOutput)
		}
		subset, err := cfg.ExportSubset(args)
		if err != nil {
			return err
		}
		if err := saveKubeconfig(exportOutput, subset); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Exported %d contexts, %d clusters, %d users to %s\n",
			len(subset.Contexts), len(subset.Clusters), len(subset.Users), exportOutput)
		return nil
	},
}

func init() {
	exportCmd.Flags().StringVarP(&exportOutput, "output", "o", "kubeconfig-export.yaml", "output file path")
	exportCmd.Flags().BoolVar(&exportForce, "force", false, "overwrite the output file if it exists")
	rootCmd.AddCommand(exportCmd)
}

package cmd

import (
	"fmt"

	"github.com/positronico/kkonf/internal/models"
	"github.com/spf13/cobra"
)

var nsCmd = &cobra.Command{
	Use:   "ns [namespace]",
	Short: "Show or set the namespace of the current context",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path, cfg, err := loadKubeconfig()
		if err != nil {
			return err
		}
		if cfg.CurrentContext == "" {
			return fmt.Errorf("no current context set")
		}
		context := cfg.FindContext(cfg.CurrentContext)
		if context == nil {
			return fmt.Errorf("current context %q not found", cfg.CurrentContext)
		}
		if len(args) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), models.DisplayNamespace(context.Context.Namespace))
			return nil
		}
		if err := cfg.SetNamespace(cfg.CurrentContext, args[0]); err != nil {
			return err
		}
		if err := saveKubeconfig(path, cfg); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Namespace set to %q for context %q\n",
			models.DisplayNamespace(context.Context.Namespace), cfg.CurrentContext)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(nsCmd)
}

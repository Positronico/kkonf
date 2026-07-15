package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var ctxCmd = &cobra.Command{
	Use:   "ctx [name]",
	Short: "List contexts, or switch the current context",
	Long: `Without arguments, lists all contexts (the current one marked with *).
With a name, switches the current context and saves the file.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path, cfg, err := loadKubeconfig()
		if err != nil {
			return err
		}
		if len(args) == 0 {
			for _, c := range cfg.Contexts {
				marker := "  "
				if c.Name == cfg.CurrentContext {
					marker = "* "
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s%s\n", marker, c.Name)
			}
			return nil
		}
		if cfg.FindContext(args[0]) == nil {
			return fmt.Errorf("context %q not found", args[0])
		}
		if args[0] == cfg.CurrentContext {
			fmt.Fprintf(cmd.OutOrStdout(), "Already using context %q\n", args[0])
			return nil
		}
		if err := cfg.SetCurrentContext(args[0]); err != nil {
			return err
		}
		if err := saveKubeconfig(path, cfg); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Switched to context %q\n", args[0])
		return nil
	},
}

func init() {
	rootCmd.AddCommand(ctxCmd)
}

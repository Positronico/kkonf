package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var renameCmd = &cobra.Command{
	Use:   "rename (cluster|user|context) OLD NEW",
	Short: "Rename a cluster, user, or context, updating all references",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		kind, oldName, newName := args[0], args[1], args[2]
		path, cfg, err := loadKubeconfig()
		if err != nil {
			return err
		}
		switch kind {
		case "cluster":
			err = cfg.RenameCluster(oldName, newName)
		case "user":
			err = cfg.RenameUser(oldName, newName)
		case "context":
			err = cfg.RenameContext(oldName, newName)
		default:
			return fmt.Errorf("unknown kind %q (want cluster, user, or context)", kind)
		}
		if err != nil {
			return err
		}
		if err := saveKubeconfig(path, cfg); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Renamed %s %q to %q\n", kind, oldName, newName)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(renameCmd)
}

package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/positronico/kkonf/v2/internal/config"
	"github.com/spf13/cobra"
)

var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Manage kubeconfig backups",
}

var backupListCmd = &cobra.Command{
	Use:   "list",
	Short: "List backups of the config file, newest first",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := resolveConfigPath()
		if err != nil {
			return err
		}
		backups, err := config.ListBackups(path)
		if err != nil {
			return err
		}
		if len(backups) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No backups found")
			return nil
		}
		for _, b := range backups {
			line := b
			if info, err := os.Stat(b); err == nil {
				line = fmt.Sprintf("%s  %s", info.ModTime().Format("2006-01-02 15:04:05"), b)
			}
			fmt.Fprintln(cmd.OutOrStdout(), line)
		}
		return nil
	},
}

var backupRestoreCmd = &cobra.Command{
	Use:   "restore [backup-file]",
	Short: "Restore the config file from a backup (latest if omitted)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path, err := resolveConfigPath()
		if err != nil {
			return err
		}
		var backupPath string
		if len(args) == 1 {
			backupPath = args[0]
			if _, err := os.Stat(backupPath); err != nil {
				return fmt.Errorf("backup file not found: %s", backupPath)
			}
		} else {
			backups, err := config.ListBackups(path)
			if err != nil {
				return err
			}
			if len(backups) == 0 {
				return fmt.Errorf("no backups found for %s", path)
			}
			backupPath = backups[0]
		}
		mgr := config.NewBackupManager(path)
		// Preserve the pre-restore state so a restore is itself undoable —
		// and abort if that safety copy cannot be made.
		pre, err := mgr.Create()
		if err != nil {
			return fmt.Errorf("restore aborted, could not back up current state: %w", err)
		}
		if pre != "" {
			fmt.Fprintf(cmd.OutOrStdout(), "Current config backed up to %s\n", filepath.Base(pre))
		}
		if err := mgr.Restore(backupPath); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Restored %s from %s\n", path, filepath.Base(backupPath))
		return nil
	},
}

func init() {
	backupCmd.AddCommand(backupListCmd)
	backupCmd.AddCommand(backupRestoreCmd)
	rootCmd.AddCommand(backupCmd)
}

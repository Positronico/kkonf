package cmd

import (
	"fmt"

	"github.com/positronico/kkonf/internal/models"
	"github.com/spf13/cobra"
)

var consolidateDryRun bool

var consolidateCmd = &cobra.Command{
	Use:   "consolidate",
	Short: "Merge duplicate users (identical auth settings) into one",
	Long: `Finds groups of users with identical definitions and merges each group
into a single user with a suggested name, updating all context references.
Use --dry-run to see what would happen without changing anything.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		path, cfg, err := loadKubeconfig()
		if err != nil {
			return err
		}
		groups := cfg.DuplicateUserGroups()
		if len(groups) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No duplicate users found")
			return nil
		}
		out := cmd.OutOrStdout()
		changed := false
		// Track names claimed within this run so dry-run reports the same
		// names the real run would produce when suggestions collide.
		taken := map[string]bool{}
		for _, group := range groups {
			newName := uniqueConsolidatedName(cfg, group, taken)
			taken[newName] = true
			names := make([]string, len(group.Users))
			for i, u := range group.Users {
				names[i] = u.Name
			}
			if consolidateDryRun {
				fmt.Fprintf(out, "Would consolidate %v into %q (%s auth)\n", names, newName, group.AuthMethod)
				continue
			}
			updated, err := cfg.ConsolidateUsers(names, newName)
			if err != nil {
				return err
			}
			changed = true
			fmt.Fprintf(out, "Consolidated %v into %q (%d contexts updated)\n", names, newName, len(updated))
		}
		if consolidateDryRun || !changed {
			return nil
		}
		if err := saveKubeconfig(path, cfg); err != nil {
			return err
		}
		fmt.Fprintf(out, "Saved %s\n", path)
		return nil
	},
}

// uniqueConsolidatedName suggests a merged name, suffixing it if the
// suggestion collides with an existing user outside the group or with a name
// already claimed earlier in this run.
func uniqueConsolidatedName(cfg *models.Config, group models.DuplicateGroup, taken map[string]bool) string {
	inGroup := make(map[string]bool, len(group.Users))
	for _, u := range group.Users {
		inGroup[u.Name] = true
	}
	base := models.SuggestConsolidatedName(group)
	name := base
	for i := 2; ; i++ {
		existing := cfg.FindUser(name)
		if (existing == nil || inGroup[name]) && !taken[name] {
			return name
		}
		name = fmt.Sprintf("%s-%d", base, i)
	}
}

func init() {
	consolidateCmd.Flags().BoolVar(&consolidateDryRun, "dry-run", false, "show what would be consolidated without saving")
	rootCmd.AddCommand(consolidateCmd)
}

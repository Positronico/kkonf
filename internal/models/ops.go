package models

import (
	"fmt"
	"strings"
)

// Ops are the single mutation surface for a Config. Every front-end (survey
// menus, TUI, subcommands) goes through these so uniqueness checks and
// cross-reference rewriting can't be forgotten at a call site.

func (c *Config) AddCluster(cluster NamedCluster) error {
	if cluster.Name == "" {
		return fmt.Errorf("cluster name cannot be empty")
	}
	if c.FindCluster(cluster.Name) != nil {
		return fmt.Errorf("a cluster named %q already exists", cluster.Name)
	}
	c.Clusters = append(c.Clusters, cluster)
	return nil
}

func (c *Config) AddUser(user NamedUser) error {
	if user.Name == "" {
		return fmt.Errorf("user name cannot be empty")
	}
	if c.FindUser(user.Name) != nil {
		return fmt.Errorf("a user named %q already exists", user.Name)
	}
	c.Users = append(c.Users, user)
	return nil
}

func (c *Config) AddContext(context NamedContext) error {
	if context.Name == "" {
		return fmt.Errorf("context name cannot be empty")
	}
	if c.FindContext(context.Name) != nil {
		return fmt.Errorf("a context named %q already exists", context.Name)
	}
	c.Contexts = append(c.Contexts, context)
	return nil
}

// RenameCluster renames a cluster and rewrites every context referencing it.
func (c *Config) RenameCluster(oldName, newName string) error {
	cluster := c.FindCluster(oldName)
	if cluster == nil {
		return fmt.Errorf("cluster %q not found", oldName)
	}
	if newName == "" {
		return fmt.Errorf("cluster name cannot be empty")
	}
	if newName == oldName {
		return nil
	}
	if c.FindCluster(newName) != nil {
		return fmt.Errorf("a cluster named %q already exists", newName)
	}
	cluster.Name = newName
	for i := range c.Contexts {
		if c.Contexts[i].Context.Cluster == oldName {
			c.Contexts[i].Context.Cluster = newName
		}
	}
	return nil
}

// RenameUser renames a user and rewrites every context referencing it.
func (c *Config) RenameUser(oldName, newName string) error {
	user := c.FindUser(oldName)
	if user == nil {
		return fmt.Errorf("user %q not found", oldName)
	}
	if newName == "" {
		return fmt.Errorf("user name cannot be empty")
	}
	if newName == oldName {
		return nil
	}
	if c.FindUser(newName) != nil {
		return fmt.Errorf("a user named %q already exists", newName)
	}
	user.Name = newName
	for i := range c.Contexts {
		if c.Contexts[i].Context.User == oldName {
			c.Contexts[i].Context.User = newName
		}
	}
	return nil
}

// RenameContext renames a context, following current-context along.
func (c *Config) RenameContext(oldName, newName string) error {
	context := c.FindContext(oldName)
	if context == nil {
		return fmt.Errorf("context %q not found", oldName)
	}
	if newName == "" {
		return fmt.Errorf("context name cannot be empty")
	}
	if newName == oldName {
		return nil
	}
	if c.FindContext(newName) != nil {
		return fmt.Errorf("a context named %q already exists", newName)
	}
	context.Name = newName
	if c.CurrentContext == oldName {
		c.CurrentContext = newName
	}
	return nil
}

// DeleteCluster removes a cluster. With cascade, contexts referencing it are
// removed too (following current-context); without, they are left dangling
// for the caller to have confirmed. Returns the removed context names.
func (c *Config) DeleteCluster(name string, cascade bool) ([]string, error) {
	if c.FindCluster(name) == nil {
		return nil, fmt.Errorf("cluster %q not found", name)
	}
	var removed []string
	if cascade {
		for _, ctxName := range c.GetContextsUsingCluster(name) {
			if c.RemoveContext(ctxName) {
				removed = append(removed, ctxName)
			}
		}
	}
	c.RemoveCluster(name)
	return removed, nil
}

// DeleteUser removes a user, with the same cascade semantics as DeleteCluster.
func (c *Config) DeleteUser(name string, cascade bool) ([]string, error) {
	if c.FindUser(name) == nil {
		return nil, fmt.Errorf("user %q not found", name)
	}
	var removed []string
	if cascade {
		for _, ctxName := range c.GetContextsUsingUser(name) {
			if c.RemoveContext(ctxName) {
				removed = append(removed, ctxName)
			}
		}
	}
	c.RemoveUser(name)
	return removed, nil
}

// DeleteContext removes a context, clearing current-context if it pointed there.
func (c *Config) DeleteContext(name string) error {
	if !c.RemoveContext(name) {
		return fmt.Errorf("context %q not found", name)
	}
	return nil
}

// SetCurrentContext switches the current context, validating the target exists.
func (c *Config) SetCurrentContext(name string) error {
	if c.FindContext(name) == nil {
		return fmt.Errorf("context %q not found", name)
	}
	c.CurrentContext = name
	return nil
}

// NormalizeNamespace maps the literal "default" to "" so both spellings of
// the default namespace persist identically. DisplayNamespace is its inverse.
func NormalizeNamespace(ns string) string {
	if ns == "default" {
		return ""
	}
	return ns
}

func DisplayNamespace(ns string) string {
	if ns == "" {
		return "default"
	}
	return ns
}

// SetNamespace updates a context's namespace (normalized).
func (c *Config) SetNamespace(contextName, namespace string) error {
	context := c.FindContext(contextName)
	if context == nil {
		return fmt.Errorf("context %q not found", contextName)
	}
	context.Context.Namespace = NormalizeNamespace(namespace)
	return nil
}

// DuplicateGroup is a set of users sharing an identical definition.
type DuplicateGroup struct {
	Signature  string
	AuthMethod string
	Users      []NamedUser
}

// DuplicateUserGroups returns duplicate-user groups in first-occurrence order.
func (c *Config) DuplicateUserGroups() []DuplicateGroup {
	bySig := make(map[string][]NamedUser)
	var order []string
	for _, user := range c.Users {
		sig := user.User.GetSignature()
		if _, seen := bySig[sig]; !seen {
			order = append(order, sig)
		}
		bySig[sig] = append(bySig[sig], user)
	}
	var groups []DuplicateGroup
	for _, sig := range order {
		users := bySig[sig]
		if len(users) > 1 {
			groups = append(groups, DuplicateGroup{
				Signature:  sig,
				AuthMethod: users[0].User.GetAuthMethod(),
				Users:      users,
			})
		}
	}
	return groups
}

// ConsolidateUsers merges the named users into a single user called newName —
// either one of the group or a fresh name — rewriting context references.
// The consolidated user keeps the list position of the group's first member.
// Returns the names of contexts whose user reference changed.
func (c *Config) ConsolidateUsers(names []string, newName string) ([]string, error) {
	if len(names) == 0 {
		return nil, fmt.Errorf("no users to consolidate")
	}
	if newName == "" {
		return nil, fmt.Errorf("consolidated user name cannot be empty")
	}
	inGroup := make(map[string]bool, len(names))
	for _, name := range names {
		if c.FindUser(name) == nil {
			return nil, fmt.Errorf("user %q not found", name)
		}
		inGroup[name] = true
	}

	var keep NamedUser
	if existing := c.FindUser(newName); existing != nil {
		if !inGroup[newName] {
			return nil, fmt.Errorf("user %q already exists and is not part of the group", newName)
		}
		keep = *existing
	} else {
		keep = *c.FindUser(names[0])
		keep.Name = newName
	}

	var updated []string
	for i := range c.Contexts {
		if inGroup[c.Contexts[i].Context.User] && c.Contexts[i].Context.User != newName {
			c.Contexts[i].Context.User = newName
			updated = append(updated, c.Contexts[i].Name)
		}
	}

	newUsers := make([]NamedUser, 0, len(c.Users))
	placed := false
	for _, user := range c.Users {
		if inGroup[user.Name] {
			if !placed {
				newUsers = append(newUsers, keep)
				placed = true
			}
			continue
		}
		newUsers = append(newUsers, user)
	}
	c.Users = newUsers
	return updated, nil
}

// SuggestConsolidatedName proposes a merged name for a duplicate-user group.
func SuggestConsolidatedName(group DuplicateGroup) string {
	if group.AuthMethod == "exec" && group.Users[0].User.Exec != nil {
		command := group.Users[0].User.Exec.Command
		switch {
		case strings.Contains(command, "gke-gcloud-auth-plugin"):
			return "gke-user"
		case strings.Contains(command, "aws"):
			return "eks-user"
		case strings.Contains(command, "az"):
			return "aks-user"
		}
		return fmt.Sprintf("%s-user", group.AuthMethod)
	}
	if prefix := commonNamePrefix(group.Users); prefix != "" {
		return prefix + "-user"
	}
	return fmt.Sprintf("%s-user", group.AuthMethod)
}

func commonNamePrefix(users []NamedUser) string {
	if len(users) == 0 {
		return ""
	}
	prefix := users[0].Name
	for _, user := range users[1:] {
		for !strings.HasPrefix(user.Name, prefix) && len(prefix) > 0 {
			prefix = prefix[:len(prefix)-1]
		}
		if prefix == "" {
			break
		}
	}
	return strings.TrimRight(prefix, "-_")
}

// ExportSubset builds a standalone config holding the named contexts plus the
// clusters and users they depend on. With no names, the whole config is
// copied. current-context is kept only if its context is included.
func (c *Config) ExportSubset(contextNames []string) (*Config, error) {
	out := &Config{
		APIVersion:  c.APIVersion,
		Kind:        c.Kind,
		Preferences: c.Preferences,
	}
	if len(contextNames) == 0 {
		out.Clusters = append(out.Clusters, c.Clusters...)
		out.Users = append(out.Users, c.Users...)
		out.Contexts = append(out.Contexts, c.Contexts...)
		out.CurrentContext = c.CurrentContext
		out.Extensions = c.Extensions
		out.Extra = c.Extra
		return out, nil
	}
	for _, name := range contextNames {
		context := c.FindContext(name)
		if context == nil {
			return nil, fmt.Errorf("context %q not found", name)
		}
		if out.FindContext(name) != nil {
			continue
		}
		out.Contexts = append(out.Contexts, *context)
		if cluster := c.FindCluster(context.Context.Cluster); cluster != nil && out.FindCluster(cluster.Name) == nil {
			out.Clusters = append(out.Clusters, *cluster)
		}
		if user := c.FindUser(context.Context.User); user != nil && out.FindUser(user.Name) == nil {
			out.Users = append(out.Users, *user)
		}
		if name == c.CurrentContext {
			out.CurrentContext = name
		}
	}
	return out, nil
}

// MergeAction is what to do with one conflicting item during Merge.
type MergeAction string

const (
	MergeSkip    MergeAction = "skip"
	MergeReplace MergeAction = "replace"
	MergeRename  MergeAction = "rename"
)

type MergeOptions struct {
	// OnConflict decides per conflicting item; nil means skip all conflicts.
	// kind is "cluster", "user", or "context".
	OnConflict func(kind, name string) MergeAction
	// Rename supplies the new name for MergeRename decisions. Returning ""
	// (or a name that itself conflicts) skips the item. nil skips all renames.
	Rename func(kind, oldName string) string
}

type MergeResult struct {
	Added, Replaced, Renamed, Skipped int
}

func (r MergeResult) Total() int { return r.Added + r.Replaced + r.Renamed }

// Merge imports another config into c. Renamed clusters/users have their
// references inside the imported config rewritten *before* its contexts are
// merged, so imported contexts follow their renamed dependencies. The
// imported config is consumed and must not be reused afterwards.
func (c *Config) Merge(imported *Config, opts MergeOptions) MergeResult {
	onConflict := opts.OnConflict
	if onConflict == nil {
		onConflict = func(string, string) MergeAction { return MergeSkip }
	}
	rename := func(kind, oldName string) string {
		if opts.Rename == nil {
			return ""
		}
		return opts.Rename(kind, oldName)
	}

	var res MergeResult

	for idx := range imported.Clusters {
		cluster := imported.Clusters[idx]
		if c.FindCluster(cluster.Name) == nil {
			c.Clusters = append(c.Clusters, cluster)
			res.Added++
			continue
		}
		switch onConflict("cluster", cluster.Name) {
		case MergeReplace:
			*c.FindCluster(cluster.Name) = cluster
			res.Replaced++
		case MergeRename:
			oldName := cluster.Name
			newName := rename("cluster", oldName)
			if newName == "" || c.FindCluster(newName) != nil {
				res.Skipped++
				continue
			}
			cluster.Name = newName
			c.Clusters = append(c.Clusters, cluster)
			for i := range imported.Contexts {
				if imported.Contexts[i].Context.Cluster == oldName {
					imported.Contexts[i].Context.Cluster = newName
				}
			}
			res.Renamed++
		default:
			res.Skipped++
		}
	}

	for idx := range imported.Users {
		user := imported.Users[idx]
		if c.FindUser(user.Name) == nil {
			c.Users = append(c.Users, user)
			res.Added++
			continue
		}
		switch onConflict("user", user.Name) {
		case MergeReplace:
			*c.FindUser(user.Name) = user
			res.Replaced++
		case MergeRename:
			oldName := user.Name
			newName := rename("user", oldName)
			if newName == "" || c.FindUser(newName) != nil {
				res.Skipped++
				continue
			}
			user.Name = newName
			c.Users = append(c.Users, user)
			for i := range imported.Contexts {
				if imported.Contexts[i].Context.User == oldName {
					imported.Contexts[i].Context.User = newName
				}
			}
			res.Renamed++
		default:
			res.Skipped++
		}
	}

	for idx := range imported.Contexts {
		context := imported.Contexts[idx]
		if c.FindContext(context.Name) == nil {
			c.Contexts = append(c.Contexts, context)
			res.Added++
			continue
		}
		switch onConflict("context", context.Name) {
		case MergeReplace:
			*c.FindContext(context.Name) = context
			res.Replaced++
		case MergeRename:
			newName := rename("context", context.Name)
			if newName == "" || c.FindContext(newName) != nil {
				res.Skipped++
				continue
			}
			context.Name = newName
			c.Contexts = append(c.Contexts, context)
			res.Renamed++
		default:
			res.Skipped++
		}
	}

	return res
}

package cmd

import (
	"os"

	"github.com/pankaj/claude-context/internal/config"
	"github.com/spf13/cobra"
)

// activeTicketIDs returns IDs of all active tickets for shell completion.
func activeTicketIDs(dataDir string) []string {
	cfgMgr := config.NewManager(dataDir)
	cfg, err := cfgMgr.Load()
	if err != nil {
		return nil
	}
	ids := make([]string, 0, len(cfg.Tickets.Active))
	for _, t := range cfg.Tickets.Active {
		ids = append(ids, t.TicketID)
	}
	return ids
}

// allTicketIDs returns IDs of active + archived tickets.
func allTicketIDs(dataDir string) []string {
	cfgMgr := config.NewManager(dataDir)
	cfg, err := cfgMgr.Load()
	if err != nil {
		return nil
	}
	ids := make([]string, 0, len(cfg.Tickets.Active)+len(cfg.Tickets.Archived))
	for _, t := range cfg.Tickets.Active {
		ids = append(ids, t.TicketID)
	}
	for _, t := range cfg.Tickets.Archived {
		ids = append(ids, t.TicketID)
	}
	return ids
}

// projectNames returns all managed project context names.
func projectNames(dataDir string) []string {
	cfgMgr := config.NewManager(dataDir)
	cfg, err := cfgMgr.Load()
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(cfg.ManagedProjects))
	for _, p := range cfg.ManagedProjects {
		names = append(names, p.ContextName)
	}
	return names
}

// globalContextNames returns all global context names.
func globalContextNames(dataDir string) []string {
	cfgMgr := config.NewManager(dataDir)
	cfg, err := cfgMgr.Load()
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(cfg.GlobalContexts))
	for _, g := range cfg.GlobalContexts {
		names = append(names, g.Name)
	}
	return names
}

func resolveDataDir() string {
	if dataDir != "" {
		return dataDir
	}
	if env := os.Getenv("CCTX_DATA_DIR"); env != "" {
		return env
	}
	home, _ := os.UserHomeDir()
	return home + "/.cctx"
}

// completeActiveTickets is a ValidArgsFunction for commands taking an active ticket ID.
func completeActiveTickets(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return activeTicketIDs(resolveDataDir()), cobra.ShellCompDirectiveNoFileComp
}

// completeAllTickets is a ValidArgsFunction for commands that accept any ticket ID.
func completeAllTickets(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return allTicketIDs(resolveDataDir()), cobra.ShellCompDirectiveNoFileComp
}

// completeProjects is a ValidArgsFunction for commands taking a project name.
func completeProjects(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return projectNames(resolveDataDir()), cobra.ShellCompDirectiveNoFileComp
}

// completeTicketThenProjects completes ticket ID for arg[0], projects for subsequent args.
func completeTicketThenProjects(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	dd := resolveDataDir()
	if len(args) == 0 {
		return activeTicketIDs(dd), cobra.ShellCompDirectiveNoFileComp
	}
	return projectNames(dd), cobra.ShellCompDirectiveNoFileComp
}

// completeGlobalContexts is a ValidArgsFunction for global sub-commands taking a context name.
func completeGlobalContexts(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return globalContextNames(resolveDataDir()), cobra.ShellCompDirectiveNoFileComp
}

// completeGlobalThenProjects completes global name for arg[0], projects for subsequent args.
func completeGlobalThenProjects(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	dd := resolveDataDir()
	if len(args) == 0 {
		return globalContextNames(dd), cobra.ShellCompDirectiveNoFileComp
	}
	return projectNames(dd), cobra.ShellCompDirectiveNoFileComp
}

func init() {
	registerCompletions()
}

func registerCompletions() {
	// ticket sub-commands
	ticketShowCmd.ValidArgsFunction = completeAllTickets
	ticketCompleteCmd.ValidArgsFunction = completeActiveTickets
	ticketAbandonCmd.ValidArgsFunction = completeActiveTickets
	ticketArchiveCmd.ValidArgsFunction = completeActiveTickets
	ticketEditCmd.ValidArgsFunction = completeAllTickets
	ticketDeleteCmd.ValidArgsFunction = completeAllTickets
	ticketLinkCmd.ValidArgsFunction = completeTicketThenProjects
	ticketUnlinkCmd.ValidArgsFunction = completeTicketThenProjects

	// global sub-commands
	globalEnableCmd.ValidArgsFunction = completeGlobalContexts
	globalDisableCmd.ValidArgsFunction = completeGlobalContexts
	globalRemoveCmd.ValidArgsFunction = completeGlobalContexts
	globalShowCmd.ValidArgsFunction = completeGlobalContexts
	globalLinkCmd.ValidArgsFunction = completeGlobalThenProjects
	globalUnlinkCmd.ValidArgsFunction = completeGlobalThenProjects
	globalTemplatesShowCmd.ValidArgsFunction = completeTemplateNames
	globalTemplatesResetCmd.ValidArgsFunction = completeTemplateNames

	// top-level commands
	unlinkCmd.ValidArgsFunction = completeProjects
}

// completeTemplateNames lists available template names (embedded + user overrides).
func completeTemplateNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	// Known embedded templates
	names := []string{"ticket", "global", "script", "python"}
	// Also scan ~/.cctx/templates/ for user overrides
	dd := resolveDataDir()
	entries, err := os.ReadDir(dd + "/templates")
	if err == nil {
		seen := map[string]bool{}
		for _, n := range names {
			seen[n] = true
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			n := e.Name()
			if len(n) > 3 && n[len(n)-3:] == ".md" {
				stem := n[:len(n)-3]
				if !seen[stem] {
					names = append(names, stem)
					seen[stem] = true
				}
			}
		}
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

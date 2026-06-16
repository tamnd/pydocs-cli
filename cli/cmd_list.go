package cli

import (
	"github.com/spf13/cobra"
)

func (a *App) listCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all Python standard library modules",
		RunE: func(cmd *cobra.Command, _ []string) error {
			limit := a.effectiveLimit(50)
			a.progressf("fetching module index...")
			mods, err := a.client.List(cmd.Context(), limit)
			if err != nil {
				return mapFetchErr(err)
			}
			return a.renderOrEmpty(mods, len(mods))
		},
	}
}

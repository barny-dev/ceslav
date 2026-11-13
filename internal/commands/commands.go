package commands

import (
	"github.com/barny-dev/ceslav/internal/commands/projection"
	"github.com/barny-dev/ceslav/internal/commands/rowcount"
	"github.com/barny-dev/ceslav/internal/commands/rows"
	"github.com/barny-dev/ceslav/internal/commands/sort"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "ceslav",
		Short:         "Count, sort and transform CSV records",
		Long:          "Read CSV from files or standard input; count and sort records, select or drop columns, and select, drop, reorder or reverse rows.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE:          func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}
	cmd.AddCommand(rowcount.Cmd(), sort.Cmd(), projection.SelectCmd(), projection.DropCmd(), rows.Cmd())
	return cmd
}

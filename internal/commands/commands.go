package commands

import (
	"github.com/barny-dev/ceslav/internal/commands/rowcount"
	"github.com/barny-dev/ceslav/internal/commands/sort"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "ceslav",
		Short:         "Count and sort CSV records",
		Long:          "Read CSV from files or standard input, count records, and sort by string or numeric columns.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE:          func(cmd *cobra.Command, _ []string) error { return cmd.Help() },
	}
	cmd.AddCommand(rowcount.Cmd(), sort.Cmd())
	return cmd
}

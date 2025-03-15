package commands

import (
	"fmt"
	"github.com/barny-dev/ceslav/internal/commands/rowcount"
	"github.com/barny-dev/ceslav/internal/commands/sort"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	cmd := root()
	cmd.AddCommand(rowcount.Cmd())
	cmd.AddCommand(sort.Cmd())
	return cmd
}

func root() *cobra.Command {
	return &cobra.Command{
		Use:   "ceslav",
		Short: "a csv tool",
		Long: `a csv tool that currently supports:
 * no functionality whatsoever`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Hello, World!")
		},
	}
}

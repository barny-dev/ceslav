package flags

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"os"
)

func AddInputFileFlag(fs *pflag.FlagSet) {
	fs.StringP("input", "i", "-", "csv input file ('-' for stdin) ")
}

func GetInputFileFlag(fs *pflag.FlagSet) string {
	val, err := fs.GetString("input")
	if err != nil {
		panic("error while getting 'input' flag")
	}
	return val
}

func AddOutputFileFlag(fs *pflag.FlagSet) {
	fs.StringP("output", "o", "-", "csv output file ('-' for stdout)")
}

func GetOutputFileFlag(fs *pflag.FlagSet) string {
	val, err := fs.GetString("output")
	if err != nil {
		panic("error while getting 'output' flag")
	}
	return val
}

func AddHeaderRowFlag(fs *pflag.FlagSet) {
	fs.BoolP("header", "j", false, "expect header row on input")
}

func GetHeaderRowFlag(fs *pflag.FlagSet) bool {
	val, err := fs.GetBool("header")
	if err != nil {
		panic("error while getting 'header' flag")
	}
	return val
}

func AddOutputHeaderRowFlag(fs *pflag.FlagSet) {
	fs.BoolP("output-header", "k", false, "expect header row on output")
}

func GetOutputHeaderRowFlag(fs *pflag.FlagSet) bool {
	val, err := fs.GetBool("output-header")
	if err != nil {
		panic("error while getting 'output-header' flag")
	}
	return val
}

func HasHeader(cmd *cobra.Command) bool {
	return GetHeaderRowFlag(cmd.Flags())
}

func HasOutputHeader(cmd *cobra.Command) bool {
	return GetOutputHeaderRowFlag(cmd.Flags())
}

func GetInput(cmd *cobra.Command) (*os.File, error) {
	fileName := GetInputFileFlag(cmd.Flags())
	if fileName == "-" {
		return os.Stdin, nil
	}

	file, openErr := os.OpenFile(fileName, os.O_RDONLY, 0)
	if openErr != nil {
		return nil, fmt.Errorf("could not open input file %s, %w", fileName, openErr)
	}

	return file, nil
}

func GetOutput(cmd *cobra.Command) (*os.File, error) {
	fileName := GetOutputFileFlag(cmd.Flags())
	if fileName == "-" {
		return os.Stdout, nil
	}

	file, openErr := os.OpenFile(fileName, os.O_CREATE|os.O_TRUNC, 0)
	if openErr != nil {
		return nil, fmt.Errorf("could not open output file %s, %w", fileName, openErr)
	}

	return file, nil
}

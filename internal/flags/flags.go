package flags

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func AddInputFileFlag(fs *pflag.FlagSet) {
	fs.StringP("input", "i", "-", "CSV input file ('-' for stdin)")
}

func GetInputFileFlag(fs *pflag.FlagSet) string {
	val, err := fs.GetString("input")
	if err != nil {
		panic("error while getting 'input' flag")
	}
	return val
}

func AddOutputFileFlag(fs *pflag.FlagSet) {
	fs.StringP("output", "o", "-", "CSV output file ('-' for stdout)")
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
	fs.BoolP("output-header", "k", false, "write a header row on output")
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

// GetOutput opens without truncating first, so aliases of the input can be
// checked using open file identities (including hard links and symlinks).
func GetOutput(cmd *cobra.Command, input *os.File) (*os.File, error) {
	fileName := GetOutputFileFlag(cmd.Flags())
	if fileName == "-" {
		return os.Stdout, nil
	}
	file, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return nil, fmt.Errorf("open output %q: %w", fileName, err)
	}
	fail := func(err error) (*os.File, error) {
		return nil, errors.Join(err, file.Close())
	}
	inputInfo, err := input.Stat()
	if err != nil {
		return fail(fmt.Errorf("stat input: %w", err))
	}
	outputInfo, err := file.Stat()
	if err != nil {
		return fail(fmt.Errorf("stat output: %w", err))
	}
	if os.SameFile(inputInfo, outputInfo) {
		return fail(errors.New("input and output refer to the same file"))
	}
	// Pipes and devices do not support Truncate; opening them already positions
	// the stream correctly. Only regular files need their previous data removed.
	if outputInfo.Mode().IsRegular() {
		if err := file.Truncate(0); err != nil {
			return fail(fmt.Errorf("truncate output %q: %w", fileName, err))
		}
	}
	return file, nil
}

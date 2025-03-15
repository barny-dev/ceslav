package rowcount

import (
	"errors"
	"github.com/barny-dev/ceslav/internal/flags"
	"github.com/barny-dev/ceslav/internal/utilities/processor"
	"github.com/barny-dev/ceslav/internal/utilities/run"
	"github.com/spf13/cobra"
	"log"
	_ "os"
	"strconv"
)

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:  "row-count [FLAGS]",
		RunE: runCmd,
	}
	fs := cmd.Flags()
	flags.AddInputFileFlag(fs)
	flags.AddOutputFileFlag(fs)
	flags.AddHeaderRowFlag(fs)
	flags.AddOutputHeaderRowFlag(fs)
	return cmd
}

func runCmd(cmd *cobra.Command, _ []string) error {
	log.Printf("run: rowcount\n")
	return run.RunOneToOneCommand(cmd, NewProcessor)
}

const HEADER_NOT_EXPECTED = 0
const HEADER_EXPECTED = 1
const HEADER_READ = 2

type Processor struct {
	headerStatus int
	outputHeader bool
	count        uint
	sink         processor.Sink
}

func NewProcessor(cmd *cobra.Command, sink processor.Sink) (processor.Processor, error) {
	var headerStatus int
	if flags.HasHeader(cmd) {
		headerStatus = HEADER_EXPECTED
	} else {
		headerStatus = HEADER_NOT_EXPECTED
	}

	return &Processor{
		headerStatus: headerStatus,
		outputHeader: flags.HasOutputHeader(cmd),
		count:        0,
		sink:         sink,
	}, nil
}

func (proc *Processor) ProcessRow(_ []string) (processor.Terminate, error) {
	if proc.headerStatus == HEADER_EXPECTED {
		return true, errors.New("invalid state: header expected")
	}
	proc.count++
	return false, nil
}

func (proc *Processor) ProcessHeader(_ []string) (processor.Terminate, error) {
	if proc.headerStatus == HEADER_NOT_EXPECTED {
		return true, errors.New("configuration error: header not expected")
	}
	if proc.headerStatus == HEADER_EXPECTED {
		proc.headerStatus = HEADER_READ
		return false, nil
	}
	if proc.headerStatus == HEADER_READ {
		return true, errors.New("invalid state: duplicate header read")
	}
	return true, errors.New("invalid state: unknown header status")
}

func (proc *Processor) Complete() error {
	if proc.headerStatus == HEADER_EXPECTED {
		return errors.New("invalid input: no header")
	}
	if proc.outputHeader {
		err := proc.sink([]string{"row count"})
		if err != nil {
			return err
		}
	}
	rowCountValue := strconv.FormatUint(uint64(proc.count), 10)
	err := proc.sink([]string{rowCountValue})
	return err
}

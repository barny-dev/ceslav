package rows

import (
	"errors"

	"github.com/barny-dev/ceslav/internal/flags"
	"github.com/barny-dev/ceslav/internal/utilities/processor"
	"github.com/barny-dev/ceslav/internal/utilities/run"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "rows [flags]", Short: "Keep or drop the first or last N CSV data records", Args: cobra.NoArgs,
		Long: "Choose exactly one of --first N and --last N. Add --drop to remove those records instead of keeping them. Headers are excluded from N. Keeping zero records stops before reading data; --last uses at most N buffered records.",
		RunE: func(cmd *cobra.Command, _ []string) error { return run.RunOneToOneCommand(cmd, NewProcessor) },
	}
	fs := cmd.Flags()
	flags.AddInputFileFlag(fs)
	flags.AddOutputFileFlag(fs)
	flags.AddHeaderRowFlag(fs)
	flags.AddOutputHeaderRowFlag(fs)
	fs.Int("first", 0, "number of data records from the start")
	fs.Int("last", 0, "number of data records from the end")
	fs.Bool("drop", false, "remove the chosen records instead of retaining them")
	return cmd
}

type Processor struct {
	n, remaining                                          int
	fromEnd, drop, expectHeader, outputHeader, headerRead bool
	buffer                                                [][]string
	next                                                  int
	sink                                                  processor.Sink
}

func NewProcessor(cmd *cobra.Command, sink processor.Sink) (processor.Processor, error) {
	fs := cmd.Flags()
	if fs.Changed("first") == fs.Changed("last") {
		return nil, errors.New("specify exactly one of --first and --last")
	}
	fromEnd := fs.Changed("last")
	flag := "first"
	if fromEnd {
		flag = "last"
	}
	n, err := fs.GetInt(flag)
	if err != nil {
		return nil, err
	}
	if n < 0 {
		return nil, errors.New("row count must be nonnegative")
	}
	drop, err := fs.GetBool("drop")
	if err != nil {
		return nil, err
	}
	proc := &Processor{n: n, remaining: n, fromEnd: fromEnd, drop: drop, expectHeader: flags.HasHeader(cmd), outputHeader: flags.HasOutputHeader(cmd), sink: sink}
	if proc.outputHeader && !proc.expectHeader {
		return nil, errors.New("--output-header requires --header")
	}
	return proc, nil
}

func (proc *Processor) keepZero() bool { return !proc.drop && proc.n == 0 }

func (proc *Processor) Start() (processor.Terminate, error) {
	// With headers, ProcessHeader performs the same stop after reading only
	// the header. Otherwise no input record needs to be read at all.
	return proc.keepZero() && !proc.expectHeader, nil
}

func (proc *Processor) ProcessHeader(header []string) (processor.Terminate, error) {
	if !proc.expectHeader {
		return true, errors.New("header not expected")
	}
	if proc.headerRead {
		return true, errors.New("duplicate header")
	}
	proc.headerRead = true
	if proc.outputHeader {
		if err := proc.sink(header); err != nil {
			return true, err
		}
	}
	return proc.keepZero(), nil
}

func (proc *Processor) ProcessRow(record []string) (processor.Terminate, error) {
	if proc.expectHeader && !proc.headerRead {
		return true, errors.New("header expected before data")
	}
	if proc.keepZero() {
		return true, nil
	}
	if !proc.fromEnd {
		if proc.drop {
			if proc.remaining > 0 {
				proc.remaining--
				return false, nil
			}
			err := proc.sink(record)
			return err != nil, err
		}
		if err := proc.sink(record); err != nil {
			return true, err
		}
		proc.remaining--
		return proc.remaining == 0, nil
	}
	if proc.n == 0 {
		err := proc.sink(record)
		return err != nil, err
	}
	// Grow only with observed input, rather than allocating an untrusted N.
	if len(proc.buffer) < proc.n {
		proc.buffer = append(proc.buffer, append([]string(nil), record...))
		return false, nil
	}
	if proc.drop {
		if err := proc.sink(proc.buffer[proc.next]); err != nil {
			return true, err
		}
	}
	proc.buffer[proc.next] = append([]string(nil), record...)
	proc.next++
	if proc.next == proc.n {
		proc.next = 0
	}
	return false, nil
}

func (proc *Processor) Complete() error {
	if proc.expectHeader && !proc.headerRead {
		return errors.New("header expected")
	}
	if proc.fromEnd && !proc.drop {
		position := proc.next
		for i := 0; i < len(proc.buffer); i++ {
			if err := proc.sink(proc.buffer[position]); err != nil {
				return err
			}
			position++
			if position == len(proc.buffer) {
				position = 0
			}
		}
	}
	proc.buffer = nil
	return nil
}

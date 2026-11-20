package rows

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/barny-dev/ceslav/internal/flags"
	"github.com/barny-dev/ceslav/internal/utilities/processor"
	"github.com/barny-dev/ceslav/internal/utilities/run"
	"github.com/spf13/cobra"
)

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "rows [selectors] [flags]", Short: "Select, drop, reorder or reverse CSV data records", Args: cobra.MaximumNArgs(1),
		Long: "Choose a comma-separated row list, --first N or --last N. Row numbers start at 1 and exclude the header. Ranges include both endpoints and may descend. Append *COUNT to repeat a row or range as a block. Use --reverse alone to reverse all data rows.",
		Example: "  ceslav rows '1,3-4,7' -jk -i input.csv\n" +
			"  ceslav rows '2-4*2' -jk -i input.csv\n" +
			"  ceslav rows --last 10 --drop -jk -i input.csv\n" +
			"  ceslav rows --reverse -jk -i input.csv",
		RunE: func(cmd *cobra.Command, _ []string) error { return run.RunOneToOneCommand(cmd, NewProcessor) },
	}
	fs := cmd.Flags()
	flags.AddInputFileFlag(fs)
	flags.AddOutputFileFlag(fs)
	flags.AddHeaderRowFlag(fs)
	flags.AddOutputHeaderRowFlag(fs)
	fs.String("first", "", "select `N` data rows from the start (nonnegative decimal)")
	fs.String("last", "", "select `N` data rows from the end (nonnegative decimal)")
	fs.Bool("drop", false, "remove the chosen records instead of retaining them")
	fs.Bool("reverse", false, "reverse the resulting data records, keeping the header first")
	return cmd
}

type Processor struct {
	n, remaining                           int
	fromEnd, drop, reverse                 bool
	expectHeader, outputHeader, headerRead bool
	pendingHeader                          []string
	buffer, reversed                       [][]string
	next                                   int
	selection                              *rowSelection
	streamSelection                        bool
	nextSelector                           int
	rowNumber                              int64
	selectedRows                           map[int64][]string
	sink                                   processor.Sink
}

func NewProcessor(cmd *cobra.Command, sink processor.Sink) (processor.Processor, error) {
	fs := cmd.Flags()
	if fs.NArg() > 1 {
		return nil, errors.New("expected one comma-separated row selector list")
	}
	drop, err := fs.GetBool("drop")
	if err != nil {
		return nil, err
	}
	reverse, err := fs.GetBool("reverse")
	if err != nil {
		return nil, err
	}
	modes := fs.NArg()
	for _, flag := range []string{"first", "last"} {
		if fs.Changed(flag) {
			modes++
		}
	}
	if modes > 1 || modes == 0 && (!reverse || drop) {
		return nil, errors.New("specify exactly one of selectors, --first and --last, or use --reverse alone")
	}
	proc := &Processor{drop: drop, reverse: reverse, expectHeader: flags.HasHeader(cmd), outputHeader: flags.HasOutputHeader(cmd), sink: sink}
	if proc.outputHeader && !proc.expectHeader {
		return nil, errors.New("--output-header requires --header")
	}
	if fs.NArg() == 1 {
		proc.selection, err = parseRowSelection(fs.Arg(0))
		if err != nil {
			return nil, err
		}
		proc.streamSelection = !drop && !reverse && proc.selection.inInputOrder()
	} else if modes == 0 {
		// Reversing all rows is equivalent to reversing a drop-first-zero pass.
		proc.drop = true
	} else {
		proc.fromEnd = fs.Changed("last")
		flag := "first"
		if proc.fromEnd {
			flag = "last"
		}
		value, err := fs.GetString(flag)
		if err != nil {
			return nil, err
		}
		count, err := strconv.ParseUint(value, 10, strconv.IntSize-1)
		if err != nil {
			return nil, fmt.Errorf("--%s row count %q must be a nonnegative decimal integer fitting in int", flag, value)
		}
		proc.n = int(count)
		proc.remaining = proc.n
	}
	return proc, nil
}

func (proc *Processor) keepZero() bool {
	return proc.selection == nil && !proc.drop && proc.n == 0
}

func (proc *Processor) emit(record []string) error {
	if proc.reverse {
		proc.reversed = append(proc.reversed, append([]string(nil), record...))
		return nil
	}
	return proc.sink(record)
}

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
		// Delay output if the data cannot be emitted until completion.
		if proc.reverse || !proc.drop && (proc.fromEnd || proc.selection != nil && !proc.streamSelection) {
			proc.pendingHeader = append([]string(nil), header...)
		} else if err := proc.sink(header); err != nil {
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
	if proc.streamSelection {
		proc.rowNumber++
		selector := proc.selection.ranges[proc.nextSelector]
		if proc.rowNumber >= selector.first {
			for copy := int64(0); copy < selector.copies; copy++ {
				if err := proc.sink(record); err != nil {
					return true, err
				}
			}
			if proc.rowNumber == selector.last {
				proc.nextSelector++
			}
		}
		return proc.nextSelector == len(proc.selection.ranges), nil
	}
	if proc.selection != nil {
		if proc.drop && proc.rowNumber >= proc.selection.last {
			// No later row can match; stop advancing the row counter as well.
			err := proc.emit(record)
			return err != nil, err
		}
		proc.rowNumber++
		if proc.selection.contains(proc.rowNumber) {
			if !proc.drop {
				if proc.selectedRows == nil {
					proc.selectedRows = make(map[int64][]string)
				}
				proc.selectedRows[proc.rowNumber] = append([]string(nil), record...)
			}
		} else if proc.drop {
			if err := proc.emit(record); err != nil {
				return true, err
			}
		}
		return !proc.drop && proc.rowNumber == proc.selection.last, nil
	}
	if !proc.fromEnd {
		if proc.drop {
			if proc.remaining > 0 {
				proc.remaining--
				return false, nil
			}
			err := proc.emit(record)
			return err != nil, err
		}
		if err := proc.emit(record); err != nil {
			return true, err
		}
		proc.remaining--
		return proc.remaining == 0, nil
	}
	if proc.n == 0 {
		err := proc.emit(record)
		return err != nil, err
	}
	// Grow only with observed input, rather than allocating an untrusted N.
	if len(proc.buffer) < proc.n {
		proc.buffer = append(proc.buffer, append([]string(nil), record...))
		return false, nil
	}
	if proc.drop {
		if err := proc.emit(proc.buffer[proc.next]); err != nil {
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
	defer func() {
		proc.buffer, proc.reversed, proc.selectedRows = nil, nil, nil
		proc.pendingHeader = nil
	}()
	if proc.expectHeader && !proc.headerRead {
		return errors.New("header expected")
	}
	if proc.pendingHeader != nil {
		if err := proc.sink(proc.pendingHeader); err != nil {
			return err
		}
	}
	if proc.selection != nil && !proc.drop && !proc.streamSelection {
		return proc.selection.forEach(proc.rowNumber, proc.reverse, func(index int64) error {
			return proc.sink(proc.selectedRows[index])
		})
	}
	if proc.fromEnd && !proc.drop {
		position := proc.next
		step := 1
		if proc.reverse {
			position--
			if position < 0 {
				position = len(proc.buffer) - 1
			}
			step = -1
		}
		for i := 0; i < len(proc.buffer); i++ {
			if err := proc.sink(proc.buffer[position]); err != nil {
				return err
			}
			position += step
			if position == len(proc.buffer) {
				position = 0
			} else if position < 0 {
				position = len(proc.buffer) - 1
			}
		}
	}
	for i := len(proc.reversed) - 1; i >= 0; i-- {
		if err := proc.sink(proc.reversed[i]); err != nil {
			return err
		}
	}
	return nil
}

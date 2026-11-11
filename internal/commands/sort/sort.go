package sort

import (
	"errors"
	"fmt"

	"github.com/barny-dev/ceslav/internal/columns"
	"github.com/barny-dev/ceslav/internal/flags"
	"github.com/barny-dev/ceslav/internal/utilities/processor"
	"github.com/barny-dev/ceslav/internal/utilities/row"
	"github.com/barny-dev/ceslav/internal/utilities/rowheap"
	"github.com/barny-dev/ceslav/internal/utilities/run"
	"github.com/barny-dev/ceslav/internal/utilities/sortfunction"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "sort [flags]",
		Short:   "Sort CSV records by string or numeric columns",
		Long:    "Sort CSV records by one or more keys, in priority order. Each key is a direction (+/-), type (s/d), and column (#index/%name). Indices start at 0; names require --header. All records are held in memory.",
		Example: "  ceslav sort -j -k -b '+s%name,-d#1' -i input.csv",
		Args:    cobra.NoArgs,
		RunE:    runCmd,
	}
	fs := cmd.Flags()
	flags.AddInputFileFlag(fs)
	flags.AddOutputFileFlag(fs)
	flags.AddHeaderRowFlag(fs)
	flags.AddOutputHeaderRowFlag(fs)
	AddSortByFlag(fs)
	return cmd
}

func runCmd(cmd *cobra.Command, _ []string) error {
	return run.RunOneToOneCommand(cmd, NewProcessor)
}

type Processor struct {
	expectHeader  bool
	header        []string
	outputHeader  bool
	sortByList    []SortKey
	columnIndices []int
	sink          processor.Sink
	heap          *rowheap.RowHeap
}

func NewProcessor(cmd *cobra.Command, sink processor.Sink) (processor.Processor, error) {
	proc := &Processor{
		expectHeader: flags.HasHeader(cmd),
		outputHeader: flags.HasOutputHeader(cmd),
		sink:         sink,
	}
	if proc.outputHeader && !proc.expectHeader {
		return nil, errors.New("--output-header requires --header")
	}
	sortByList, err := getSortKeys(cmd)
	if err != nil {
		return nil, err
	}
	proc.sortByList = sortByList
	if !proc.expectHeader {
		if err := proc.initialize(nil); err != nil {
			return nil, err
		}
	}
	return proc, nil
}

func (proc *Processor) initialize(header []string) error {
	functions := make([]sortfunction.SortFunction, len(proc.sortByList))
	proc.columnIndices = make([]int, len(proc.sortByList))
	for i, key := range proc.sortByList {
		index, err := key.column.Resolve(header)
		if err != nil {
			return err
		}
		proc.columnIndices[i] = index
		if key.columnType == ColumnTypeDecimal {
			functions[i] = sortfunction.AsDecimal(key.sortDirection, index)
		} else {
			functions[i] = sortfunction.AsString(key.sortDirection, index)
		}
	}
	proc.heap = rowheap.New(sortfunction.All(functions...))
	return nil
}

func (proc *Processor) ProcessHeader(header []string) (processor.Terminate, error) {
	if !proc.expectHeader {
		return true, errors.New("header not expected")
	}
	proc.header = append([]string(nil), header...)
	if err := proc.initialize(header); err != nil {
		return true, err
	}
	return false, nil
}

func (proc *Processor) ProcessRow(record []string) (processor.Terminate, error) {
	if proc.heap == nil {
		return true, errors.New("uninitialized sort")
	}
	// Validate before insertion, including singleton inputs that never compare.
	for i, key := range proc.sortByList {
		index := proc.columnIndices[i]
		if err := columns.ValidateIndex(index, len(record)); err != nil {
			return true, err
		}
		if key.columnType == ColumnTypeDecimal {
			if _, err := sortfunction.ParseDecimal(record[index]); err != nil {
				return true, fmt.Errorf("column %s: %w", key.column, err)
			}
		}
	}
	proc.heap.PushRow(row.InitRow(record))
	return false, nil
}

func (proc *Processor) Complete() error {
	if proc.outputHeader {
		if err := proc.sink(proc.header); err != nil {
			return err
		}
	}
	for proc.heap.Len() > 0 {
		if err := proc.sink(proc.heap.PopRow().Columns); err != nil {
			return err
		}
	}
	return nil
}

func AddSortByFlag(fs *pflag.FlagSet) {
	fs.StringSliceP("sort-by", "b", nil, "required sort keys: +/- for direction, s/d for type, #index/%name for column")
}

func getSortKeys(cmd *cobra.Command) ([]SortKey, error) {
	sources, err := cmd.Flags().GetStringSlice("sort-by")
	if err != nil {
		return nil, err
	}
	if len(sources) == 0 {
		return nil, errors.New("at least one --sort-by key is required")
	}
	keys := make([]SortKey, len(sources))
	for i, source := range sources {
		keys[i], err = ParseSortKey(source)
		if err != nil {
			return nil, err
		}
	}
	return keys, nil
}

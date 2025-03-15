package sort

import (
	"errors"
	"fmt"
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
		Use:  "sort [FLAGS]",
		RunE: runCmd,
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
	expectHeader bool
	header       []string
	outputHeader bool
	sortByList   []SortKey
	sink         processor.Sink
	heap         *rowheap.RowHeap
}

func NewProcessor(cmd *cobra.Command, sink processor.Sink) (processor.Processor, error) {
	var proc = new(Processor)
	proc.expectHeader = flags.HasHeader(cmd)
	proc.outputHeader = flags.HasOutputHeader(cmd)
	if proc.outputHeader && !proc.expectHeader {
		return nil, fmt.Errorf("--output-header is required when --expect-header is true")
	}
	sortByList, err := getSortKeys(cmd)
	if err != nil {
		return nil, err
	}
	proc.sortByList = sortByList
	proc.sink = sink
	if !proc.expectHeader {
		// if no header expected, heap can be initialized right away
		// else wait until header is received
		rh, err := initializeHeap(sortByList, nil)
		if err != nil {
			return nil, err
		}
		proc.heap = rh
	}
	return proc, nil
}

func (proc *Processor) ProcessHeader(header []string) (processor.Terminate, error) {
	if !proc.expectHeader {
		return true, errors.New("processing error: header not expected")
	}
	proc.header = header
	rh, err := initializeHeap(proc.sortByList, header)
	if err != nil {
		return true, err
	}
	proc.heap = rh
	return false, nil
}

func (proc *Processor) ProcessRow(r []string) (processor.Terminate, error) {
	if proc.heap == nil {
		return true, errors.New("uninitialized heap")
	}
	proc.heap.PushRow(row.InitRow(r))
	return false, nil
}
func (proc *Processor) Complete() error {
	if proc.outputHeader {
		if proc.header == nil {
			return errors.New("no header")
		}
		err := proc.sink(proc.header)
		if err != nil {
			return err
		}
	}
	for proc.heap.Len() > 0 {
		err := proc.sink(proc.heap.PopRow().Columns)
		if err != nil {
			return err
		}
	}
	return nil
}

func AddSortByFlag(fs *pflag.FlagSet) {
	fs.StringSliceP("sort-by", "b", nil, "+s%col1,-d#2")
}

func getSortKeys(cmd *cobra.Command) ([]SortKey, error) {
	f := cmd.Flags()
	sortKeyStrings, err := f.GetStringSlice("sort-by")
	if err != nil {
		return nil, err
	}
	sortKeys := make([]SortKey, len(sortKeyStrings))
	for i := 0; i < len(sortKeyStrings); i++ {
		sortKeys[i], err = ParseSortKey(sortKeyStrings[i])
		if err != nil {
			return nil, err
		}
	}
	return sortKeys, nil
}

func initializeHeap(sortKeys []SortKey, header []string) (*rowheap.RowHeap, error) {
	sortFunc, err := generateSortFunction(sortKeys, header)
	if err != nil {
		return nil, err
	}
	rh := rowheap.New(sortFunc)
	return rh, nil
}

func generateSortFunction(sortKeys []SortKey, header []string) (sortfunction.SortFunction, error) {
	var sortFuncs = make([]sortfunction.SortFunction, len(sortKeys))
	for i, s := range sortKeys {
		sortFunc, err := sortKeyToSortFunction(s, header)
		if err != nil {
			return nil, err
		}
		sortFuncs[i] = sortFunc
	}
	return sortfunction.All(sortFuncs...), nil
}

func sortKeyToSortFunction(sortKey SortKey, header []string) (sortfunction.SortFunction, error) {
	var columnIndex int
	var columnCount *int
	var err error
	if header != nil {
		columnCount = new(int)
		*columnCount = len(header)
	}
	if sortKey.columnBy == ColumnByIndex {
		columnIndex = int(sortKey.columnIndex)
		if columnCount != nil && columnIndex >= *columnCount {
			return nil, errors.New("configuration error: sort by column index too small")
		}
	} else if sortKey.columnBy == ColumnByName {
		columnIndex, err = findColumnIndexByColumnName(sortKey, header)
		if err != nil {
			return nil, err
		}
	} else {
		return nil, errors.New("configuration error: sort by column must be identified by index or name")
	}

	if sortKey.columnType == ColumnTypeDecimal {
		return sortfunction.AsDecimal(sortKey.sortDirection, columnIndex), nil
	} else if sortKey.columnType == ColumnTypeString {
		return sortfunction.AsString(sortKey.sortDirection, columnIndex), nil
	} else {
		return nil, errors.New("configuration error: sort as must be decimal or string")
	}
}

func findColumnIndexByColumnName(sortKey SortKey, header []string) (int, error) {
	for i, n := range header {
		if n == sortKey.columnBy {
			return i, nil
		}
	}
	return 0, errors.New("configuration error: sort by column name not found in header")
}

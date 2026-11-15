package projection

import (
	"errors"
	"fmt"
	"sort"

	"github.com/barny-dev/ceslav/internal/columns"
	"github.com/barny-dev/ceslav/internal/flags"
	"github.com/barny-dev/ceslav/internal/utilities/processor"
	"github.com/barny-dev/ceslav/internal/utilities/run"
	"github.com/spf13/cobra"
)

func SelectCmd() *cobra.Command { return command(false) }
func DropCmd() *cobra.Command   { return command(true) }

func command(drop bool) *cobra.Command {
	name, short := "select", "Select CSV columns in the requested order"
	if drop {
		name, short = "drop", "Remove CSV columns, retaining the order of the remainder"
	}
	cmd := &cobra.Command{
		Use: name + " [columns] [flags]", Short: short, Args: cobra.MaximumNArgs(1),
		Long: short + ". Supply one comma-separated list such as name,city or #2,#0-1. Bare names and %name require --header; #indices start at 0 and support inclusive ascending/descending ranges. For select, source=output-name renames one column and requires --header --output-header. Escape commas, equals signs and backslashes in names with a backslash. Alternatively repeat --column with literal #index or %name selectors. Duplicate selectors and zero-column output are errors.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return run.RunOneToOneCommand(cmd, func(cmd *cobra.Command, sink processor.Sink) (processor.Processor, error) {
				return NewProcessor(cmd, sink, drop)
			})
		},
	}
	fs := cmd.Flags()
	flags.AddInputFileFlag(fs)
	flags.AddOutputFileFlag(fs)
	flags.AddHeaderRowFlag(fs)
	flags.AddOutputHeaderRowFlag(fs)
	fs.StringArrayP("column", "c", nil, "column selector (#index or %name); repeat to specify multiple columns")
	return cmd
}

type Processor struct {
	selectors                        []columnSelector
	drop, expectHeader, outputHeader bool
	width                            int
	projection                       []selectedColumn
	sink                             processor.Sink
}

type selectedColumn struct {
	index int
	alias string
}

func NewProcessor(cmd *cobra.Command, sink processor.Sink, drop bool) (processor.Processor, error) {
	proc := &Processor{drop: drop, expectHeader: flags.HasHeader(cmd), outputHeader: flags.HasOutputHeader(cmd), width: -1, sink: sink}
	if proc.outputHeader && !proc.expectHeader {
		return nil, errors.New("--output-header requires --header")
	}
	selectors, err := readSelectors(cmd)
	if err != nil {
		return nil, err
	}
	proc.selectors = selectors
	for _, selector := range selectors {
		if selector.alias != "" {
			if drop {
				return nil, errors.New("column renaming is supported by select only")
			}
			if !proc.expectHeader || !proc.outputHeader {
				return nil, errors.New("column renaming requires --header and --output-header")
			}
		}
	}
	if !proc.expectHeader {
		// Resolve names and aliases before opening input, even when it is empty.
		if _, err := proc.resolve(nil, -1); err != nil {
			return nil, err
		}
	}
	return proc, nil
}

func (proc *Processor) resolve(header []string, width int) ([]selectedColumn, error) {
	type span struct {
		first, last int
		selector    columnSelector
	}
	spans := make([]span, 0, len(proc.selectors))
	for _, selector := range proc.selectors {
		first, err := selector.first.Resolve(header)
		if err != nil {
			return nil, err
		}
		last, err := selector.last.Resolve(header)
		if err != nil {
			return nil, err
		}
		if width >= 0 {
			for _, index := range []int{first, last} {
				if err := columns.ValidateIndex(index, width); err != nil {
					return nil, err
				}
			}
		}
		spans = append(spans, span{first, last, selector})
	}
	// Check overlaps without expanding ranges, even before input width is known.
	ordered := append([]span(nil), spans...)
	sort.Slice(ordered, func(i, j int) bool {
		return min(ordered[i].first, ordered[i].last) < min(ordered[j].first, ordered[j].last)
	})
	for i := 1; i < len(ordered); i++ {
		current, previous := ordered[i], ordered[i-1]
		low := min(current.first, current.last)
		if low <= max(previous.first, previous.last) {
			return nil, fmt.Errorf("duplicate column #%d in selectors %s and %s", low, previous.selector, current.selector)
		}
	}
	if width < 0 {
		return nil, nil
	}
	var selected []selectedColumn
	for _, span := range spans {
		step := 1
		if span.first > span.last {
			step = -1
		}
		for index := span.first; ; index += step {
			selected = append(selected, selectedColumn{index: index, alias: span.selector.alias})
			if index == span.last {
				break
			}
		}
	}
	return selected, nil
}

func (proc *Processor) prepare(header []string, width int) error {
	selected, err := proc.resolve(header, width)
	if err != nil {
		return err
	}
	projection := selected
	if proc.drop {
		removed := make(map[int]bool, len(selected))
		for _, column := range selected {
			removed[column.index] = true
		}
		projection = make([]selectedColumn, 0)
		for index := 0; index < width; index++ {
			if !removed[index] {
				projection = append(projection, selectedColumn{index: index})
			}
		}
	}
	if len(projection) == 0 {
		return errors.New("projection would leave no columns")
	}
	proc.projection, proc.width = projection, width
	return nil
}

func (proc *Processor) project(record []string) []string {
	result := make([]string, len(proc.projection))
	for i, column := range proc.projection {
		result[i] = record[column.index]
	}
	return result
}

func (proc *Processor) projectHeader(header []string) ([]string, error) {
	result := proc.project(header)
	counts := make(map[string]int, len(result))
	for i, column := range proc.projection {
		if column.alias != "" {
			result[i] = column.alias
		}
		counts[result[i]]++
	}
	for _, column := range proc.projection {
		if column.alias != "" && counts[column.alias] > 1 {
			return nil, fmt.Errorf("renamed column %q collides with another output column", column.alias)
		}
	}
	return result, nil
}

func (proc *Processor) ProcessHeader(header []string) (processor.Terminate, error) {
	if !proc.expectHeader {
		return true, errors.New("header not expected")
	}
	if proc.width >= 0 {
		return true, errors.New("duplicate header")
	}
	if err := proc.prepare(header, len(header)); err != nil {
		return true, err
	}
	if proc.outputHeader {
		output, err := proc.projectHeader(header)
		if err != nil {
			return true, err
		}
		if err := proc.sink(output); err != nil {
			return true, err
		}
	}
	return false, nil
}

func (proc *Processor) ProcessRow(record []string) (processor.Terminate, error) {
	if proc.width < 0 {
		if proc.expectHeader {
			return true, errors.New("header expected before data")
		}
		if err := proc.prepare(nil, len(record)); err != nil {
			return true, err
		}
	}
	if len(record) != proc.width {
		return true, fmt.Errorf("expected %d columns, got %d", proc.width, len(record))
	}
	err := proc.sink(proc.project(record))
	return err != nil, err
}

func (proc *Processor) Complete() error { return nil }

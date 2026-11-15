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
		Long: short + ". Supply one comma-separated list such as name,city or #2,#0-1. Bare names and %name require --header; #indices start at 0 and support inclusive ascending/descending ranges. Alternatively repeat --column with literal #index or %name selectors, including names containing commas. Duplicate selectors and zero-column output are errors.",
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
	indices                          []int
	sink                             processor.Sink
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
	if !proc.expectHeader {
		// Resolve names and aliases before opening input, even when it is empty.
		if _, err := proc.resolve(nil, -1); err != nil {
			return nil, err
		}
	}
	return proc, nil
}

func (proc *Processor) resolve(header []string, width int) ([]int, error) {
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
	var indices []int
	for _, span := range spans {
		step := 1
		if span.first > span.last {
			step = -1
		}
		for index := span.first; ; index += step {
			indices = append(indices, index)
			if index == span.last {
				break
			}
		}
	}
	return indices, nil
}

func (proc *Processor) prepare(header []string, width int) error {
	selected, err := proc.resolve(header, width)
	if err != nil {
		return err
	}
	indices := selected
	if proc.drop {
		removed := make(map[int]bool, len(selected))
		for _, index := range selected {
			removed[index] = true
		}
		indices = make([]int, 0)
		for index := 0; index < width; index++ {
			if !removed[index] {
				indices = append(indices, index)
			}
		}
	}
	if len(indices) == 0 {
		return errors.New("projection would leave no columns")
	}
	proc.indices, proc.width = indices, width
	return nil
}

func (proc *Processor) project(record []string) []string {
	result := make([]string, len(proc.indices))
	for i, index := range proc.indices {
		result[i] = record[index]
	}
	return result
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
		if err := proc.sink(proc.project(header)); err != nil {
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

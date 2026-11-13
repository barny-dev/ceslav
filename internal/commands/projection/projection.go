package projection

import (
	"errors"
	"fmt"

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
		Use: name + " [flags]", Short: short, Args: cobra.NoArgs,
		Long: short + ". Repeat --column for each #index or %name selector. Indices start at 0; names require --header. Duplicate selectors and zero-column output are errors.",
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
	references                       []columns.Reference
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
	selectors, err := cmd.Flags().GetStringArray("column")
	if err != nil {
		return nil, err
	}
	if len(selectors) == 0 {
		return nil, errors.New("at least one --column is required")
	}
	for _, selector := range selectors {
		ref, err := columns.Parse(selector)
		if err != nil {
			return nil, err
		}
		proc.references = append(proc.references, ref)
	}
	if !proc.expectHeader {
		// Resolve names and aliases before opening input, even when it is empty.
		if _, err := proc.resolve(nil); err != nil {
			return nil, err
		}
	}
	return proc, nil
}

func (proc *Processor) resolve(header []string) ([]int, error) {
	seen := make(map[int]columns.Reference, len(proc.references))
	indices := make([]int, 0, len(proc.references))
	for _, ref := range proc.references {
		index, err := ref.Resolve(header)
		if err != nil {
			return nil, err
		}
		if earlier, exists := seen[index]; exists {
			return nil, fmt.Errorf("duplicate column %s (already specified as %s)", ref, earlier)
		}
		seen[index] = ref
		indices = append(indices, index)
	}
	return indices, nil
}

func (proc *Processor) prepare(header []string, width int) error {
	selected, err := proc.resolve(header)
	if err != nil {
		return err
	}
	for _, index := range selected {
		if err := columns.ValidateIndex(index, width); err != nil {
			return err
		}
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

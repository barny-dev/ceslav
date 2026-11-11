package processor

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

type Terminate = bool
type Sink = func([]string) error

type Processor interface {
	ProcessHeader([]string) (Terminate, error)
	ProcessRow([]string) (Terminate, error)
	Complete() error
}

type Initializer = func(*cobra.Command, Sink) (Processor, error)

// Run processes one record at a time. A deliberate stop completes the processor
// without reading another record. Failures and cancellation skip completion.
func Run(ctx context.Context, input *csv.Reader, hasHeader bool, proc Processor) error {
	complete := func() error {
		if err := ctx.Err(); err != nil {
			return context.Cause(ctx)
		}
		if err := proc.Complete(); err != nil {
			return fmt.Errorf("complete processing: %w", err)
		}
		return nil
	}
	for record := 1; ; record++ {
		if err := ctx.Err(); err != nil {
			return context.Cause(ctx)
		}
		values, err := input.Read()
		if ctx.Err() != nil {
			return context.Cause(ctx)
		}
		if errors.Is(err, io.EOF) {
			if hasHeader && record == 1 {
				return errors.New("input: header expected, but input is empty")
			}
			return complete()
		}
		if err != nil {
			return fmt.Errorf("input record %d: %w", record, err)
		}
		var stop bool
		if hasHeader && record == 1 {
			stop, err = proc.ProcessHeader(values)
		} else {
			stop, err = proc.ProcessRow(values)
		}
		if err != nil {
			return fmt.Errorf("process record %d: %w", record, err)
		}
		if stop {
			return complete()
		}
	}
}

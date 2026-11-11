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
// Cancellation closes input to interrupt a pending Read; Close must unblock Read.
// On ordinary completion input stays open and remains the caller's responsibility.
func Run(ctx context.Context, input io.ReadCloser, hasHeader bool, proc Processor) (outcome error) {
	if ctx.Err() != nil {
		return context.Cause(ctx)
	}
	closed := make(chan struct{})
	var closeErr error
	stop := context.AfterFunc(ctx, func() {
		closeErr = input.Close()
		close(closed)
	})
	defer func() {
		if !stop() {
			// Join a callback that has started, so it cannot close input after
			// the caller resumes or race with normal input cleanup.
			<-closed
		}
		if cause := context.Cause(ctx); cause != nil && !errors.Is(outcome, cause) {
			outcome = errors.Join(cause, outcome)
		}
		if closeErr != nil {
			outcome = errors.Join(outcome, fmt.Errorf("close canceled input: %w", closeErr))
		}
	}()
	inputCSV := csv.NewReader(input)
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
		values, err := inputCSV.Read()
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

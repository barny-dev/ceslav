package processor

import (
	"context"
	"errors"
	"github.com/barny-dev/ceslav/internal/utilities/local_errors"
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

type Configuration struct {
	Initializer Initializer
	HasHeader   bool
	Input       <-chan []string
	Output      chan<- []string
}

func (cfg Configuration) Run(
	ctx context.Context,
	cancel context.CancelCauseFunc,
	cmd *cobra.Command,
) {
	defer close(cfg.Output)
	sink := func(row []string) error {
		select {
		case cfg.Output <- row:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	proc, err := cfg.Initializer(cmd, sink)

	if err != nil {
		cancel(local_errors.Processor(err))
	}
	if cfg.HasHeader {
		var row []string
		var open bool
		select {
		case row, open = <-cfg.Input:
		case <-ctx.Done():
			return
		}
		if !open {
			cancel(local_errors.Processor(errors.New("header expected")))
			return
		}
		terminate, err := proc.ProcessHeader(row)
		if err != nil {
			cancel(local_errors.Processor(err))
		}
		if terminate {
			return
		}
	}
	for {
		var row []string
		var open bool
		select {
		case row, open = <-cfg.Input:
		case <-ctx.Done():
		}
		if !open {
			err := proc.Complete()
			if err != nil {
				cancel(local_errors.Processor(err))
			}
			return
		}
		terminate, err := proc.ProcessRow(row)
		if err != nil {
			cancel(local_errors.Processor(err))
			return
		}
		if terminate {
			return
		}
	}
}

package run

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/barny-dev/ceslav/internal/flags"
	"github.com/barny-dev/ceslav/internal/utilities/processor"
	"github.com/spf13/cobra"
)

func RunOneToOneCommand(cmd *cobra.Command, initialize processor.Initializer) (outcome error) {
	var inputFile, outputFile *os.File
	var outputCSV *csv.Writer
	// Open output only after validation, when a processor first emits a record.
	// Empty successful commands still create/truncate their destination below.
	openOutput := func() error {
		if outputCSV != nil {
			return nil
		}
		var err error
		outputFile, err = flags.GetOutput(cmd, inputFile)
		if err != nil {
			return err
		}
		outputCSV = csv.NewWriter(outputFile)
		return nil
	}
	sink := func(record []string) error {
		if err := cmd.Context().Err(); err != nil {
			return context.Cause(cmd.Context())
		}
		if err := openOutput(); err != nil {
			return err
		}
		if err := outputCSV.Write(record); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
		return nil
	}
	proc, err := initialize(cmd, sink)
	if err != nil {
		return fmt.Errorf("configuration: %w", err)
	}
	inputFile, err = flags.GetInput(cmd)
	if err != nil {
		return err
	}
	input := &closeOnceInput{ReadCloser: inputFile}
	defer func() {
		if inputFile != os.Stdin {
			if err := input.Close(); err != nil && !errors.Is(outcome, err) {
				outcome = errors.Join(outcome, fmt.Errorf("close input: %w", err))
			}
		}
	}()
	defer func() {
		var closer io.Closer
		if outputFile != nil && outputFile != os.Stdout {
			closer = outputFile
		}
		outcome = errors.Join(outcome, finishOutput(outputCSV, closer))
	}()

	if err := processor.Run(cmd.Context(), input, flags.HasHeader(cmd), proc); err != nil {
		return err
	}
	return openOutput()
}

// Always attempt both flush and close; neither failure may hide the other.
func finishOutput(writer *csv.Writer, closer io.Closer) (outcome error) {
	if writer != nil {
		writer.Flush()
		if err := writer.Error(); err != nil {
			outcome = fmt.Errorf("flush output: %w", err)
		}
	}
	if closer != nil {
		if err := closer.Close(); err != nil {
			outcome = errors.Join(outcome, fmt.Errorf("close output: %w", err))
		}
	}
	return outcome
}

// The cancellation callback and normal cleanup share one close operation.
type closeOnceInput struct {
	io.ReadCloser
	once sync.Once
	err  error
}

func (input *closeOnceInput) Close() error {
	input.once.Do(func() { input.err = input.ReadCloser.Close() })
	return input.err
}

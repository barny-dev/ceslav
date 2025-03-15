package run

import (
	"context"
	"encoding/csv"
	"github.com/barny-dev/ceslav/internal/flags"
	"github.com/barny-dev/ceslav/internal/utilities/input"
	"github.com/barny-dev/ceslav/internal/utilities/output"
	"github.com/barny-dev/ceslav/internal/utilities/processor"
	"github.com/spf13/cobra"
	"log"
	"os"
)

func RunOneToOneCommand(cmd *cobra.Command, procInitializer processor.Initializer) (outcome error) {
	ctx, cancel := context.WithCancelCause(cmd.Context())
	var inputCsv *csv.Reader
	{
		inputFile, err := flags.GetInput(cmd)
		if err != nil {
			return err
		}
		inputCsv = csv.NewReader(inputFile)
		inputCsv.ReuseRecord = true
		if inputFile != os.Stdin {
			context.AfterFunc(ctx, func() {
				err := inputFile.Close()
				if err != nil {
					log.Panic("err: can't close input file")
				}
			})
		}
	}
	var outputCsv *csv.Writer
	{
		outputFile, err := flags.GetOutput(cmd)
		if err != nil {
			return err
		}
		outputCsv = csv.NewWriter(outputFile)
		if outputFile != os.Stdout {
			context.AfterFunc(ctx, func() {
				err := outputFile.Close()
				if err != nil {
					log.Panic("err: can't close output file")
				}
			})
		}
	}
	//context.WithCancel(ctx)
	//ctx.Done()
	inputChannel := make(chan []string)
	outputChannel := make(chan []string)
	procCfg := processor.Configuration{
		Initializer: procInitializer,
		HasHeader:   flags.HasHeader(cmd),
		Input:       inputChannel,
		Output:      outputChannel,
	}
	go input.ReadInput(ctx, cancel, inputCsv, inputChannel)
	go output.WriteOutput(ctx, cancel, outputCsv, outputChannel)
	go procCfg.Run(ctx, cancel, cmd)

	<-ctx.Done()
	err := ctx.Err()
	if err != context.Canceled {
		return err
	} else {
		return nil
	}
}

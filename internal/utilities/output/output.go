package output

import (
	"context"
	"encoding/csv"
	"fmt"
	"github.com/barny-dev/ceslav/internal/utilities/local_errors"
	"log"
)

func WriteOutput(
	ctx context.Context,
	cancel context.CancelCauseFunc,
	outputCsv *csv.Writer,
	outputChannel <-chan []string,
) {
	defer protect(cancel)
	for {
		var record []string
		var open bool
		select {
		case record, open = <-outputChannel:
			log.Printf("record: %v\n", record)
		case <-ctx.Done():
			return
		}
		if !open {
			outputCsv.Flush()
			if err := outputCsv.Error(); err != nil {
				cancel(local_errors.Output(err))
			} else {
				cancel(nil)
			}
			return
		}
		err := outputCsv.Write(record)
		if err != nil {
			cancel(local_errors.Output(err))
			return
		}
	}
}

func protect(cancel context.CancelCauseFunc) {
	panicking := recover()
	if panicking != nil {
		cancel(local_errors.Output(fmt.Errorf("panicking - %v", panicking)))
	}
}

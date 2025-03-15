package input

import (
	"context"
	"encoding/csv"
	"fmt"
	"github.com/barny-dev/ceslav/internal/utilities/local_errors"
	"io"
	"log"
	"slices"
)

func ReadInput(
	ctx context.Context,
	cancel context.CancelCauseFunc,
	inputCsv *csv.Reader,
	inputChannel chan<- []string,
) {
	defer close(inputChannel)
	defer protect(cancel)
	for {
		record, err := inputCsv.Read()
		if err == io.EOF {
			return
		}
		if err != nil {
			cancel(local_errors.Input(err))
			return
		}
		log.Printf("record: %v\n", record)
		select {
		case inputChannel <- slices.Clone(record):
		case <-ctx.Done():
			return
		}

	}
}

func protect(cancel context.CancelCauseFunc) {
	panicking := recover()
	if panicking != nil {
		cancel(local_errors.Input(fmt.Errorf("panicking - %v", panicking)))
	}
}

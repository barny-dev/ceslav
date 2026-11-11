package run

import (
	"encoding/csv"
	"errors"
	"testing"
)

type failedOutput struct {
	writeErr, closeErr error
	closed             bool
}

func (f *failedOutput) Write(p []byte) (int, error) {
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	return len(p), nil
}
func (f *failedOutput) Close() error { f.closed = true; return f.closeErr }

func TestFinishOutputPreservesFlushAndCloseFailures(t *testing.T) {
	flushErr, closeErr := errors.New("disk full"), errors.New("close failed")
	stream := &failedOutput{writeErr: flushErr, closeErr: closeErr}
	writer := csv.NewWriter(stream)
	if err := writer.Write([]string{"buffered"}); err != nil {
		t.Fatal(err)
	}
	err := finishOutput(writer, stream)
	if !errors.Is(err, flushErr) || !errors.Is(err, closeErr) {
		t.Fatalf("lost failure: %v", err)
	}
	if !stream.closed {
		t.Fatal("did not close after flush failed")
	}
}

func TestFinishOutputReportsCloseFailure(t *testing.T) {
	closeErr := errors.New("close failed")
	stream := &failedOutput{closeErr: closeErr}
	writer := csv.NewWriter(stream)
	if err := writer.Write([]string{"buffered"}); err != nil {
		t.Fatal(err)
	}
	if err := finishOutput(writer, stream); !errors.Is(err, closeErr) {
		t.Fatalf("error: %v", err)
	}
}

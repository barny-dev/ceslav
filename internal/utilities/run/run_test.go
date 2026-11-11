package run

import (
	"encoding/csv"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"testing"
	"time"
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

// A gated close exposes overlapping cancellation and normal-cleanup calls
// without needing a file, pipe, CSV stream, or scheduler sleeps.
type gatedCloseInput struct {
	calls            atomic.Int32
	started, release chan struct{}
	closeErr         error
}

func (input *gatedCloseInput) Read([]byte) (int, error) { return 0, io.EOF }
func (input *gatedCloseInput) Close() error {
	if input.calls.Add(1) == 1 {
		close(input.started)
	}
	<-input.release
	return input.closeErr
}

func TestConcurrentInputCleanupClosesOnceAndRetainsError(t *testing.T) {
	failure := errors.New("input close failed")
	source := &gatedCloseInput{started: make(chan struct{}), release: make(chan struct{}), closeErr: failure}
	var release sync.Once
	defer release.Do(func() { close(source.release) })
	input := &closeOnceInput{ReadCloser: source}
	const callers = 32
	start := make(chan struct{})
	results := make(chan error, callers)
	for i := 0; i < callers; i++ {
		go func() { <-start; results <- input.Close() }()
	}
	close(start)
	select {
	case <-source.started:
	case <-time.After(5 * time.Second):
		t.Fatal("close did not start")
	}
	release.Do(func() { close(source.release) })
	for i := 0; i < callers; i++ {
		select {
		case err := <-results:
			if !errors.Is(err, failure) {
				t.Fatalf("cleanup lost close failure: %v", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("cleanup caller remained blocked")
		}
	}
	if source.calls.Load() != 1 {
		t.Fatalf("underlying input closed %d times", source.calls.Load())
	}
	if err := input.Close(); !errors.Is(err, failure) {
		t.Fatalf("later cleanup changed error: %v", err)
	}
	if source.calls.Load() != 1 {
		t.Fatal("later cleanup closed input again")
	}
}

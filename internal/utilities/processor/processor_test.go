package processor

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

type probeProcessor struct {
	headers, rows, completed       int
	stopHeader, stopRow            bool
	headerErr, rowErr, completeErr error
	afterRow                       func()
	afterComplete                  func()
}

func (p *probeProcessor) ProcessHeader([]string) (bool, error) {
	p.headers++
	return p.stopHeader, p.headerErr
}
func (p *probeProcessor) ProcessRow([]string) (bool, error) {
	p.rows++
	if p.afterRow != nil {
		p.afterRow()
	}
	return p.stopRow, p.rowErr
}
func (p *probeProcessor) Complete() error {
	p.completed++
	if p.afterComplete != nil {
		p.afterComplete()
	}
	return p.completeErr
}

type failingReader struct {
	called bool
	cause  error
}

func (r *failingReader) Read([]byte) (int, error) { r.called = true; return 0, r.cause }

func TestRunLifecycle(t *testing.T) {
	failure := errors.New("injected failure")
	cases := []struct {
		name, input                         string
		header                              bool
		proc                                probeProcessor
		wantHeaders, wantRows, wantComplete int
		wantErr                             error
	}{
		{"EOF", "one\ntwo\n", false, probeProcessor{}, 0, 2, 1, nil},
		{"empty", "", false, probeProcessor{}, 0, 0, 1, nil},
		{"header only", "head\n", true, probeProcessor{}, 1, 0, 1, nil},
		{"header and rows", "head\none\ntwo\n", true, probeProcessor{}, 1, 2, 1, nil},
		{"stop at header", "head\none\n", true, probeProcessor{stopHeader: true}, 1, 0, 1, nil},
		{"stop at row", "one\ntwo\n", false, probeProcessor{stopRow: true}, 0, 1, 1, nil},
		{"header error", "head\none\n", true, probeProcessor{headerErr: failure}, 1, 0, 0, failure},
		{"row error", "one\ntwo\n", false, probeProcessor{rowErr: failure}, 0, 1, 0, failure},
		{"error with stop", "one\ntwo\n", false, probeProcessor{rowErr: failure, stopRow: true}, 0, 1, 0, failure},
		{"complete error", "one\n", false, probeProcessor{completeErr: failure}, 0, 1, 1, failure},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			proc := tc.proc
			err := Run(context.Background(), io.NopCloser(strings.NewReader(tc.input)), tc.header, &proc)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("error: %v, want %v", err, tc.wantErr)
			}
			if proc.headers != tc.wantHeaders || proc.rows != tc.wantRows || proc.completed != tc.wantComplete {
				t.Fatalf("headers/rows/completions: %d/%d/%d, want %d/%d/%d", proc.headers, proc.rows, proc.completed, tc.wantHeaders, tc.wantRows, tc.wantComplete)
			}
		})
	}
}

func TestEarlyStopDoesNotReadAgain(t *testing.T) {
	blocked := &failingReader{cause: errors.New("must not read beyond selected row")}
	input := io.NopCloser(io.MultiReader(strings.NewReader("one\n"), blocked))
	proc := &probeProcessor{stopRow: true}
	if err := Run(context.Background(), input, false, proc); err != nil {
		t.Fatal(err)
	}
	if blocked.called {
		t.Fatal("read after deliberate stop")
	}
	if proc.completed != 1 {
		t.Fatalf("completed %d times", proc.completed)
	}
}

func TestInputErrorSkipsCompletion(t *testing.T) {
	failure := errors.New("read failed")
	input := io.NopCloser(&failingReader{cause: failure})
	proc := &probeProcessor{}
	if err := Run(context.Background(), input, false, proc); !errors.Is(err, failure) {
		t.Fatalf("error: %v", err)
	}
	if proc.completed != 0 {
		t.Fatal("completed after read error")
	}
}

func TestCancellationPreservesCause(t *testing.T) {
	for _, before := range []bool{true, false} {
		t.Run(map[bool]string{true: "before read", false: "during processing"}[before], func(t *testing.T) {
			failure := errors.New("caller canceled")
			ctx, cancel := context.WithCancelCause(context.Background())
			defer cancel(nil)
			proc := &probeProcessor{}
			if before {
				cancel(failure)
			} else {
				proc.afterRow = func() { cancel(failure) }
			}
			err := Run(ctx, io.NopCloser(strings.NewReader("one\ntwo\n")), false, proc)
			if !errors.Is(err, failure) {
				t.Fatalf("error: %v", err)
			}
			if proc.completed != 0 {
				t.Fatal("completed after cancellation")
			}
			if before && proc.rows != 0 {
				t.Fatal("processed a row after cancellation")
			}
		})
	}
}

type cancelingReader struct {
	reader io.Reader
	cancel func()
}

func (r cancelingReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.cancel()
	return n, err
}

func TestCancellationDuringReadSkipsProcessing(t *testing.T) {
	cause := errors.New("canceled during read")
	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)
	reader := cancelingReader{strings.NewReader("one\n"), func() { cancel(cause) }}
	proc := &probeProcessor{}
	err := Run(ctx, io.NopCloser(reader), false, proc)
	if !errors.Is(err, cause) {
		t.Fatalf("lost cancellation cause: %v", err)
	}
	if proc.rows != 0 || proc.completed != 0 {
		t.Fatal("processed or completed after cancellation during read")
	}
}

// Signal when Run has entered the underlying read, rather than relying on
// sleeps or a reader that returns immediately after cancellation.
type signaledInput struct {
	io.ReadCloser
	entered chan struct{}
	signal  sync.Once
}

func (input *signaledInput) Read(p []byte) (int, error) {
	input.signal.Do(func() { close(input.entered) })
	return input.ReadCloser.Read(p)
}

func TestCancellationUnblocksIdlePipe(t *testing.T) {
	for _, useDeadline := range []bool{false, true} {
		name := "cause"
		if useDeadline {
			name = "deadline"
		}
		t.Run(name, func(t *testing.T) {
			reader, writer := io.Pipe()
			defer reader.Close()
			defer writer.Close()
			input := &signaledInput{ReadCloser: reader, entered: make(chan struct{})}
			var ctx context.Context
			var cancel func()
			cause := errors.New("caller canceled idle input")
			if useDeadline {
				deadlineCtx, deadlineCancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
				ctx, cancel = deadlineCtx, deadlineCancel
				cause = context.DeadlineExceeded
			} else {
				causeCtx, causeCancel := context.WithCancelCause(context.Background())
				ctx, cancel = causeCtx, func() { causeCancel(cause) }
			}
			defer cancel()
			proc := &probeProcessor{}
			done := make(chan error, 1)
			go func() { done <- Run(ctx, input, false, proc) }()
			select {
			case <-input.entered:
			case <-time.After(5 * time.Second):
				t.Fatal("runner did not enter Read")
			}
			if !useDeadline {
				cancel()
			}
			select {
			case err := <-done:
				if !errors.Is(err, cause) {
					t.Fatalf("lost cancellation cause: %v", err)
				}
				if proc.rows != 0 || proc.completed != 0 {
					t.Fatal("processed or completed canceled input")
				}
			case <-time.After(5 * time.Second):
				t.Fatal("cancellation did not unblock the idle pipe")
			}
		})
	}
}

type trackedInput struct {
	io.Reader
	closed   bool
	closeErr error
}

func (input *trackedInput) Close() error { input.closed = true; return input.closeErr }

func TestOrdinaryCompletionLeavesInputOpen(t *testing.T) {
	for _, stop := range []bool{false, true} {
		input := &trackedInput{Reader: strings.NewReader("one\ntwo\n")}
		ctx, cancel := context.WithCancel(context.Background())
		proc := &probeProcessor{stopRow: stop}
		if err := Run(ctx, input, false, proc); err != nil {
			t.Fatal(err)
		}
		cancel()
		if input.closed {
			t.Fatal("closed caller input on ordinary completion or late cancellation")
		}
	}
}

func TestCancellationDuringCompletionPreservesCause(t *testing.T) {
	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)
	cause := errors.New("canceled while completing")
	proc := &probeProcessor{afterComplete: func() { cancel(cause) }}
	err := Run(ctx, io.NopCloser(strings.NewReader("")), false, proc)
	if !errors.Is(err, cause) {
		t.Fatalf("lost cancellation during completion: %v", err)
	}
}

type closingInput struct {
	io.ReadCloser
	closeStarted, closeRelease chan struct{}
	closeErr                   error
}

func (input *closingInput) Close() error {
	err := input.ReadCloser.Close()
	close(input.closeStarted)
	<-input.closeRelease
	return errors.Join(err, input.closeErr)
}

func TestCancellationJoinsCloseAndPreservesItsError(t *testing.T) {
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()
	closeErr := errors.New("close failed")
	source := &closingInput{ReadCloser: reader, closeStarted: make(chan struct{}), closeRelease: make(chan struct{}), closeErr: closeErr}
	defer close(source.closeRelease)
	input := &signaledInput{ReadCloser: source, entered: make(chan struct{})}
	ctx, cancel := context.WithCancelCause(context.Background())
	cause := errors.New("caller canceled")
	defer cancel(nil)
	done := make(chan error, 1)
	go func() { done <- Run(ctx, input, false, &probeProcessor{}) }()
	select {
	case <-input.entered:
	case <-time.After(5 * time.Second):
		t.Fatal("read not started")
	}
	cancel(cause)
	select {
	case <-source.closeStarted:
	case <-time.After(5 * time.Second):
		t.Fatal("close not started")
	}
	select {
	case err := <-done:
		t.Fatalf("returned before cancellation cleanup completed: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	source.closeRelease <- struct{}{}
	select {
	case err := <-done:
		if !errors.Is(err, cause) || !errors.Is(err, closeErr) {
			t.Fatalf("lost failure: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("runner did not finish after close")
	}
}

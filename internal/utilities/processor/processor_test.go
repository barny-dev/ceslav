package processor

import (
	"context"
	"encoding/csv"
	"errors"
	"io"
	"strings"
	"testing"
)

type probeProcessor struct {
	headers, rows, completed       int
	stopHeader, stopRow            bool
	headerErr, rowErr, completeErr error
	afterRow                       func()
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
func (p *probeProcessor) Complete() error { p.completed++; return p.completeErr }

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
			err := Run(context.Background(), csv.NewReader(strings.NewReader(tc.input)), tc.header, &proc)
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
	input := csv.NewReader(io.MultiReader(strings.NewReader("one\n"), blocked))
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
	input := csv.NewReader(&failingReader{cause: failure})
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
			err := Run(ctx, csv.NewReader(strings.NewReader("one\ntwo\n")), false, proc)
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

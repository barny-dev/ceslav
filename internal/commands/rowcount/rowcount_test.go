package rowcount

import (
	"errors"
	"reflect"
	"testing"

	"github.com/barny-dev/ceslav/internal/utilities/processor"
)

func newTestCounter(t *testing.T, args []string, sink processor.Sink) processor.Processor {
	t.Helper()
	cmd := Cmd()
	if err := cmd.ParseFlags(args); err != nil {
		t.Fatal(err)
	}
	proc, err := NewProcessor(cmd, sink)
	if err != nil {
		t.Fatal(err)
	}
	return proc
}

func TestCountRecordsWithoutCSV(t *testing.T) {
	for _, tc := range []struct {
		name      string
		args      []string
		hasHeader bool
		count     int
		want      [][]string
	}{
		{"empty", nil, false, 0, [][]string{{"0"}}},
		{"data with output header", []string{"-k"}, false, 2, [][]string{{"row count"}, {"2"}}},
		{"header excluded", []string{"-j", "-k"}, true, 3, [][]string{{"row count"}, {"3"}}},
		{"header only", []string{"-j"}, true, 0, [][]string{{"0"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got [][]string
			proc := newTestCounter(t, tc.args, func(row []string) error { got = append(got, row); return nil })
			if tc.hasHeader {
				if stop, err := proc.ProcessHeader([]string{"name"}); err != nil || stop {
					t.Fatalf("header: %v, %v", stop, err)
				}
			}
			for i := 0; i < tc.count; i++ {
				if stop, err := proc.ProcessRow([]string{"value"}); err != nil || stop {
					t.Fatalf("row: %v, %v", stop, err)
				}
			}
			if err := proc.Complete(); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("records %v, want %v", got, tc.want)
			}
		})
	}
}

func TestCounterHeaderValidation(t *testing.T) {
	discard := func([]string) error { return nil }
	missing := newTestCounter(t, []string{"-j"}, discard)
	if stop, err := missing.ProcessRow([]string{"value"}); err == nil || !stop {
		t.Fatal("accepted data before required header")
	}
	if err := missing.Complete(); err == nil {
		t.Fatal("completed without required header")
	}
	unexpected := newTestCounter(t, nil, discard)
	if stop, err := unexpected.ProcessHeader([]string{"name"}); err == nil || !stop {
		t.Fatal("accepted unexpected header")
	}
	duplicate := newTestCounter(t, []string{"-j"}, discard)
	if _, err := duplicate.ProcessHeader([]string{"name"}); err != nil {
		t.Fatal(err)
	}
	if stop, err := duplicate.ProcessHeader([]string{"name"}); err == nil || !stop {
		t.Fatal("accepted duplicate header")
	}
}

func TestCounterStopsOnSinkFailure(t *testing.T) {
	for _, failAt := range []int{1, 2} {
		t.Run(map[int]string{1: "header", 2: "count"}[failAt], func(t *testing.T) {
			failure := errors.New("sink failed")
			calls := 0
			proc := newTestCounter(t, []string{"-k"}, func([]string) error {
				calls++
				if calls == failAt {
					return failure
				}
				return nil
			})
			if _, err := proc.ProcessRow([]string{"value"}); err != nil {
				t.Fatal(err)
			}
			if err := proc.Complete(); !errors.Is(err, failure) {
				t.Fatalf("lost sink error: %v", err)
			}
			if calls != failAt {
				t.Fatalf("emitted after sink failure: %d calls", calls)
			}
		})
	}
}

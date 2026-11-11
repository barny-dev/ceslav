package sort

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/barny-dev/ceslav/internal/utilities/processor"
)

func newTestSort(t *testing.T, args []string, sink processor.Sink) processor.Processor {
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

func TestSortRecordsWithoutCSV(t *testing.T) {
	cases := []struct {
		name         string
		args, header []string
		rows, want   [][]string
	}{
		{"requested named column", []string{"-j", "-k", "-b", "+d%amount"}, []string{"name", "amount"}, [][]string{{"2", "10"}, {"1", "20"}}, [][]string{{"name", "amount"}, {"2", "10"}, {"1", "20"}}},
		{"mixed key directions", []string{"-b", "+s#0,-d#1"}, nil, [][]string{{"a", "1"}, {"b", "9"}, {"a", "3"}}, [][]string{{"a", "3"}, {"a", "1"}, {"b", "9"}}},
		{"exact numeric order", []string{"-b", "+d#0"}, nil, [][]string{{"0.10000000000000000002"}, {"1/10"}, {"0.10000000000000000001"}}, [][]string{{"1/10"}, {"0.10000000000000000001"}, {"0.10000000000000000002"}}},
		{"header only", []string{"-j", "-k", "-b", "+s%city"}, []string{"city"}, nil, [][]string{{"city"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got [][]string
			proc := newTestSort(t, tc.args, func(row []string) error {
				got = append(got, append([]string(nil), row...))
				return nil
			})
			if tc.header != nil {
				if stop, err := proc.ProcessHeader(tc.header); err != nil || stop {
					t.Fatalf("header: stop %v, error %v", stop, err)
				}
			}
			for _, row := range tc.rows {
				if stop, err := proc.ProcessRow(row); err != nil || stop {
					t.Fatalf("row: stop %v, error %v", stop, err)
				}
			}
			if err := proc.Complete(); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("records: %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSortRetainsItsOwnHeaderAndRows(t *testing.T) {
	var got [][]string
	proc := newTestSort(t, []string{"-j", "-k", "-b", "+s#0"}, func(row []string) error {
		got = append(got, append([]string(nil), row...))
		return nil
	})
	header, row := []string{"city", "value"}, []string{"Zurich", "1"}
	if _, err := proc.ProcessHeader(header); err != nil {
		t.Fatal(err)
	}
	if _, err := proc.ProcessRow(row); err != nil {
		t.Fatal(err)
	}
	header[0], row[0] = "overwritten header", "overwritten row"
	if err := proc.Complete(); err != nil {
		t.Fatal(err)
	}
	want := [][]string{{"city", "value"}, {"Zurich", "1"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("caller mutation changed output: %v", got)
	}
}

func TestSortRejectsInvalidSingletonBeforeEmission(t *testing.T) {
	cases := []struct {
		key       string
		row       []string
		wantError string
	}{
		{"+d#0", []string{"invalid"}, "invalid number"},
		{"+d#0", []string{""}, "invalid number"},
		{"+d#0", []string{"1/0"}, "invalid number"},
		{"+s#5", []string{"one"}, "out of range"},
	}
	for _, tc := range cases {
		t.Run(tc.key+"/"+strings.Join(tc.row, ","), func(t *testing.T) {
			emitted := false
			proc := newTestSort(t, []string{"-b", tc.key}, func([]string) error { emitted = true; return nil })
			stop, err := proc.ProcessRow(tc.row)
			if !stop || err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("stop %v, error %v", stop, err)
			}
			if emitted {
				t.Fatal("invalid record was emitted")
			}
		})
	}
}

func TestSortStopsOnSinkFailure(t *testing.T) {
	for _, failAt := range []int{1, 2} {
		t.Run(map[int]string{1: "header", 2: "data"}[failAt], func(t *testing.T) {
			failure := errors.New("sink failed")
			calls := 0
			proc := newTestSort(t, []string{"-j", "-k", "-b", "+s#0"}, func([]string) error {
				calls++
				if calls == failAt {
					return failure
				}
				return nil
			})
			if _, err := proc.ProcessHeader([]string{"key"}); err != nil {
				t.Fatal(err)
			}
			for _, row := range [][]string{{"c"}, {"a"}, {"b"}} {
				if _, err := proc.ProcessRow(row); err != nil {
					t.Fatal(err)
				}
			}
			if err := proc.Complete(); !errors.Is(err, failure) {
				t.Fatalf("lost sink error: %v", err)
			}
			if calls != failAt {
				t.Fatalf("emitted after failure: %d calls", calls)
			}
		})
	}
}

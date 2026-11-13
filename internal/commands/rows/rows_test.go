package rows

import (
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func newRows(t *testing.T, args []string, sink func([]string) error) *Processor {
	t.Helper()
	cmd := Cmd()
	if err := cmd.ParseFlags(args); err != nil {
		t.Fatal(err)
	}
	proc, err := NewProcessor(cmd, sink)
	if err != nil {
		t.Fatal(err)
	}
	return proc.(*Processor)
}

func drive(proc *Processor, header []string, records [][]string) error {
	stop, err := proc.Start()
	if err != nil {
		return err
	}
	if !stop && header != nil {
		stop, err = proc.ProcessHeader(header)
		if err != nil {
			return err
		}
	}
	for _, row := range records {
		if stop {
			break
		}
		stop, err = proc.ProcessRow(row)
		if err != nil {
			return err
		}
	}
	return proc.Complete()
}

func TestKeepAndDropRows(t *testing.T) {
	records := [][]string{{"1"}, {"2"}, {"3"}, {"4"}, {"5"}}
	for _, tc := range []struct {
		name        string
		args        []string
		header      []string
		input, want [][]string
	}{
		{"first", []string{"--first", "2"}, nil, records, records[:2]},
		{"drop first", []string{"--first", "2", "--drop"}, nil, records, records[2:]},
		{"last", []string{"--last", "2"}, nil, records, records[3:]},
		{"drop last", []string{"--last", "2", "--drop"}, nil, records, records[:3]},
		{"first zero", []string{"--first", "0"}, nil, records, nil},
		{"last zero", []string{"--last", "0"}, nil, records, nil},
		{"drop first zero", []string{"--first", "0", "--drop"}, nil, records, records},
		{"drop last zero", []string{"--last", "0", "--drop"}, nil, records, records},
		{"oversized first", []string{"--first", "10"}, nil, records, records},
		{"oversized last", []string{"--last", "10"}, nil, records, records},
		{"oversized drop first", []string{"--first", "10", "--drop"}, nil, records, nil},
		{"oversized drop last", []string{"--last", "10", "--drop"}, nil, records, nil},
		{"empty", []string{"--last", "2"}, nil, nil, nil},
		{"header not counted", []string{"--first", "1", "-j", "-k"}, []string{"id"}, records, [][]string{{"id"}, {"1"}}},
		{"zero with header", []string{"--last", "0", "-j", "-k"}, []string{"id"}, records, [][]string{{"id"}}},
		{"header only", []string{"--last", "2", "-j", "-k"}, []string{"id"}, nil, [][]string{{"id"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got [][]string
			proc := newRows(t, tc.args, func(row []string) error { got = append(got, append([]string(nil), row...)); return nil })
			if err := drive(proc, tc.header, tc.input); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSuffixBufferOwnsRowsAndPreservesOrder(t *testing.T) {
	for _, drop := range []bool{false, true} {
		args := []string{"--last", "3"}
		if drop {
			args = append(args, "--drop")
		}
		var got []string
		proc := newRows(t, args, func(row []string) error { got = append(got, row[0]); return nil })
		row := []string{""}
		for i := 1; i <= 8; i++ {
			row[0] = strconv.Itoa(i)
			if _, err := proc.ProcessRow(row); err != nil {
				t.Fatal(err)
			}
			if len(proc.buffer) > 3 {
				t.Fatal("buffer exceeded N")
			}
		}
		if err := proc.Complete(); err != nil {
			t.Fatal(err)
		}
		want := []string{"6", "7", "8"}
		if drop {
			want = []string{"1", "2", "3", "4", "5"}
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("drop=%v: %v, want %v", drop, got, want)
		}
	}
}

func TestHugeLimitAllocatesOnlyForObservedRows(t *testing.T) {
	limit := strconv.Itoa(int(^uint(0) >> 1))
	proc := newRows(t, []string{"--last", limit}, func([]string) error { return nil })
	if len(proc.buffer) != 0 {
		t.Fatal("buffer allocated before input")
	}
	for i := 0; i < 3; i++ {
		if _, err := proc.ProcessRow([]string{"value"}); err != nil {
			t.Fatal(err)
		}
	}
	if len(proc.buffer) != 3 || cap(proc.buffer) > 6 {
		t.Fatal("buffer not bounded by observed rows")
	}
}

func TestRowsConfigurationErrors(t *testing.T) {
	for _, args := range [][]string{nil, {"--drop"}, {"--first", "1", "--last", "2"}, {"--first", "-1"}, {"--last", "-1"}, {"--first", "0", "-k"}, {"--first", "9999999999999999999999999"}, {""}, {"0"}, {"1-"}, {"1", "--first", "2"}, {"--last", "1", "2"}, {"--reverse", "--drop"}, {"1", "2"}, {"--reverse=false"}, {"--reverse", "-k"}} {
		cmd := Cmd()
		if err := cmd.ParseFlags(args); err != nil {
			continue
		}
		if _, err := NewProcessor(cmd, func([]string) error { return nil }); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}

func TestRowSelectorsAndReverse(t *testing.T) {
	records := [][]string{{"1"}, {"2"}, {"3"}, {"4"}, {"5"}, {"6"}, {"7"}}
	for _, tc := range []struct {
		name, want string
		args       []string
	}{
		{"selector list", "1,3,4,7", []string{"1,3-4,7"}},
		{"descending range", "7,4,3,1", []string{"7,4-3,1"}},
		{"repeated rows", "3,1,3,4", []string{"3,1,3-4"}},
		{"reverse selectors", "1,3,4,7", []string{"7,4-3,1", "--reverse"}},
		{"drop selectors", "2,5,6", []string{"1,3-4,7", "--drop"}},
		{"drop overlaps", "1,6,7", []string{"4-2,3-5", "--drop"}},
		{"reverse drop selectors", "6,5,2", []string{"1,3-4,7", "--drop", "--reverse"}},
		{"clip ascending", "6,7", []string{"6-10"}},
		{"clip descending", "7,6", []string{"10-6"}},
		{"clip reversed", "6,7", []string{"10-6", "--reverse"}},
		{"missing selectors", "", []string{"8-10,20-15"}},
		{"drop missing selectors", "1,2,3,4,5,6,7", []string{"8", "--drop"}},
		{"reverse all", "7,6,5,4,3,2,1", []string{"--reverse"}},
		{"reverse first", "3,2,1", []string{"--first", "3", "--reverse"}},
		{"reverse last wrapped", "7,6,5", []string{"--last", "3", "--reverse"}},
		{"reverse last oversized", "7,6,5,4,3,2,1", []string{"--last", "10", "--reverse"}},
		{"reverse drop first", "7,6,5,4", []string{"--first", "3", "--drop", "--reverse"}},
		{"reverse drop last", "4,3,2,1", []string{"--last", "3", "--drop", "--reverse"}},
		{"reverse first zero", "", []string{"--first", "0", "--reverse"}},
		{"reverse last zero", "", []string{"--last", "0", "--reverse"}},
		{"reverse drop zero", "7,6,5,4,3,2,1", []string{"--last", "0", "--drop", "--reverse"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			proc := newRows(t, tc.args, func(row []string) error { got = append(got, row[0]); return nil })
			if err := drive(proc, nil, records); err != nil {
				t.Fatal(err)
			}
			if strings.Join(got, ",") != tc.want {
				t.Fatalf("got %v, want %s", got, tc.want)
			}
		})
	}
}

func TestSelectionAndReverseHeadersAndEmptyInput(t *testing.T) {
	for _, args := range [][]string{{"2,1"}, {"2-1", "--reverse"}, {"--reverse"}, {"2", "--drop", "--reverse"}} {
		for _, records := range [][][]string{nil, {{"1"}, {"2"}}} {
			var got []string
			withHeader := append(append([]string(nil), args...), "-j", "-k")
			proc := newRows(t, withHeader, func(row []string) error { got = append(got, row[0]); return nil })
			if err := drive(proc, []string{"header"}, records); err != nil {
				t.Fatal(err)
			}
			if len(got) == 0 || got[0] != "header" {
				t.Fatalf("%v moved header: %v", args, got)
			}
			if records == nil && len(got) != 1 {
				t.Fatalf("emitted nonexistent data: %v", got)
			}
		}
		proc := newRows(t, args, func([]string) error { t.Fatal("emitted data for empty input"); return nil })
		if err := drive(proc, nil, nil); err != nil {
			t.Fatal(err)
		}
	}
}

func TestReorderedRowsOwnInput(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"3,1,2,1"}, "3,1,2,1"},
		{[]string{"--reverse"}, "3,2,1"},
		{[]string{"--first", "2", "--reverse"}, "2,1"},
		{[]string{"--last", "2", "--reverse"}, "3,2"},
		{[]string{"1", "--drop", "--reverse"}, "3,2"},
	} {
		var got []string
		proc := newRows(t, tc.args, func(row []string) error { got = append(got, row[0]); return nil })
		row := []string{""}
		for i := 1; i <= 3; i++ {
			row[0] = strconv.Itoa(i)
			stop, err := proc.ProcessRow(row)
			if err != nil {
				t.Fatal(err)
			}
			if stop {
				break
			}
		}
		row[0] = "overwritten"
		if err := proc.Complete(); err != nil {
			t.Fatal(err)
		}
		if strings.Join(got, ",") != tc.want {
			t.Fatalf("%v: got %v, want %s", tc.args, got, tc.want)
		}
	}
}

func TestSparseSelectionBuffersOnlyMatches(t *testing.T) {
	var got []string
	proc := newRows(t, []string{"9223372036854775807,2"}, func(row []string) error { got = append(got, row[0]); return nil })
	for i := 1; i <= 5; i++ {
		if _, err := proc.ProcessRow([]string{strconv.Itoa(i)}); err != nil {
			t.Fatal(err)
		}
	}
	if len(proc.selectedRows) != 1 || len(proc.buffer) != 0 || len(proc.reversed) != 0 {
		t.Fatal("buffered unselected rows")
	}
	if err := proc.Complete(); err != nil || strings.Join(got, ",") != "2" {
		t.Fatalf("got %v, error %v", got, err)
	}
}

func TestSelectionAndReverseSinkErrors(t *testing.T) {
	failure := errors.New("sink failed")
	for _, args := range [][]string{{"1"}, {"2", "--drop"}, {"1", "--reverse"}, {"2", "--drop", "--reverse"}, {"--reverse"}, {"--first", "1", "--reverse"}, {"--last", "1", "--reverse"}, {"--last", "1", "--drop", "--reverse"}} {
		proc := newRows(t, args, func([]string) error { return failure })
		if err := drive(proc, nil, [][]string{{"a"}, {"b"}, {"c"}}); !errors.Is(err, failure) {
			t.Fatalf("%v lost sink error: %v", args, err)
		}
	}
}

func TestRowsSinkErrors(t *testing.T) {
	failure := errors.New("sink failed")
	for _, args := range [][]string{{"--first", "2"}, {"--first", "0", "--drop"}, {"--last", "0", "--drop"}, {"--last", "1"}, {"--last", "1", "--drop"}} {
		proc := newRows(t, args, func([]string) error { return failure })
		if err := drive(proc, nil, [][]string{{"a"}, {"b"}, {"c"}}); !errors.Is(err, failure) {
			t.Fatalf("%v lost sink error: %v", args, err)
		}
	}
	proc := newRows(t, []string{"--first", "0", "-j", "-k"}, func([]string) error { return failure })
	if err := drive(proc, []string{"header"}, nil); !errors.Is(err, failure) {
		t.Fatalf("lost header sink error: %v", err)
	}
}

func TestDropLastEmitsBeforeCompletion(t *testing.T) {
	emitted := 0
	proc := newRows(t, []string{"--last", "2", "--drop"}, func([]string) error { emitted++; return nil })
	for i := 0; i < 3; i++ {
		if _, err := proc.ProcessRow([]string{"value"}); err != nil {
			t.Fatal(err)
		}
	}
	if emitted != 1 {
		t.Fatalf("did not stream buffered prefix: %d", emitted)
	}
	if err := proc.Complete(); err != nil {
		t.Fatal(err)
	}
	if emitted != 1 {
		t.Fatal("emitted the dropped suffix")
	}
}

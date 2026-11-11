package rows

import (
	"errors"
	"reflect"
	"strconv"
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
	for _, args := range [][]string{nil, {"--drop"}, {"--first", "1", "--last", "2"}, {"--first", "-1"}, {"--last", "-1"}, {"--first", "0", "-k"}, {"--first", "9999999999999999999999999"}} {
		cmd := Cmd()
		if err := cmd.ParseFlags(args); err != nil {
			continue
		}
		if _, err := NewProcessor(cmd, func([]string) error { return nil }); err == nil {
			t.Fatalf("accepted %v", args)
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

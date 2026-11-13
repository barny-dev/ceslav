package projection

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/barny-dev/ceslav/internal/utilities/processor"
)

func newProjection(t *testing.T, drop bool, args []string, sink processor.Sink) processor.Processor {
	t.Helper()
	cmd := SelectCmd()
	if drop {
		cmd = DropCmd()
	}
	if err := cmd.ParseFlags(args); err != nil {
		t.Fatal(err)
	}
	proc, err := NewProcessor(cmd, sink, drop)
	if err != nil {
		t.Fatal(err)
	}
	return proc
}

func TestProjection(t *testing.T) {
	cases := []struct {
		name         string
		drop         bool
		args, header []string
		rows, want   [][]string
	}{
		{"reorder names and indices", false, []string{"-j", "-k", "-c", "%city", "-c", "#0"}, []string{"name", "age", "city"}, [][]string{{"A", "30", "Gdansk"}}, [][]string{{"city", "name"}, {"Gdansk", "A"}}},
		{"drop retains remaining order", true, []string{"-j", "-k", "-c", "%age"}, []string{"name", "age", "city"}, [][]string{{"A", "30", "Gdansk"}}, [][]string{{"name", "city"}, {"A", "Gdansk"}}},
		{"headerless selection", false, []string{"-c", "#2", "-c", "#0"}, nil, [][]string{{"a", "b", "c"}}, [][]string{{"c", "a"}}},
		{"headerless drop", true, []string{"-c", "#1"}, nil, [][]string{{"a", "b", "c"}}, [][]string{{"a", "c"}}},
		{"comma in header name", false, []string{"-j", "-k", "-c", "%unit,price"}, []string{"unit,price", "other"}, [][]string{{"10", "x"}}, [][]string{{"unit,price"}, {"10"}}},
		{"duplicate header by index", false, []string{"-j", "-k", "-c", "#1"}, []string{"same", "same"}, [][]string{{"a", "b"}}, [][]string{{"same"}, {"b"}}},
		{"header omitted", false, []string{"-j", "-c", "%name"}, []string{"name", "age"}, [][]string{{"A", "30"}}, [][]string{{"A"}}},
		{"header only", false, []string{"-j", "-k", "-c", "#1"}, []string{"name", "age"}, nil, [][]string{{"age"}}},
		{"empty input", false, []string{"-c", "#0"}, nil, nil, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got [][]string
			proc := newProjection(t, tc.drop, tc.args, func(row []string) error { got = append(got, append([]string(nil), row...)); return nil })
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
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestProjectionConfigurationErrors(t *testing.T) {
	for _, args := range [][]string{nil, {"-c", "bad"}, {"-c", "%name"}, {"-c", "#0", "-c", "#0"}, {"-k", "-c", "#0"}} {
		cmd := SelectCmd()
		if err := cmd.ParseFlags(args); err != nil {
			t.Fatal(err)
		}
		if _, err := NewProcessor(cmd, func([]string) error { return nil }, false); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}

func TestProjectionValidatesBeforeEmitting(t *testing.T) {
	cases := []struct {
		name         string
		drop         bool
		args, header []string
		wantError    string
	}{
		{"unknown name", false, []string{"-j", "-k", "-c", "%missing"}, []string{"name"}, "not found"},
		{"ambiguous name", false, []string{"-j", "-k", "-c", "%name"}, []string{"name", "name"}, "ambiguous"},
		{"alias duplicate", false, []string{"-j", "-k", "-c", "%name", "-c", "#0"}, []string{"name", "age"}, "duplicate column"},
		{"drop alias duplicate", true, []string{"-j", "-k", "-c", "%name", "-c", "#0"}, []string{"name", "age"}, "duplicate column"},
		{"header index", false, []string{"-j", "-k", "-c", "#2"}, []string{"name"}, "out of range"},
		{"drop all", true, []string{"-j", "-k", "-c", "#0"}, []string{"name"}, "no columns"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			emitted := false
			proc := newProjection(t, tc.drop, tc.args, func([]string) error { emitted = true; return nil })
			stop, err := proc.ProcessHeader(tc.header)
			if !stop || err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("stop %v, error %v", stop, err)
			}
			if emitted {
				t.Fatal("emitted invalid projection")
			}
		})
	}
	for _, drop := range []bool{false, true} {
		emitted := false
		proc := newProjection(t, drop, []string{"-c", "#2"}, func([]string) error { emitted = true; return nil })
		stop, err := proc.ProcessRow([]string{"one"})
		if !stop || err == nil || emitted {
			t.Fatalf("invalid singleton: stop %v, error %v, emitted %v", stop, err, emitted)
		}
	}
}

func TestProjectionRejectsChangedWidth(t *testing.T) {
	calls := 0
	proc := newProjection(t, false, []string{"-c", "#0"}, func([]string) error { calls++; return nil })
	if _, err := proc.ProcessRow([]string{"a", "b"}); err != nil {
		t.Fatal(err)
	}
	if stop, err := proc.ProcessRow([]string{"c"}); err == nil || !stop {
		t.Fatal("accepted changed width")
	}
	if calls != 1 {
		t.Fatal("emitted malformed row")
	}
}

func TestProjectionPropagatesSinkErrors(t *testing.T) {
	failure := errors.New("sink failed")
	headerProc := newProjection(t, false, []string{"-j", "-k", "-c", "#0"}, func([]string) error { return failure })
	if stop, err := headerProc.ProcessHeader([]string{"name"}); !stop || !errors.Is(err, failure) {
		t.Fatalf("header: %v, %v", stop, err)
	}
	rowProc := newProjection(t, false, []string{"-c", "#0"}, func([]string) error { return failure })
	if stop, err := rowProc.ProcessRow([]string{"value"}); !stop || !errors.Is(err, failure) {
		t.Fatalf("row: %v, %v", stop, err)
	}
}

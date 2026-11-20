package projection

import (
	"strconv"
	"strings"
	"testing"
)

func TestCompactSelectorSyntax(t *testing.T) {
	for _, tc := range []struct {
		input, first, last string
	}{
		{"name", "%name", "%name"},
		{"%name", "%name", "%name"},
		{"0", "%0", "%0"},
		{"first-name", "%first-name", "%first-name"},
		{"#0", "#0", "#0"},
		{"#0-2", "#0", "#2"},
		{"#2-#0", "#2", "#0"},
	} {
		selector, err := parseCompactSelector(tc.input)
		if err != nil || selector.first.String() != tc.first || selector.last.String() != tc.last {
			t.Errorf("%q: got %v, error %v; want %s through %s", tc.input, selector, err, tc.first, tc.last)
		}
	}
	for _, token := range []string{"", "%", "#", "#-1", "#1-", "#1--2", "#1-2-3", "#x", "#1.5", "#0-x", "#0-999999999999999999999999"} {
		if _, err := parseCompactSelector(token); err == nil {
			t.Errorf("accepted %q", token)
		}
	}
}

func TestHugeColumnRangeIsValidatedBeforeExpansion(t *testing.T) {
	limit := strconv.Itoa(int(^uint(0) >> 1))
	for _, drop := range []bool{false, true} {
		emitted := false
		proc := newProjection(t, drop, []string{"#0-" + limit}, func([]string) error { emitted = true; return nil })
		if stop, err := proc.ProcessRow([]string{"a", "b"}); !stop || err == nil || !strings.Contains(err.Error(), "out of range") || emitted {
			t.Fatalf("stop=%v, error=%v, emitted=%v", stop, err, emitted)
		}
		// Unknown-width empty input must neither expand the range nor invent a width.
		empty := newProjection(t, drop, []string{"#0-" + limit}, func([]string) error { t.Fatal("emitted empty input"); return nil })
		if err := empty.Complete(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestHugeOverlappingRangesStayCompactBeforeInput(t *testing.T) {
	limit := strconv.Itoa(int(^uint(0) >> 1))
	for _, drop := range []bool{false, true} {
		command := SelectCmd()
		if drop {
			command = DropCmd()
		}
		if err := command.ParseFlags([]string{"#0-" + limit + ",#" + limit + "-1"}); err != nil {
			t.Fatal(err)
		}
		_, err := NewProcessor(command, func([]string) error { t.Fatal("emitted before input"); return nil }, drop)
		if drop {
			if err != nil {
				t.Fatalf("drop rejected overlapping ranges: %v", err)
			}
		} else if err == nil || !strings.Contains(err.Error(), "duplicate column") {
			t.Fatalf("select accepted unrenamed overlapping ranges: %v", err)
		}
	}
}

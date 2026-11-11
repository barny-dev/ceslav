package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCommandRegressions(t *testing.T) {
	cases := []struct {
		name                   string
		args                   []string
		input, want, wantError string
	}{
		{"named value beside name", []string{"sort", "-j", "-k", "-b", "+d%value"}, "name,value\n2,10\n1,20\n", "name,value\n2,10\n1,20\n", ""},
		{"arbitrary named header", []string{"sort", "-j", "-k", "-b", "+s%city"}, "city,value\nZurich,1\nAthens,2\n", "city,value\nAthens,2\nZurich,1\n", ""},
		{"multiple mixed keys", []string{"sort", "-b", "+s#0,-d#1"}, "a,1\nb,9\na,3\n", "a,3\na,1\nb,9\n", ""},
		{"exact rational numbers", []string{"sort", "-b", "+d#0"}, "0.10000000000000000002\n1/10\n0.10000000000000000001\n", "1/10\n0.10000000000000000001\n0.10000000000000000002\n", ""},
		{"multiline CSV", []string{"row-count", "-j", "-k"}, "name,note\nAlice,\"two\nlines\"\nBob,plain\n", "row count\n2\n", ""},
		{"header retained after reading", []string{"sort", "-j", "-k", "-b", "+s#0"}, "city,value\nZurich,1\nAthens,2\n", "city,value\nAthens,2\nZurich,1\n", ""},
		{"empty count", []string{"row-count"}, "", "0\n", ""},
		{"empty sort", []string{"sort", "-b", "+s#0"}, "", "", ""},
		{"header only", []string{"sort", "-j", "-k", "-b", "+s%city"}, "city,value\n", "city,value\n", ""},
		{"duplicate names by index", []string{"sort", "-j", "-k", "-b", "+d#1"}, "v,v\nx,2\ny,1\n", "v,v\ny,1\nx,2\n", ""},
		{"unknown name", []string{"sort", "-j", "-b", "+s%missing"}, "city,value\nZurich,1\n", "", "column %missing not found"},
		{"duplicate named key", []string{"sort", "-j", "-b", "+s%v"}, "v,v\na,b\n", "", "ambiguous"},
		{"name without header", []string{"sort", "-b", "+s%city"}, "Zurich,1\n", "", "requires --header"},
		{"missing header", []string{"row-count", "-j"}, "", "", "header expected"},
		{"missing sort header", []string{"sort", "-j", "-b", "+s#0"}, "", "", "header expected"},
		{"output header without input header", []string{"sort", "-k", "-b", "+s#0"}, "a\n", "", "--output-header requires --header"},
		{"index out of range", []string{"sort", "-b", "+s#5"}, "a,b\nc,d\n", "", "record 1: column index #5 out of range"},
		{"singleton index out of range", []string{"sort", "-b", "+s#5"}, "a,b\n", "", "record 1: column index #5 out of range"},
		{"index against header only", []string{"sort", "-j", "-b", "+s#2"}, "a,b\n", "", "column index #2 out of range"},
		{"invalid numeric data", []string{"sort", "-b", "+d#0"}, "5\ninvalid\n-2\n", "", "record 2: column #0: invalid number"},
		{"singleton invalid numeric data", []string{"sort", "-b", "+d#0"}, "invalid\n", "", "record 1: column #0: invalid number"},
		{"empty numeric field", []string{"sort", "-j", "-b", "+d%value"}, "name,value\na,\n", "", "record 2: column %value: invalid number"},
		{"malformed CSV", []string{"row-count"}, "a,b\nc\n", "", "input record 2:"},
		{"bad quoting", []string{"row-count"}, "\"unterminated", "", "input record 1:"},
		{"no sort keys", []string{"sort"}, "a\n", "", "at least one --sort-by key"},
		{"unsupported type", []string{"sort", "-b", "+t#0"}, "a\n", "", "column type specifier"},
		{"invalid direction", []string{"sort", "-b", "xs#0"}, "a\n", "", "sort direction specifier"},
		{"negative index", []string{"sort", "-b", "+s#-1"}, "a\n", "", "nonnegative integer"},
		{"overflowing index", []string{"sort", "-b", "+s#999999999999999999999999999"}, "a\n", "", "nonnegative integer"},
		{"empty selector", []string{"sort", "-b", "+s%"}, "a\n", "", "sort key too short"},
		{"unexpected argument", []string{"row-count", "extra"}, "a\n", "", "unknown command"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			output, err := WithCommand(testBinary).WithArgs(tc.args...).WithStdin(tc.input).Run()
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("wanted error containing %q, got output %q, error %v", tc.wantError, output, err)
				}
				if strings.Contains(err.Error(), "panic:") {
					t.Fatalf("command panicked: %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if output != tc.want {
				t.Fatalf("wanted output %q, got %q", tc.want, output)
			}
		})
	}
}

func TestOutputFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "count.csv")
	if _, err := WithCommand(testBinary).WithArgs("row-count", "-k", "-o", path).WithStdin("a\nb\n").Run(); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "row count\n2\n" {
		t.Fatalf("output: %q", contents)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0600 != 0600 {
		t.Fatalf("output lacks owner read/write permissions: %v", info.Mode())
	}
}

func TestOutputNotTruncatedOnInvalidInput(t *testing.T) {
	cases := []struct {
		name  string
		args  []string
		input string
	}{
		{"bad configuration", []string{"sort", "-b", "+t#0"}, "a\n"},
		{"bad column name", []string{"sort", "-j", "-b", "+s%missing"}, "city\nZurich\n"},
		{"bad index", []string{"sort", "-b", "+s#3"}, "a\n"},
		{"bad CSV", []string{"row-count"}, "a,b\nc\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "existing.csv")
			if err := os.WriteFile(path, []byte("preserve me\n"), 0600); err != nil {
				t.Fatal(err)
			}
			args := append(append([]string(nil), tc.args...), "-o", path)
			if _, err := WithCommand(testBinary).WithArgs(args...).WithStdin(tc.input).Run(); err == nil {
				t.Fatal("expected failure")
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != "preserve me\n" {
				t.Fatalf("destination modified: %q", data)
			}
		})
	}
}

func TestSameInputOutputRejected(t *testing.T) {
	for _, alias := range []string{"same path", "hard link", "symlink"} {
		t.Run(alias, func(t *testing.T) {
			directory := t.TempDir()
			input := filepath.Join(directory, "input.csv")
			const contents = "name,value\nA,1\n"
			if err := os.WriteFile(input, []byte(contents), 0600); err != nil {
				t.Fatal(err)
			}
			output := input
			if alias != "same path" {
				output = filepath.Join(directory, "output.csv")
				var err error
				if alias == "hard link" {
					err = os.Link(input, output)
				} else {
					err = os.Symlink(input, output)
				}
				if err != nil {
					t.Skipf("%s unavailable: %v", alias, err)
				}
			}
			_, err := WithCommand(testBinary).WithArgs("row-count", "-i", input, "-o", output).Run()
			if err == nil || !strings.Contains(err.Error(), "same file") {
				t.Fatalf("expected same-file error, got %v", err)
			}
			data, err := os.ReadFile(input)
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != contents {
				t.Fatalf("input modified: %q", data)
			}
		})
	}
}

func TestSuccessfulEmptySortTruncatesOutput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "output.csv")
	if err := os.WriteFile(path, []byte("old data\n"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := WithCommand(testBinary).WithArgs("sort", "-b", "+s#0", "-o", path).WithStdin("").Run()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 0 {
		t.Fatalf("expected empty output, got %q", data)
	}
}

func TestOutputFailureIsReported(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("/dev/full is a Linux device")
	}
	if _, err := os.Stat("/dev/full"); err != nil {
		t.Skip("/dev/full unavailable")
	}
	_, err := WithCommand(testBinary).WithArgs("row-count", "-o", "/dev/full").WithStdin("a\n").Run()
	if err == nil || !strings.Contains(err.Error(), "flush output") {
		t.Fatalf("expected flush error, got %v", err)
	}
}

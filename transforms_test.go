package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTransformCommands(t *testing.T) {
	cases := []struct {
		name                   string
		args                   []string
		input, want, wantError string
	}{
		{"row selector list", []string{"rows", "1,3-4,7"}, "1\n2\n3\n4\n5\n6\n7\n", "1\n3\n4\n7\n", ""},
		{"reorder descending ranges", []string{"rows", "5,3-1"}, "1\n2\n3\n4\n5\n", "5\n3\n2\n1\n", ""},
		{"repeat rows", []string{"rows", "2,1-2"}, "a\nb\n", "b\na\nb\n", ""},
		{"drop row list", []string{"rows", "4-3,1", "--drop"}, "1\n2\n3\n4\n5\n", "2\n5\n", ""},
		{"reverse dropped rows", []string{"rows", "4-3,1", "--drop", "--reverse"}, "1\n2\n3\n4\n5\n", "5\n2\n", ""},
		{"reverse all rows", []string{"rows", "--reverse"}, "a\nb\nc\n", "c\nb\na\n", ""},
		{"reverse selector order", []string{"rows", "3,1-2", "--reverse"}, "a\nb\nc\n", "b\na\nc\n", ""},
		{"reverse first rows", []string{"rows", "--first", "2", "--reverse"}, "a\nb\nc\n", "b\na\n", ""},
		{"reverse last rows", []string{"rows", "--last", "2", "--reverse"}, "a\nb\nc\nd\n", "d\nc\n", ""},
		{"reverse keeps header first", []string{"rows", "--reverse", "-j", "-k"}, "id\na\nb\n", "id\nb\na\n", ""},
		{"selectors exclude header", []string{"rows", "2,1", "-j", "-k"}, "id\na\nb\n", "id\nb\na\n", ""},
		{"selected header omitted", []string{"rows", "2,1", "-j"}, "id\na\nb\n", "b\na\n", ""},
		{"selectors count records", []string{"rows", "2,1"}, "\"two\nlines\"\nlast\n", "last\n\"two\nlines\"\n", ""},
		{"reverse preserves empty cell", []string{"rows", "--reverse"}, "first\n\"\"\nlast\n", "last\n\"\"\nfirst\n", ""},
		{"selector clips huge range", []string{"rows", "9223372036854775807-1"}, "a\nb\n", "b\na\n", ""},
		{"missing rows ignored", []string{"rows", "9,1"}, "a\nb\n", "a\n", ""},
		{"selectors stop before invalid tail", []string{"rows", "2,1"}, "a,b\nc,d\nbroken\n", "c,d\na,b\n", ""},
		{"reverse first skips invalid tail", []string{"rows", "--first", "1", "--reverse"}, "a,b\nbroken\n", "a,b\n", ""},
		{"selectors validate selected prefix", []string{"rows", "2,1"}, "a,b\nbroken\n", "", "input record 2"},
		{"reverse validates all input", []string{"rows", "--reverse"}, "a,b\nbroken\n", "", "input record 2"},
		{"drop selectors validate all input", []string{"rows", "1", "--drop"}, "a,b\nbroken\n", "", "input record 2"},
		{"selector mode conflict", []string{"rows", "1,2", "--first", "2"}, "a\n", "", "exactly one"},
		{"zero row number", []string{"rows", "0"}, "a\n", "", "invalid row selector"},
		{"invalid row range", []string{"rows", "1-"}, "a\n", "", "invalid row selector"},
		{"select empty cells", []string{"select", "-c", "#1"}, "a,\nb,\n", "\"\"\n\"\"\n", ""},
		{"drop leaves empty cells", []string{"drop", "-c", "#0"}, "a,\nb,\n", "\"\"\n\"\"\n", ""},
		{"rows preserve empty cells", []string{"rows", "--last", "3"}, "first\n\"\"\nlast\n", "first\n\"\"\nlast\n", ""},
		{"select reorders", []string{"select", "-j", "-k", "-c", "%city", "-c", "#0"}, "name,age,city\nA,30,Gdansk\n", "city,name\nGdansk,A\n", ""},
		{"drop by name", []string{"drop", "-j", "-k", "-c", "%age"}, "name,age,city\nA,30,Gdansk\n", "name,city\nA,Gdansk\n", ""},
		{"comma selector", []string{"select", "-j", "-k", "-c", "%unit,price"}, "\"unit,price\",other\n10,x\n", "\"unit,price\"\n10\n", ""},
		{"select indices", []string{"select", "-c", "#2", "-c", "#0"}, "a,b,c\n", "c,a\n", ""},
		{"drop indices", []string{"drop", "-c", "#1"}, "a,b,c\n", "a,c\n", ""},
		{"drop all", []string{"drop", "-c", "#0"}, "one\n", "", "no columns"},
		{"duplicate alias", []string{"select", "-j", "-c", "#0", "-c", "%name"}, "name,age\nA,30\n", "", "duplicate column"},
		{"ambiguous column", []string{"select", "-j", "-c", "%name"}, "name,name\na,b\n", "", "ambiguous"},
		{"singleton bad index", []string{"select", "-c", "#2"}, "a,b\n", "", "out of range"},
		{"partial streaming output", []string{"select", "-c", "#0"}, "a,b\nc\n", "a\n", "input record 2"},
		{"first rows", []string{"rows", "--first", "2"}, "1\n2\n3\n4\n", "1\n2\n", ""},
		{"drop first rows", []string{"rows", "--first", "2", "--drop"}, "1\n2\n3\n4\n", "3\n4\n", ""},
		{"last rows", []string{"rows", "--last", "2"}, "1\n2\n3\n4\n", "3\n4\n", ""},
		{"drop last rows", []string{"rows", "--last", "2", "--drop"}, "1\n2\n3\n4\n", "1\n2\n", ""},
		{"headers excluded", []string{"rows", "-j", "-k", "--first", "1"}, "id\n1\n2\n", "id\n1\n", ""},
		{"last quoted record", []string{"rows", "--last", "1"}, "\"two\nlines\",1\nplain,2\n", "plain,2\n", ""},
		{"first quoted record", []string{"rows", "--first", "1"}, "\"two\nlines\",1\nplain,2\n", "\"two\nlines\",1\n", ""},
		{"zero skips input", []string{"rows", "--first", "0"}, "\"unterminated", "", ""},
		{"prefix skips invalid tail", []string{"rows", "--first", "1"}, "a,b\nc\n", "a,b\n", ""},
		{"suffix validates input", []string{"rows", "--last", "1"}, "a,b\nc\n", "", "input record 2"},
		{"zero last header", []string{"rows", "-j", "-k", "--last", "0"}, "id\n1\n2\n", "id\n", ""},
		{"negative count", []string{"rows", "--first", "-1"}, "1\n", "", "nonnegative"},
		{"both ends", []string{"rows", "--first", "0", "--last", "0"}, "1\n", "", "exactly one"},
		{"count required", []string{"rows"}, "1\n", "", "exactly one"},
		{"missing selection", []string{"select"}, "1\n", "", "--column"},
		{"missing header at zero", []string{"rows", "-j", "--first", "0"}, "", "", "header expected"},
		{"output header needs input", []string{"rows", "-k", "--first", "0"}, "", "", "requires --header"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := WithCommand(testBinary).WithArgs(tc.args...).WithStdin(tc.input).Run()
			if tc.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantError) {
					t.Fatalf("wanted %q, got %v", tc.wantError, err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestPrefixAndZeroSelectionDoNotWaitForInputEOF(t *testing.T) {
	for _, tc := range []struct {
		name        string
		args        []string
		input, want string
	}{
		{"first one", []string{"rows", "--first", "1"}, "one\n", "one\n"},
		{"first zero", []string{"rows", "--first", "0"}, "", ""},
		{"last zero", []string{"rows", "--last", "0"}, "", ""},
		{"zero with header", []string{"rows", "--first", "0", "-j", "-k"}, "header\n", "header\n"},
		{"selectors", []string{"rows", "3,1"}, "one\ntwo\nthree\n", "three\none\n"},
		{"reversed selectors", []string{"rows", "3,1", "--reverse"}, "one\ntwo\nthree\n", "one\nthree\n"},
		{"reversed prefix", []string{"rows", "--first", "2", "--reverse"}, "one\ntwo\n", "two\none\n"},
		{"reversed zero", []string{"rows", "--first", "0", "--reverse"}, "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, testBinary, tc.args...)
			stdin, err := cmd.StdinPipe()
			if err != nil {
				t.Fatal(err)
			}
			defer stdin.Close()
			var stdout, stderr bytes.Buffer
			cmd.Stdout, cmd.Stderr = &stdout, &stderr
			if err := cmd.Start(); err != nil {
				t.Fatal(err)
			}
			waited := false
			defer func() {
				if !waited {
					cancel()
					_ = cmd.Wait()
				}
			}()
			if tc.input != "" {
				if _, err := stdin.Write([]byte(tc.input)); err != nil {
					t.Fatal(err)
				}
			}
			// Keep stdin open: success must not depend on EOF from the producer.
			err = cmd.Wait()
			waited = true
			if err != nil {
				t.Fatalf("waited for EOF: %v; stderr %s", err, stderr.String())
			}
			if stdout.String() != tc.want {
				t.Fatalf("got %q, want %q", stdout.String(), tc.want)
			}
		})
	}
}

func TestStreamingProjectionRejectsSameInputOutput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "input.csv")
	const data = "name,age\nA,30\nB,40\n"
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := WithCommand(testBinary).WithArgs("select", "-j", "-k", "-c", "%name", "-i", path, "-o", path).Run()
	if err == nil || !strings.Contains(err.Error(), "same file") {
		t.Fatalf("wanted same-file error, got %v", err)
	}
	got, readErr := os.ReadFile(path)
	if readErr != nil || string(got) != data {
		t.Fatalf("input changed: %q, %v", got, readErr)
	}
}

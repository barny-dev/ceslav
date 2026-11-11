package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

var testBinary string

func TestMain(m *testing.M) { os.Exit(runTests(m)) }

func runTests(m *testing.M) int {
	directory, err := os.MkdirTemp("", "ceslav-test-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer os.RemoveAll(directory)
	testBinary = filepath.Join(directory, "ceslav")
	if runtime.GOOS == "windows" {
		testBinary += ".exe"
	}
	build := exec.Command("go", "build", "-o", testBinary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "build integration executable: %v\n%s", err, output)
		return 1
	}
	return m.Run()
}

type RunCommand struct {
	command string
	args    []string
	stdin   *string
}

func WithCommand(cmd string) *RunCommand                  { return &RunCommand{command: cmd} }
func (r *RunCommand) WithArgs(args ...string) *RunCommand { r.args = args; return r }
func (r *RunCommand) WithStdin(stdin string) *RunCommand  { r.stdin = &stdin; return r }

func (r *RunCommand) Run() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, r.command, r.args...)
	if r.stdin != nil {
		cmd.Stdin = strings.NewReader(*r.stdin)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		return string(output), fmt.Errorf("command failed: %w; stderr: %s", err, stderr.String())
	}
	return string(output), nil
}

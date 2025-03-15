package main

import (
	"fmt"
	"io"
	"os/exec"
)

type RunCommand struct {
	command string
	args    []string
	stdin   *string
}

func WithCommand(cmd string) *RunCommand {
	r := RunCommand{command: cmd}
	return &r
}

func (r *RunCommand) WithArgs(args ...string) *RunCommand {
	r.args = args
	return r
}

func (r *RunCommand) WithStdin(stdin string) *RunCommand {
	r.stdin = &stdin
	return r
}

func (r *RunCommand) Run() (string, error) {
	cmd := exec.Command(r.command, r.args...)
	var stdin io.WriteCloser
	if r.stdin != nil {
		_stdin, err := cmd.StdinPipe()
		if err != nil {
			return "", fmt.Errorf("error setting up stdin pipe: %w", err)
		}
		stdin = _stdin
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("error setting up stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("error setting up stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("error starting command: %w", err)
	}

	if r.stdin != nil {
		_, err = io.WriteString(stdin, *r.stdin)
		if err != nil {
			return "", fmt.Errorf("error writing to stdin: %w", err)
		}
		if err := stdin.Close(); err != nil {
			return "", fmt.Errorf("error closing stdin pipe: %w", err)
		}
	}

	var output string
	if bytes, err := io.ReadAll(stdout); err != nil {
		return "", fmt.Errorf("error reading stdout: %w", err)
	} else {
		output = string(bytes)
	}
	var errOutput string
	if bytes, err := io.ReadAll(stderr); err != nil {
		return "", fmt.Errorf("error reading stderr: %w", err)
	} else {
		errOutput = string(bytes)
	}

	if err := cmd.Wait(); err != nil {
		cmdErr := fmt.Errorf("command returned exit error: %w\n*** stderr ***\n%s\n**************", err, errOutput)
		return output, cmdErr
	}
	return output, nil
}

//func CreateTempFile(t *testing.T, contents string) string {
//	file, err := os.CreateTemp(t.TempDir(), t.Name())
//	if err != nil {
//		t.Fatalf("could not create temp file: %v", err)
//	}
//	return file
//}

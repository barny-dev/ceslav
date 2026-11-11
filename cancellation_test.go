package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/barny-dev/ceslav/internal/commands"
)

func TestExecuteContextCancelsIdleStdin(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()
	original := os.Stdin
	os.Stdin = reader
	defer func() { os.Stdin = original }()
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	cmd := commands.Cmd()
	cmd.SetArgs([]string{"row-count"})
	done := make(chan error, 1)
	go func() { done <- cmd.ExecuteContext(ctx) }()
	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("lost deadline: %v", err)
		}
	case <-time.After(5 * time.Second):
		// Unblock the command even when the regression reappears.
		reader.Close()
		<-done
		t.Fatal("ExecuteContext remained blocked on idle stdin")
	}
	if _, err := reader.Stat(); err == nil {
		t.Fatal("canceled input was not closed")
	}
}

func TestOrdinaryCommandLeavesStdinOpen(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	defer writer.Close()
	if _, err := writer.WriteString("one\n"); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	original := os.Stdin
	os.Stdin = reader
	defer func() { os.Stdin = original }()
	ctx, cancel := context.WithCancel(context.Background())
	cmd := commands.Cmd()
	output := filepath.Join(t.TempDir(), "count.csv")
	cmd.SetArgs([]string{"row-count", "-o", output})
	err = cmd.ExecuteContext(ctx)
	cancel()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reader.Stat(); err != nil {
		t.Fatalf("ordinary completion or late cancellation closed stdin: %v", err)
	}
	data, err := os.ReadFile(output)
	if err != nil || string(data) != "1\n" {
		t.Fatalf("count output %q, error %v", data, err)
	}
}

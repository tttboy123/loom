package tui

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestRunProgramUsesControlledIOAndExitsWithoutStoppingDaemon(t *testing.T) {
	client := &fakeReadClient{}
	var output bytes.Buffer
	err := RunProgram(
		context.Background(),
		client,
		strings.NewReader("q"),
		&output,
		ProgramOptions{DisableRenderer: true},
	)
	if err != nil {
		t.Fatal(err)
	}
	if client.err != nil {
		t.Fatalf("client changed during exit: %v", client.err)
	}
}

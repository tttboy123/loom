package tui

import (
	"context"
	"io"

	tea "github.com/charmbracelet/bubbletea"
)

type ProgramOptions struct {
	DisableRenderer bool
}

func RunProgram(
	ctx context.Context,
	client ReadClient,
	input io.Reader,
	output io.Writer,
	options ProgramOptions,
) error {
	model, err := newModelWithContext(ctx, client)
	if err != nil {
		return err
	}
	programOptions := []tea.ProgramOption{
		tea.WithContext(ctx),
		tea.WithInput(input),
		tea.WithOutput(output),
	}
	if options.DisableRenderer {
		programOptions = append(programOptions, tea.WithoutRenderer())
	}
	_, err = tea.NewProgram(model, programOptions...).Run()
	return err
}

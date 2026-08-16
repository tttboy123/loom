//go:build !unix

package execution

import "context"

func (e *SandboxExecutor) Read(context.Context, ReadRequest) (ReadResult, error) {
	return ReadResult{}, ErrUnsupportedTool
}

func (e *SandboxExecutor) Grep(context.Context, GrepRequest) (GrepResult, error) {
	return GrepResult{}, ErrUnsupportedTool
}

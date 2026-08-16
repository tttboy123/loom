package harnessadapter

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const harnessSystemPromptFilename = "loom-system-prompt.txt"

func harnessSystemPromptPath(tempPath string) string {
	return filepath.Join(tempPath, ".loom-private", harnessSystemPromptFilename)
}

func materializeHarnessSystemPrompt(tempPath, prompt string) (string, func() error, error) {
	if !cleanHarnessAbsolutePath(tempPath) || prompt == "" ||
		len(prompt) > maxHarnessPromptBytes || !utf8.ValidString(prompt) ||
		strings.IndexByte(prompt, 0) >= 0 {
		return "", nil, ErrHarnessProtocol
	}
	info, err := os.Lstat(tempPath)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", nil, ErrHarnessProtocol
	}
	privatePath := filepath.Join(tempPath, ".loom-private")
	if err := os.Mkdir(privatePath, 0o700); err != nil {
		return "", nil, ErrHarnessProtocol
	}
	path := harnessSystemPromptPath(tempPath)
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		_ = os.Remove(privatePath)
		return "", nil, ErrHarnessProtocol
	}
	writeErr := error(nil)
	if _, err := file.WriteString(prompt); err != nil {
		writeErr = err
	}
	if err := file.Sync(); err != nil {
		writeErr = errors.Join(writeErr, err)
	}
	if err := file.Close(); err != nil {
		writeErr = errors.Join(writeErr, err)
	}
	if writeErr != nil {
		_ = os.Remove(path)
		_ = os.Remove(privatePath)
		return "", nil, ErrHarnessProtocol
	}
	cleanup := func() error {
		fileErr := os.Remove(path)
		dirErr := os.Remove(privatePath)
		if fileErr != nil && !errors.Is(fileErr, os.ErrNotExist) ||
			dirErr != nil && !errors.Is(dirErr, os.ErrNotExist) {
			return ErrHarnessProtocol
		}
		return nil
	}
	return path, cleanup, nil
}

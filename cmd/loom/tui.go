package main

import (
	"context"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"time"

	"loom-pi-rebuild/internal/api"
	"loom-pi-rebuild/internal/localipc"
	"loom-pi-rebuild/internal/tui"
)

func runTUIApp(
	ctx context.Context,
	args []string,
	input io.Reader,
	output io.Writer,
) error {
	flags := flag.NewFlagSet("app", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	socket := flags.String("socket", "", "")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		return errors.New("invalid app input")
	}
	socketPath, err := productSocketPath(*socket)
	if err != nil {
		return err
	}
	client, err := localipc.NewClient(localipc.ClientConfig{
		SocketPath: socketPath,
		Timeout:    5 * time.Second,
	})
	if err != nil {
		return err
	}
	readClient, err := tui.NewDaemonReadClient(client)
	if err != nil {
		return err
	}
	return tui.RunProgram(
		ctx,
		readClient,
		input,
		output,
		tui.ProgramOptions{},
	)
}

func readProductSnapshot(
	ctx context.Context,
	socketPath string,
	request api.LocalProductSnapshotRequest,
) (api.LocalProductSnapshot, error) {
	client, err := newProductClient(socketPath)
	if err != nil {
		return api.LocalProductSnapshot{}, err
	}
	var snapshot api.LocalProductSnapshot
	if err := client.Call(ctx, "snapshot", request, &snapshot); err != nil {
		return api.LocalProductSnapshot{}, mapRemoteProductError(err)
	}
	return snapshot, nil
}

func readProductTimeline(
	ctx context.Context,
	socketPath string,
	request api.LocalProductTimelineRequest,
) (api.LocalProductTimelinePage, error) {
	client, err := newProductClient(socketPath)
	if err != nil {
		return api.LocalProductTimelinePage{}, err
	}
	var page api.LocalProductTimelinePage
	if err := client.Call(ctx, "timeline_page", request, &page); err != nil {
		return page, mapRemoteProductError(err)
	}
	if page.Gap != nil {
		return page, &localProductRemoteError{code: "stream_gap"}
	}
	return page, nil
}

func newProductClient(socketPath string) (*localipc.Client, error) {
	return localipc.NewClient(localipc.ClientConfig{
		SocketPath: socketPath,
		Timeout:    5 * time.Second,
	})
}

func mapRemoteProductError(err error) error {
	var remote *localipc.RemoteError
	if errors.As(err, &remote) {
		return &localProductRemoteError{code: remote.Code}
	}
	return err
}

func productSocketPath(explicit string) (string, error) {
	if explicit != "" {
		if !filepath.IsAbs(explicit) ||
			filepath.Clean(explicit) != explicit ||
			filepath.Base(explicit) != "loomd.sock" {
			return "", errors.New("invalid product socket")
		}
		return explicit, nil
	}
	home, err := os.UserHomeDir()
	if err != nil || !filepath.IsAbs(home) || filepath.Clean(home) != home {
		return "", errors.New("invalid user home")
	}
	return filepath.Join(
		home,
		"Library",
		"Application Support",
		"Loom",
		"run",
		"loomd.sock",
	), nil
}
